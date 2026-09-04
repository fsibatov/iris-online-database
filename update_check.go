package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	githubLatestReleaseAPI = "https://api.github.com/repos/fsibatov/iris-online-database/releases/latest"
	githubLatestReleaseURL = "https://github.com/fsibatov/iris-online-database/releases/latest"
	githubReleaseTagPrefix = "https://github.com/fsibatov/iris-online-database/releases/tag/v"
	maxUpdateResponseBytes = 32 << 10
	updateSuccessCacheTTL  = 30 * time.Minute
	updateFailureCacheTTL  = 30 * time.Second
	updateRequestTimeout   = 5 * time.Second
)

const (
	updateFailureCanceled        = "canceled"
	updateFailureConfiguration   = "configuration"
	updateFailureInvalidResponse = "invalid_response"
	updateFailureNetwork         = "network"
	updateFailureRateLimited     = "rate_limited"
	updateFailureService         = "service_unavailable"
	updateFailureTimeout         = "timeout"
)

var versionPattern = regexp.MustCompile(`(?i)^v?\s*(\d+)\.(\d+)(?:\.(\d+))?\.?$`)

type updateCheckResult struct {
	CurrentVersion    string `json:"currentVersion"`
	LatestVersion     string `json:"latestVersion,omitempty"`
	UpdateAvailable   bool   `json:"updateAvailable"`
	ReleaseURL        string `json:"releaseUrl,omitempty"`
	Checked           bool   `json:"checked"`
	Stale             bool   `json:"stale,omitempty"`
	Failure           string `json:"failure,omitempty"`
	RetryAfterSeconds int64  `json:"retryAfterSeconds,omitempty"`
	diagnostic        string
}

type updateChecker struct {
	mu        sync.Mutex
	attempted bool
	checkedAt time.Time
	result    updateCheckResult
	lastGood  updateCheckResult
	client    *http.Client
	apiURL    string
	webURL    string
}

