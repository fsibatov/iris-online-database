package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCheckCommunityNewsJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("refresh") == "" {
			t.Fatal("refresh cache-buster is missing")
		}
		if r.Header.Get("Cache-Control") != "no-cache" || r.Header.Get("Pragma") != "no-cache" {
			t.Fatalf("cache bypass headers are missing: Cache-Control=%q Pragma=%q", r.Header.Get("Cache-Control"), r.Header.Get("Pragma"))
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"schema":1,"community_url":"https://vk.ru/wall-59626511","post_id":62336,"post_url":"https://vk.ru/wall-59626511_62336","text":"Первая строка\\nВторая строка","published_at":"2026-08-11T18:30:00Z","source_updated_at":"2026-08-11T19:00:00Z"}`)
	}))
	defer server.Close()

	result := checkCommunityNewsJSON(context.Background(), server.Client(), server.URL, false)
	if !result.Available || result.LatestPostID != 62336 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.LatestPostText != "Первая строка\nВторая строка" {
		t.Fatalf("unexpected text: %q", result.LatestPostText)
	}
	if result.PublishedAt != "2026-08-11T18:30:00Z" {
		t.Fatalf("unexpected published date: %q", result.PublishedAt)
	}
}

func TestCheckCommunityNewsJSONRejectsWrongPostURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"schema":1,"post_id":62336,"post_url":"https://example.com/wall-59626511_62336"}`)
	}))
	defer server.Close()
	if result := checkCommunityNewsJSON(context.Background(), server.Client(), server.URL, false); result.Available {
		t.Fatalf("invalid external URL accepted: %+v", result)
	}
}

func TestCommunityCheckerForceBypassesCachedResult(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		id := 62336 + calls - 1
		fmt.Fprintf(w, `{"schema":1,"post_id":%d,"post_url":"https://vk.ru/wall-59626511_%d"}`, id, id)
	}))
	defer server.Close()
	checker := &communityChecker{client: server.Client(), newsURL: server.URL, result: communityStatusResult{CommunityURL: vkCommunityPageURL}}
	first := checker.Check(context.Background(), false)
	cached := checker.Check(context.Background(), false)
	forced := checker.Check(context.Background(), true)
	if first.LatestPostID != 62336 || cached.LatestPostID != 62336 || forced.LatestPostID != 62337 || calls != 2 {
		t.Fatalf("unexpected cache behavior: first=%+v cached=%+v forced=%+v calls=%d", first, cached, forced, calls)
	}
}

func TestCommunityCheckerRefreshesExpiredCachedResult(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		fmt.Fprint(w, `{"schema":1,"post_id":62337,"post_url":"https://vk.ru/wall-59626511_62337"}`)
	}))
	defer server.Close()

	checker := &communityChecker{
		client:    server.Client(),
		newsURL:   server.URL,
		attempted: true,
		checkedAt: time.Now().Add(-communityCacheTTL - time.Second),
		result:    communityStatusResult{Available: true, CommunityURL: vkCommunityPageURL, LatestPostID: 62336},
	}
	result := checker.Check(context.Background(), false)
	if result.LatestPostID != 62337 || calls != 1 {
		t.Fatalf("expired cache was not refreshed: result=%+v calls=%d", result, calls)
	}
}

func TestCleanCommunityPostText(t *testing.T) {
	got := cleanCommunityPostText("  строка  один \\n\\n  строка   два  ")
	if got != "строка один\nстрока два" {
		t.Fatalf("unexpected cleaned text: %q", got)
	}
	long := strings.Repeat("я", maxCommunityPostTextLength+20)
	if got := cleanCommunityPostText(long); len([]rune(got)) != maxCommunityPostTextLength+1 || !strings.HasSuffix(got, "…") {
		t.Fatalf("long text was not truncated correctly: %d", len([]rune(got)))
	}
}
