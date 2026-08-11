package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	githubLatestReleaseAPI = "https://api.github.com/repos/fsibatov/iris-online-database/releases/latest"
	githubLatestReleaseURL = "https://github.com/fsibatov/iris-online-database/releases/latest"
	maxUpdateResponseBytes = 32 << 10
)

var versionPattern = regexp.MustCompile(`^[vV]?(\d+)\.(\d+)\.(\d+)$`)

type updateCheckResult struct {
	CurrentVersion  string `json:"currentVersion"`
	LatestVersion   string `json:"latestVersion,omitempty"`
	UpdateAvailable bool   `json:"updateAvailable"`
	ReleaseURL      string `json:"releaseUrl,omitempty"`
	Checked         bool   `json:"checked"`
}

type updateChecker struct {
	once   sync.Once
	result updateCheckResult
	client *http.Client
	apiURL string
}

func newUpdateChecker() *updateChecker {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 1
	transport.MaxIdleConnsPerHost = 1
	transport.IdleConnTimeout = 5 * time.Second
	return &updateChecker{
		client: &http.Client{
			Transport: transport,
			Timeout:   5 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		apiURL: githubLatestReleaseAPI,
	}
}

func (c *updateChecker) Check(ctx context.Context) updateCheckResult {
	if c == nil {
		return updateCheckResult{CurrentVersion: appVersion}
	}
	c.once.Do(func() {
		c.result = checkLatestRelease(ctx, c.client, c.apiURL, appVersion)
	})
	return c.result
}

func checkLatestRelease(ctx context.Context, client *http.Client, apiURL, currentVersion string) updateCheckResult {
	result := updateCheckResult{CurrentVersion: currentVersion}
	if client == nil || apiURL == "" {
		return result
	}
	defer client.CloseIdleConnections()

	if ctx == nil {
		ctx = context.Background()
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return result
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "IrisOnlineDatabase")
	request.Header.Set("X-GitHub-Api-Version", "2026-03-10")

	response, err := client.Do(request)
	if err != nil {
		return result
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return result
	}

	limited := io.LimitReader(response.Body, maxUpdateResponseBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil || len(body) > maxUpdateResponseBytes {
		return result
	}
	var payload struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return result
	}
	latest, err := normalizeVersion(payload.TagName)
	if err != nil {
		return result
	}
	current, err := normalizeVersion(currentVersion)
	if err != nil {
		return result
	}

	result.Checked = true
	result.LatestVersion = latest
	result.UpdateAvailable = compareVersions(latest, current) > 0
	if result.UpdateAvailable {
		result.ReleaseURL = githubLatestReleaseURL
	}
	return result
}

func normalizeVersion(value string) (string, error) {
	match := versionPattern.FindStringSubmatch(strings.TrimSpace(value))
	if match == nil {
		return "", errors.New("unsupported version format")
	}
	parts := make([]int, 3)
	for i := range parts {
		number, err := strconv.Atoi(match[i+1])
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
	parse := func(value string) [3]int {
		var out [3]int
		chunks := strings.Split(value, ".")
		for i := 0; i < len(out); i++ {
			out[i], _ = strconv.Atoi(chunks[i])
		}
		return out
	}
	left, right := parse(av), parse(bv)
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
