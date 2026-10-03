package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestCheckPodcastEpisodesUsesSupportedRoute(t *testing.T) {
	called := false
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != http.MethodGet {
			t.Errorf("expected method %s, got %s", http.MethodGet, r.Method)
		}
		if r.URL.Path != "/api/podcasts/podcast-item/checknew" {
			t.Errorf("expected path /api/podcasts/podcast-item/checknew, got %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("limit"); got != "7" {
			t.Errorf("expected query limit=7, got %s", got)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	}))
	defer testServer.Close()

	result, err := handleCheckPodcastEpisodes(context.Background(), makeRequest(map[string]interface{}{
		"base_url":   testServer.URL,
		"token":      "test-token",
		"podcast_id": "podcast-item",
		"limit":      7,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.IsError {
		t.Fatalf("result returned error: %v", result)
	}
	if !called {
		t.Fatal("expected test server to be called")
	}
}

var _ func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) = handleCheckPodcastEpisodes

func TestPodcastsFeedUsesSupportedRoute(t *testing.T) {
	called := false
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != http.MethodPost {
			t.Errorf("expected method %s, got %s", http.MethodPost, r.Method)
		}
		if r.URL.Path != "/api/podcasts/feed" {
			t.Errorf("expected path /api/podcasts/feed, got %s", r.URL.Path)
		}
		var payload map[string]string
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode payload: %v", err)
		}
		if got := payload["rssFeed"]; got != "https://example.com/feed.xml" {
			t.Errorf("expected rssFeed payload, got %q", got)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	}))
	defer testServer.Close()

	result, err := handlePodcasts(context.Background(), makeRequest(map[string]interface{}{
		"base_url": testServer.URL,
		"token":    "test-token",
		"feed":     true,
		"rss_feed": "https://example.com/feed.xml",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.IsError {
		t.Fatalf("result returned error: %v", result)
	}
	if !called {
		t.Fatal("expected test server to be called")
	}
}
