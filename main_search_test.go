package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLibrarySearchHandler(t *testing.T) {
	tests := []struct {
		name          string
		query         interface{}
		limit         interface{}
		expectedQuery string
		expectedLimit string
		expectError   bool
	}{
		{
			name:        "missing query",
			expectError: true,
		},
		{
			name:        "blank query",
			query:       " \t\n ",
			expectError: true,
		},
		{
			name:          "spaces and limit",
			query:         "space search",
			limit:         float64(5),
			expectedQuery: "space search",
			expectedLimit: "5",
		},
		{
			name:          "non-ASCII query",
			query:         "Schöne Grüße",
			expectedQuery: "Schöne Grüße",
		},
		{
			name:          "reserved query characters",
			query:         "a&b=c",
			expectedQuery: "a&b=c",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0
			testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				callCount++
				if r.URL.Path != "/api/libraries/lib123/search" {
					t.Errorf("expected path %q, got %q", "/api/libraries/lib123/search", r.URL.Path)
				}
				if got := r.URL.Query().Get("q"); got != tt.expectedQuery {
					t.Errorf("expected q %q, got %q", tt.expectedQuery, got)
				}
				if got := r.URL.Query().Get("limit"); got != tt.expectedLimit {
					t.Errorf("expected limit %q, got %q", tt.expectedLimit, got)
				}
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{"matches": []interface{}{}})
			}))
			defer testServer.Close()

			params := map[string]interface{}{
				"base_url":   testServer.URL,
				"token":      "test-token",
				"library_id": "lib123",
				"search":     true,
			}
			if tt.query != nil {
				params["query"] = tt.query
			}
			if tt.limit != nil {
				params["limit"] = tt.limit
			}

			result, err := createLibraryHandler()(context.Background(), makeRequest(params))
			if err != nil {
				t.Fatalf("unexpected handler error: %v", err)
			}

			if tt.expectError {
				if result == nil || !result.IsError {
					t.Fatalf("expected MCP tool error, got %#v", result)
				}
				if callCount != 0 {
					t.Fatalf("expected no HTTP calls, got %d", callCount)
				}
				return
			}

			if result == nil {
				t.Fatal("expected result, got nil")
			}
			if result.IsError {
				t.Fatalf("result returned error: %#v", result)
			}
			if callCount != 1 {
				t.Fatalf("expected one HTTP call, got %d", callCount)
			}
		})
	}
}

func TestPodcastSearchEpisodeHandler(t *testing.T) {
	tests := []struct {
		name          string
		title         interface{}
		expectedTitle string
		expectError   bool
	}{
		{
			name:        "missing title",
			expectError: true,
		},
		{
			name:        "blank title",
			title:       " \t\n ",
			expectError: true,
		},
		{
			name:          "spaces",
			title:         "episode title",
			expectedTitle: "episode title",
		},
		{
			name:          "non-ASCII title",
			title:         "Schöne Grüße",
			expectedTitle: "Schöne Grüße",
		},
		{
			name:          "reserved title characters",
			title:         "a&b=c",
			expectedTitle: "a&b=c",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0
			testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				callCount++
				if r.URL.Path != "/api/podcasts/podcast123/search-episode" {
					t.Errorf("expected path %q, got %q", "/api/podcasts/podcast123/search-episode", r.URL.Path)
				}
				if got := r.URL.Query().Get("title"); got != tt.expectedTitle {
					t.Errorf("expected title %q, got %q", tt.expectedTitle, got)
				}
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{"episodes": []interface{}{}})
			}))
			defer testServer.Close()

			params := map[string]interface{}{
				"base_url":       testServer.URL,
				"token":          "test-token",
				"podcast_id":     "podcast123",
				"search-episode": true,
			}
			if tt.title != nil {
				params["title"] = tt.title
			}

			result, err := createPodcastHandler()(context.Background(), makeRequest(params))
			if err != nil {
				t.Fatalf("unexpected handler error: %v", err)
			}

			if tt.expectError {
				if result == nil || !result.IsError {
					t.Fatalf("expected MCP tool error, got %#v", result)
				}
				if callCount != 0 {
					t.Fatalf("expected no HTTP calls, got %d", callCount)
				}
				return
			}

			if result == nil {
				t.Fatal("expected result, got nil")
			}
			if result.IsError {
				t.Fatalf("result returned error: %#v", result)
			}
			if callCount != 1 {
				t.Fatalf("expected one HTTP call, got %d", callCount)
			}
		})
	}
}