func newUpdateChecker() *updateChecker {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 2
	transport.MaxIdleConnsPerHost = 2
	transport.IdleConnTimeout = 15 * time.Second
	transport.ResponseHeaderTimeout = updateRequestTimeout
	transport.TLSHandshakeTimeout = updateRequestTimeout
	return &updateChecker{
		client: &http.Client{
			Transport: transport,
			Timeout:   updateRequestTimeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		apiURL: githubLatestReleaseAPI,
		webURL: githubLatestReleaseURL,
	}
}

func (c *updateChecker) Check(ctx context.Context, force bool) updateCheckResult {
	if c == nil {
		return updateCheckResult{CurrentVersion: appVersion, Failure: updateFailureConfiguration}
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.attempted {
		age := time.Since(c.checkedAt)
		if age >= 0 {
			if cooldown := updateRetryCooldown(c.result); cooldown > 0 && age < cooldown {
				return c.result
			}
			if !force {
				ttl := updateFailureCacheTTL
				if c.result.Checked && !c.result.Stale && c.result.Failure == "" {
					ttl = updateSuccessCacheTTL
				}
				if age < ttl {
					return c.result
				}
			}
		}
	}

	fresh := checkLatestReleaseSources(ctx, c.client, c.webURL, c.apiURL, appVersion)
	if fresh.Checked && fresh.Failure == "" {
		c.lastGood = fresh
		c.result = fresh
	} else if c.lastGood.Checked {
		fallback := c.lastGood
		fallback.Stale = true
		fallback.Failure = fresh.Failure
		fallback.RetryAfterSeconds = fresh.RetryAfterSeconds
		fallback.diagnostic = fresh.diagnostic
		c.result = fallback
	} else {
		c.result = fresh
	}
	c.attempted = true
	c.checkedAt = time.Now()
	return c.result
}

func checkLatestRelease(ctx context.Context, client *http.Client, apiURL, currentVersion string) updateCheckResult {
	return checkLatestReleaseSources(ctx, client, "", apiURL, currentVersion)
}

func checkLatestReleaseSources(ctx context.Context, client *http.Client, webURL, apiURL, currentVersion string) updateCheckResult {
	result := updateCheckResult{CurrentVersion: currentVersion}
	current, err := normalizeVersion(currentVersion)
	if err != nil {
		result.Failure = updateFailureConfiguration
		result.diagnostic = "invalid current version: " + err.Error()
		return result
	}
	if client == nil || (strings.TrimSpace(webURL) == "" && strings.TrimSpace(apiURL) == "") {
		result.Failure = updateFailureConfiguration
		result.diagnostic = "update source or HTTP client is not configured"
		return result
	}
	if ctx == nil {
		ctx = context.Background()
	}
	defer client.CloseIdleConnections()

	failures := make([]updateCheckResult, 0, 2)
	if strings.TrimSpace(webURL) != "" {
		latest, failure := latestReleaseFromWeb(ctx, client, webURL)
		if latest != "" {
			return buildSuccessfulUpdateResult(currentVersion, current, latest)
		}
		failures = append(failures, failure)
		if failure.Failure == updateFailureCanceled {
			return failureWithCurrentVersion(failure, currentVersion)
		}
	}
	if strings.TrimSpace(apiURL) != "" {
		latest, failure := latestReleaseFromAPI(ctx, client, apiURL)
		if latest != "" {
			return buildSuccessfulUpdateResult(currentVersion, current, latest)
		}
		failures = append(failures, failure)
	}

	failure := bestUpdateFailure(failures)
	return failureWithCurrentVersion(failure, currentVersion)
}

func latestReleaseFromAPI(ctx context.Context, client *http.Client, target string) (string, updateCheckResult) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return "", updateFailure(updateFailureConfiguration, 0, "create GitHub API request: "+err.Error())
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "IrisOnlineDatabase/"+appVersion)
	request.Header.Set("X-GitHub-Api-Version", "2026-03-10")

	response, err := client.Do(request)
	if err != nil {
		return "", classifyUpdateTransportError(ctx, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", classifyUpdateHTTPFailure(response)
	}

	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || (mediaType != "application/json" && mediaType != "application/vnd.github+json") {
		return "", updateFailure(updateFailureInvalidResponse, 0, "unexpected GitHub API content type: "+response.Header.Get("Content-Type"))
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxUpdateResponseBytes+1))
	if err != nil {
		return "", updateFailure(updateFailureService, 0, "read GitHub API response: "+err.Error())
	}
	if len(body) > maxUpdateResponseBytes {
		return "", updateFailure(updateFailureInvalidResponse, 0, "GitHub API response exceeds size limit")
	}
	var payload struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", updateFailure(updateFailureInvalidResponse, 0, "decode GitHub API response: "+err.Error())
	}
	latest, err := normalizeVersion(payload.TagName)
	if err != nil {
		return "", updateFailure(updateFailureInvalidResponse, 0, "invalid GitHub release tag: "+err.Error())
	}
	return latest, updateCheckResult{}
}

