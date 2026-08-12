package main

import (
	"context"
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
	maxCommunityNewsBytes      = 256 << 10
	maxCommunityPostTextLength = 4000
	communityCacheTTL          = 2 * time.Minute
)

type communityStatusResult struct {
	Available       bool   `json:"available"`
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
	mu        sync.Mutex
	attempted bool
	checkedAt time.Time
	result    communityStatusResult
	client    *http.Client
	newsURL   string
}

func newCommunityChecker() *communityChecker {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 2
	transport.MaxIdleConnsPerHost = 2
	transport.IdleConnTimeout = 5 * time.Second
	return &communityChecker{
		client: &http.Client{
			Transport: transport,
			Timeout:   7 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 3 {
					return http.ErrUseLastResponse
				}
				host := strings.ToLower(req.URL.Hostname())
				if host != "raw.githubusercontent.com" && !strings.HasSuffix(host, ".githubusercontent.com") {
					return http.ErrUseLastResponse
				}
				req.Header.Del("Referer")
				req.Header.Del("Cookie")
				req.Header.Del("Authorization")
				return nil
			},
		},
		newsURL: vkNewsJSONURL,
		result:  communityStatusResult{CommunityURL: vkCommunityPageURL},
	}
}

func (c *communityChecker) Check(ctx context.Context, force bool) communityStatusResult {
	if c == nil {
		return communityStatusResult{CommunityURL: vkCommunityPageURL}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.attempted && c.result.Available && !force && time.Since(c.checkedAt) < communityCacheTTL {
		return c.result
	}
	c.result = checkCommunityNewsJSON(ctx, c.client, c.newsURL, force)
	c.attempted = true
	c.checkedAt = time.Now()
	return c.result
}

func checkCommunityNewsJSON(ctx context.Context, client *http.Client, target string, force bool) communityStatusResult {
	result := communityStatusResult{CommunityURL: vkCommunityPageURL}
	if client == nil || strings.TrimSpace(target) == "" {
		return result
	}
	if ctx == nil {
		ctx = context.Background()
	}

	requestURL := target
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

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return result
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "IrisOnlineDatabase/1.1.0")
	request.Header.Set("Cache-Control", "no-cache")
	request.Header.Set("Pragma", "no-cache")

	response, err := client.Do(request)
	if err != nil {
		return result
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return result
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxCommunityNewsBytes+1))
	if err != nil || len(body) > maxCommunityNewsBytes {
		return result
	}

	var payload communityNewsFile
	if err := json.Unmarshal(body, &payload); err != nil {
		return result
	}
	if payload.Schema != 1 || payload.PostID <= 0 || !validCommunityPostURL(payload.PostURL, payload.PostID) {
		return result
	}

	result.Available = true
	result.LatestPostID = payload.PostID
	result.LatestPostURL = strings.TrimSpace(payload.PostURL)
	result.LatestPostText = cleanCommunityPostText(payload.Text)
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
	if err != nil || parsed.Scheme != "https" {
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
