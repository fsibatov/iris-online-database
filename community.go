package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	vkCommunityPageURL         = "https://vk.ru/wall-59626511"
	vkNewsJSONURL              = "https://raw.githubusercontent.com/fsibatov/iris-online-database/main/data/latest-vk.json"
	vkNewsGitHubAPIURL         = "https://api.github.com/repos/fsibatov/iris-online-database/contents/data/latest-vk.json?ref=main"
	maxCommunityNewsBytes      = 256 << 10
	maxCommunityPostTextLength = 4000
	communityCacheTTL          = 2 * time.Minute
	communityFailureRetryTTL   = 15 * time.Second
)

type communityStatusResult struct {
	Available       bool   `json:"available"`
	Stale           bool   `json:"stale,omitempty"`
	CommunityURL    string `json:"communityUrl"`
	LatestPostID    int64  `json:"latestPostId,omitempty"`
	LatestPostURL   string `json:"latestPostUrl,omitempty"`
	LatestPostText  string `json:"latestPostText,omitempty"`
	PublishedAt     string `json:"publishedAt,omitempty"`
	SourceUpdatedAt string `json:"sourceUpdatedAt,omitempty"`
}

type communityNewsFile struct {
	Schema        int    `json:"schema"`
	CommunityURL  string `json:"community_url"`
	PostID        int64  `json:"post_id"`
	PostURL       string `json:"post_url"`
	Text          string `json:"text"`
	PublishedAt   string `json:"published_at"`
	SourceUpdated string `json:"source_updated_at"`
}

type communityChecker struct {
	mu          sync.Mutex
	attempted   bool
	checkedAt   time.Time
	result      communityStatusResult
	lastGood    communityStatusResult
	client      *http.Client
	newsURL     string
	fallbackURL string
}

func newCommunityChecker() *communityChecker {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 2
	transport.MaxIdleConnsPerHost = 2
	transport.IdleConnTimeout = 15 * time.Second
	transport.ResponseHeaderTimeout = 5 * time.Second
	transport.TLSHandshakeTimeout = 5 * time.Second
	seed := embeddedCommunityNews()
	return &communityChecker{
		client: &http.Client{
			Transport: transport,
			Timeout:   7 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 3 || req.URL.Scheme != "https" || req.URL.User != nil || (req.URL.Port() != "" && req.URL.Port() != "443") {
					return http.ErrUseLastResponse
				}
				host := strings.ToLower(req.URL.Hostname())
				if host != "raw.githubusercontent.com" && host != "api.github.com" && !strings.HasSuffix(host, ".githubusercontent.com") {
					return http.ErrUseLastResponse
				}
				req.Header.Del("Referer")
				req.Header.Del("Cookie")
				req.Header.Del("Authorization")
				return nil
			},
		},
		newsURL:     vkNewsJSONURL,
		fallbackURL: vkNewsGitHubAPIURL,
		attempted:   seed.Available,
		checkedAt:   time.Now(),
		result:      seed,
		lastGood:    seed,
	}
}

func embeddedCommunityNews() communityStatusResult {
	body, err := embedded.ReadFile("data/latest-vk.json")
	if err != nil {
		return communityStatusResult{CommunityURL: vkCommunityPageURL}
	}
	result := decodeCommunityNews(body)
	result.Stale = result.Available
	return result
}