func latestReleaseFromWeb(ctx context.Context, client *http.Client, target string) (string, updateCheckResult) {
	request, err := http.NewRequestWithContext(ctx, http.MethodHead, target, nil)
	if err != nil {
		return "", updateFailure(updateFailureConfiguration, 0, "create GitHub release request: "+err.Error())
	}
	request.Header.Set("Accept", "text/html,application/xhtml+xml")
	request.Header.Set("User-Agent", "IrisOnlineDatabase/"+appVersion)

	response, err := client.Do(request)
	if err != nil {
		return "", classifyUpdateTransportError(ctx, err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusMovedPermanently &&
		response.StatusCode != http.StatusFound &&
		response.StatusCode != http.StatusSeeOther &&
		response.StatusCode != http.StatusTemporaryRedirect &&
		response.StatusCode != http.StatusPermanentRedirect {
		return "", classifyUpdateHTTPFailure(response)
	}
	location := strings.TrimSpace(response.Header.Get("Location"))
	if location == "" {
		return "", updateFailure(updateFailureInvalidResponse, 0, "GitHub release redirect is missing Location")
	}
	base, err := url.Parse(target)
	if err != nil {
		return "", updateFailure(updateFailureConfiguration, 0, "invalid configured GitHub release URL: "+err.Error())
	}
	redirect, err := base.Parse(location)
	if err != nil || redirect.Scheme != "https" || !strings.EqualFold(redirect.Hostname(), "github.com") || redirect.User != nil || (redirect.Port() != "" && redirect.Port() != "443") {
		return "", updateFailure(updateFailureInvalidResponse, 0, "GitHub release redirect is not trusted")
	}
	const expectedPathPrefix = "/fsibatov/iris-online-database/releases/tag/"
	if !strings.HasPrefix(redirect.EscapedPath(), expectedPathPrefix) {
		return "", updateFailure(updateFailureInvalidResponse, 0, "GitHub release redirect has unexpected path")
	}
	tagEscaped := strings.TrimPrefix(redirect.EscapedPath(), expectedPathPrefix)
	if tagEscaped == "" || strings.Contains(tagEscaped, "/") {
		return "", updateFailure(updateFailureInvalidResponse, 0, "GitHub release redirect has invalid tag")
	}
	tag, err := url.PathUnescape(tagEscaped)
	if err != nil {
		return "", updateFailure(updateFailureInvalidResponse, 0, "GitHub release redirect tag cannot be decoded")
	}
	latest, err := normalizeVersion(tag)
	if err != nil {
		return "", updateFailure(updateFailureInvalidResponse, 0, "invalid GitHub release tag: "+err.Error())
	}
	return latest, updateCheckResult{}
}

func buildSuccessfulUpdateResult(currentVersion, current, latest string) updateCheckResult {
	result := updateCheckResult{
		CurrentVersion: currentVersion,
		LatestVersion:  latest,
		Checked:        true,
	}
	result.UpdateAvailable = compareNormalizedVersions(latest, current) > 0
	if result.UpdateAvailable {
		result.ReleaseURL = githubReleaseTagPrefix + latest
	}
	return result
}

func updateFailure(code string, retryAfter int64, diagnostic string) updateCheckResult {
	return updateCheckResult{Failure: code, RetryAfterSeconds: retryAfter, diagnostic: diagnostic}
}

func failureWithCurrentVersion(result updateCheckResult, currentVersion string) updateCheckResult {
	result.CurrentVersion = currentVersion
	result.Checked = false
	result.UpdateAvailable = false
	result.LatestVersion = ""
	result.ReleaseURL = ""
	return result
}

func classifyUpdateTransportError(ctx context.Context, err error) updateCheckResult {
	if ctx != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return updateFailure(updateFailureCanceled, 0, "update request canceled")
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return updateFailure(updateFailureTimeout, 0, "update request deadline exceeded")
		}
	}
	if errors.Is(err, context.Canceled) {
		return updateFailure(updateFailureCanceled, 0, "update request canceled")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return updateFailure(updateFailureTimeout, 0, "update request deadline exceeded")
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return updateFailure(updateFailureTimeout, 0, "update request timed out: "+err.Error())
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return updateFailure(updateFailureNetwork, 0, "DNS lookup failed: "+dnsErr.Error())
	}
	return updateFailure(updateFailureNetwork, 0, "update network request failed: "+err.Error())
}

func classifyUpdateHTTPFailure(response *http.Response) updateCheckResult {
	if response == nil {
		return updateFailure(updateFailureService, 0, "empty HTTP response")
	}
	retryAfter := updateRetryAfterSeconds(response.Header)
	remaining := strings.TrimSpace(response.Header.Get("X-RateLimit-Remaining"))
	if response.StatusCode == http.StatusTooManyRequests ||
		(response.StatusCode == http.StatusForbidden && (remaining == "0" || retryAfter > 0)) {
		if retryAfter == 0 {
			retryAfter = 60
		}
		return updateFailure(updateFailureRateLimited, retryAfter, fmt.Sprintf("GitHub rate limited request with HTTP %d", response.StatusCode))
	}
	if response.StatusCode >= 500 && response.StatusCode <= 599 {
		return updateFailure(updateFailureService, retryAfter, fmt.Sprintf("GitHub service returned HTTP %d", response.StatusCode))
	}
	return updateFailure(updateFailureService, retryAfter, fmt.Sprintf("GitHub returned HTTP %d", response.StatusCode))
}

