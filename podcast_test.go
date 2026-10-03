package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestPodcastHandlersUseSupportedRoutes(t *testing.T) {
	tests := []struct {
		name            string
		handler         func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
		params          map[string]interface{}
		expectedMethod  string
		expectedPath    string
		expectedQuery   map[string]string
		expectedPayload map[string]string
	}{
		{
			name:           "list podcast library items",
			handler:        handlePodcasts,
			params:         map[string]interface{}{"library_id": "lib-podcasts"},
			expectedMethod: http.MethodGet,
			expectedPath:   "/api/libraries/lib-podcasts/items",
		},
		{
			name:           "fetch podcast feed metadata",
			handler:        handlePodcasts,
			params:         map[string]interface{}{"feed": true, "rss_feed": "https://example.com/feed.xml"},
			expectedMethod: http.MethodPost,
			expectedPath:   "/api/podcasts/feed",
			expectedPayload: map[string]string{
				"rssFeed": "https://example.com/feed.xml",
			},
		},
		{
			name:           "parse opml text",
			handler:        handlePodcasts,
			params:         map[string]interface{}{"opml": true, "opml_text": "<opml></opml>"},
			expectedMethod: http.MethodPost,
			expectedPath:   "/api/podcasts/opml/parse",
			expectedPayload: map[string]string{
				"opmlText": "<opml></opml>",
			},
		},
		{
			name:           "get podcast library item details",
			handler:        handlePodcast,
			params:         map[string]interface{}{"podcast_id": "podcast-item"},
			expectedMethod: http.MethodGet,
			expectedPath:   "/api/items/podcast-item",
		},
		{
			name:           "get podcast downloads",
			handler:        handlePodcast,
			params:         map[string]interface{}{"podcast_id": "podcast-item", "downloads": true},
			expectedMethod: http.MethodGet,
			expectedPath:   "/api/podcasts/podcast-item/downloads",
		},
		{
			name:           "search podcast episodes keeps endpoint",
			handler:        handlePodcast,
			params:         map[string]interface{}{"podcast_id": "podcast-item", "search-episode": true, "title": "Pilot"},
			expectedMethod: http.MethodGet,
			expectedPath:   "/api/podcasts/podcast-item/search-episode",
		},
		{
			name:           "get podcast episode",
			handler:        handlePodcast,
			params:         map[string]interface{}{"podcast_id": "podcast-item", "episode_id": "episode-1"},
			expectedMethod: http.MethodGet,
			expectedPath:   "/api/podcasts/podcast-item/episode/episode-1",
		},
		{
			name:           "check podcast episodes",
			handler:        handleCheckPodcastEpisodes,
			params:         map[string]interface{}{"podcast_id": "podcast-item", "limit": 7},
			expectedMethod: http.MethodGet,
			expectedPath:   "/api/podcasts/podcast-item/checknew",
			expectedQuery: map[string]string{
				"limit": "7",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true

				if r.Method != tt.expectedMethod {
					t.Errorf("expected method %s, got %s", tt.expectedMethod, r.Method)
				}
				if r.URL.Path != tt.expectedPath {
					t.Errorf("expected path %s, got %s", tt.expectedPath, r.URL.Path)
				}
				for key, expected := range tt.expectedQuery {
					if got := r.URL.Query().Get(key); got != expected {
						t.Errorf("expected query %s=%s, got %s", key, expected, got)
					}
				}

				if tt.expectedPayload != nil {
					var payload map[string]string
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
						t.Errorf("decode payload: %v", err)
					}
					for key, expected := range tt.expectedPayload {
						if got := payload[key]; got != expected {
							t.Errorf("expected payload %s=%q, got %q", key, expected, got)
						}
					}
				}

				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]bool{"ok": true})
			}))
			defer testServer.Close()

			params := map[string]interface{}{
				"base_url": testServer.URL,
				"token":    "test-token",
			}
			for key, value := range tt.params {
				params[key] = value
			}

			result, err := tt.handler(context.Background(), makeRequest(params))
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
		})
	}
}

func TestPodcastHandlersRejectBlankRequiredParameters(t *testing.T) {
	tests := []struct {
		name    string
		handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
		params  map[string]interface{}
	}{
		{
			name:    "blank library id",
			handler: handlePodcasts,
			params:  map[string]interface{}{"library_id": "   "},
		},
		{
			name:    "blank rss feed",
			handler: handlePodcasts,
			params:  map[string]interface{}{"feed": true, "rss_feed": ""},
		},
		{
			name:    "blank opml text",
			handler: handlePodcasts,
			params:  map[string]interface{}{"opml": true, "opml_text": "\t"},
		},
		{
			name:    "blank podcast id",
			handler: handlePodcast,
			params:  map[string]interface{}{"podcast_id": ""},
		},
		{
			name:    "blank check podcast id",
			handler: handleCheckPodcastEpisodes,
			params:  map[string]interface{}{"podcast_id": "   "},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				t.Fatal("validation failure should not call ABS")
			}))
			defer testServer.Close()

			params := map[string]interface{}{
				"base_url": testServer.URL,
				"token":    "test-token",
			}
			for key, value := range tt.params {
				params[key] = value
			}

			result, err := tt.handler(context.Background(), makeRequest(params))
			if err != nil {
				t.Fatalf("unexpected protocol error: %v", err)
			}
			if result == nil || !result.IsError {
				t.Fatalf("expected tool error for blank input, got %#v", result)
			}
			if called {
				t.Fatal("expected no ABS request for blank input")
			}
		})
	}
}