func (c *communityChecker) Check(ctx context.Context, force bool) communityStatusResult {
	if c == nil {
		return communityStatusResult{CommunityURL: vkCommunityPageURL}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	cacheTTL := communityCacheTTL
	if !c.result.Available {
		cacheTTL = communityFailureRetryTTL
	}
	if c.attempted && !force && time.Since(c.checkedAt) < cacheTTL {
		return c.result
	}
	fresh := checkCommunityNewsJSON(ctx, c.client, c.newsURL, force)
	if !fresh.Available && strings.TrimSpace(c.fallbackURL) != "" {
		fresh = checkCommunityNewsGitHubAPI(ctx, c.client, c.fallbackURL, force)
	}
	if fresh.Available {
		c.lastGood = fresh
		c.result = fresh
	} else if c.lastGood.Available {
		fallback := c.lastGood
		fallback.Stale = true
		c.result = fallback
	} else if c.result.Available {
		c.lastGood = c.result
	}
	c.attempted = true
	c.checkedAt = time.Now()
	return c.result
}

func communityNewsBody(ctx context.Context, client *http.Client, target string, force bool, githubAPI bool) []byte {
	if client == nil || strings.TrimSpace(target) == "" {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	requestURL := target
	if !githubAPI {
		if parsed, err := url.Parse(target); err == nil {
			query := parsed.Query()
			refresh := time.Now().UTC().Truncate(time.Minute).Unix()
			if force {
				refresh = time.Now().UnixNano()
			}
			query.Set("refresh", strconv.FormatInt(refresh, 10))
			parsed.RawQuery = query.Encode()
			requestURL = parsed.String()
		}
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil
	}
	if githubAPI {
		request.Header.Set("Accept", "application/vnd.github+json")
		request.Header.Set("X-GitHub-Api-Version", "2026-03-10")
	} else {
		request.Header.Set("Accept", "application/json")
	}
	request.Header.Set("User-Agent", "IrisOnlineDatabase/"+appVersion)
	request.Header.Set("Cache-Control", "no-cache")
	request.Header.Set("Pragma", "no-cache")

	response, err := client.Do(request)
	if err != nil {
		return nil
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxCommunityNewsBytes+1))
	if err != nil || len(body) > maxCommunityNewsBytes {
		return nil
	}
	return body
}

func checkCommunityNewsGitHubAPI(ctx context.Context, client *http.Client, target string, force bool) communityStatusResult {
	result := communityStatusResult{CommunityURL: vkCommunityPageURL}
	body := communityNewsBody(ctx, client, target, force, true)
	if len(body) == 0 {
		return result
	}
	var payload struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || !strings.EqualFold(strings.TrimSpace(payload.Encoding), "base64") {
		return result
	}
	encoded := strings.Map(func(r rune) rune {
		if r == '\r' || r == '\n' || r == ' ' || r == '\t' {
			return -1
		}
		return r
	}, payload.Content)
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(decoded) > maxCommunityNewsBytes {
		return result
	}
	return decodeCommunityNews(decoded)
}

func checkCommunityNewsJSON(ctx context.Context, client *http.Client, target string, force bool) communityStatusResult {
	result := communityStatusResult{CommunityURL: vkCommunityPageURL}
	body := communityNewsBody(ctx, client, target, force, false)
	if len(body) == 0 {
		return result
	}
	return decodeCommunityNews(body)
}

func decodeCommunityNews(body []byte) communityStatusResult {
	result := communityStatusResult{CommunityURL: vkCommunityPageURL}
	var payload communityNewsFile
	if err := json.Unmarshal(body, &payload); err != nil {
		return result
	}
	text := cleanCommunityPostText(payload.Text)
	if payload.Schema != 1 || payload.PostID <= 0 || text == "" || !validCommunityPostURL(payload.PostURL, payload.PostID) {
		return result
	}

	result.Available = true
	result.LatestPostID = payload.PostID
	result.LatestPostURL = strings.TrimSpace(payload.PostURL)
	result.LatestPostText = text
	result.PublishedAt = cleanRFC3339(payload.PublishedAt)
	result.SourceUpdatedAt = cleanRFC3339(payload.SourceUpdated)
	if strings.TrimSpace(payload.CommunityURL) == vkCommunityPageURL {
		result.CommunityURL = vkCommunityPageURL
	}
	return result
}

func validCommunityPostURL(value string, postID int64) bool {
	value = strings.TrimSpace(value)
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.User != nil || parsed.Opaque != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	if port := parsed.Port(); port != "" && port != "443" {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "vk.ru" && host != "www.vk.ru" && host != "vk.com" && host != "www.vk.com" {
		return false
	}
	expected := "wall-59626511_" + strconv.FormatInt(postID, 10)
	return strings.Trim(parsed.Path, "/") == expected
}

func cleanRFC3339(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return ""
	}
	return parsed.Format(time.RFC3339)
}

func cleanCommunityPostText(value string) string {
	value = strings.ToValidUTF8(value, "�")
	value = strings.ReplaceAll(value, "\\n", "\n")
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	lines := strings.Split(value, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.Join(strings.Fields(strings.TrimSpace(line)), " ")
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}
	value = strings.TrimSpace(strings.Join(cleaned, "\n"))
	if len([]rune(value)) > maxCommunityPostTextLength {
		runes := []rune(value)
		value = strings.TrimSpace(string(runes[:maxCommunityPostTextLength])) + "…"
	}
	return value
}