func updateRetryAfterSeconds(header http.Header) int64 {
	if header == nil {
		return 0
	}
	if raw := strings.TrimSpace(header.Get("Retry-After")); raw != "" {
		if seconds, err := strconv.ParseInt(raw, 10, 64); err == nil && seconds > 0 {
			return capUpdateRetryAfter(seconds)
		}
		if when, err := http.ParseTime(raw); err == nil {
			seconds := int64(time.Until(when).Seconds())
			if seconds > 0 {
				return capUpdateRetryAfter(seconds)
			}
		}
	}
	if raw := strings.TrimSpace(header.Get("X-RateLimit-Reset")); raw != "" {
		if reset, err := strconv.ParseInt(raw, 10, 64); err == nil {
			seconds := reset - time.Now().Unix()
			if seconds > 0 {
				return capUpdateRetryAfter(seconds)
			}
		}
	}
	return 0
}

func capUpdateRetryAfter(seconds int64) int64 {
	const maximum = int64(24 * time.Hour / time.Second)
	if seconds > maximum {
		return maximum
	}
	return seconds
}

func updateRetryCooldown(result updateCheckResult) time.Duration {
	if result.Failure == "" || result.RetryAfterSeconds <= 0 {
		return 0
	}
	return time.Duration(capUpdateRetryAfter(result.RetryAfterSeconds)) * time.Second
}

func bestUpdateFailure(values []updateCheckResult) updateCheckResult {
	if len(values) == 0 {
		return updateFailure(updateFailureService, 0, "no update source returned a result")
	}
	priority := map[string]int{
		updateFailureCanceled:        70,
		updateFailureConfiguration:   60,
		updateFailureRateLimited:     50,
		updateFailureTimeout:         40,
		updateFailureNetwork:         30,
		updateFailureService:         20,
		updateFailureInvalidResponse: 10,
	}
	best := values[0]
	for _, value := range values[1:] {
		if priority[value.Failure] > priority[best.Failure] {
			best = value
			continue
		}
		if value.Failure == best.Failure && value.RetryAfterSeconds > best.RetryAfterSeconds {
			best.RetryAfterSeconds = value.RetryAfterSeconds
		}
	}
	return best
}

func normalizeVersion(value string) (string, error) {
	match := versionPattern.FindStringSubmatch(strings.TrimSpace(value))
	if match == nil {
		return "", errors.New("unsupported version format")
	}
	parts := make([]int, 3)
	for i := range parts {
		raw := match[i+1]
		if raw == "" {
			parts[i] = 0
			continue
		}
		number, err := strconv.Atoi(raw)
		if err != nil || number < 0 {
			return "", errors.New("invalid version")
		}
		parts[i] = number
	}
	return fmt.Sprintf("%d.%d.%d", parts[0], parts[1], parts[2]), nil
}

func compareVersions(a, b string) int {
	av, aerr := normalizeVersion(a)
	bv, berr := normalizeVersion(b)
	if aerr != nil || berr != nil {
		return 0
	}
	return compareNormalizedVersions(av, bv)
}

func compareNormalizedVersions(a, b string) int {
	parse := func(value string) [3]int {
		var out [3]int
		chunks := strings.Split(value, ".")
		for i := 0; i < len(out); i++ {
			out[i], _ = strconv.Atoi(chunks[i])
		}
		return out
	}
	left, right := parse(a), parse(b)
	for i := range left {
		if left[i] < right[i] {
			return -1
		}
		if left[i] > right[i] {
			return 1
		}
	}
	return 0
}
