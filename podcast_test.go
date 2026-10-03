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
