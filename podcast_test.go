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

func TestRegisteredPodcastToolSchemas(t *testing.T) {
	s := newMCPServer()

	assertProperties := func(toolName string, properties ...string) {
		t.Helper()
		tool := s.GetTool(toolName)
		if tool == nil {
			t.Fatalf("tool %q is not registered", toolName)
		}
		for _, property := range properties {
			if _, ok := tool.Tool.InputSchema.Properties[property]; !ok {
				t.Fatalf("tool %q missing schema property %q", toolName, property)
			}
		}
	}
	assertRequired := func(toolName string, required ...string) {
		t.Helper()
		tool := s.GetTool(toolName)
		if tool == nil {
			t.Fatalf("tool %q is not registered", toolName)
		}
		for _, want := range required {
			found := false
			for _, got := range tool.Tool.InputSchema.Required {
				if got == want {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("tool %q required schema missing %q; required=%v", toolName, want, tool.Tool.InputSchema.Required)
			}
		}
	}

	assertProperties("podcasts", "library_id", "feed", "rss_feed", "opml", "opml_text")
	assertProperties("podcast", "podcast_id", "downloads", "search-episode", "episode_id")
	assertRequired("podcast", "podcast_id")
	assertProperties("check_podcast_episodes", "podcast_id", "limit")
	assertRequired("check_podcast_episodes", "podcast_id")
}

func TestRegisteredPodcastToolsUseSupportedRoutes(t *testing.T) {
	tests := []struct {
		name            string
		toolName        string
		params          map[string]interface{}
		expectedMethod  string
		expectedPath    string
		expectedQuery   string
		expectedPayload map[string]string
	}{
		{
			name:           "list podcast library items",
			toolName:       "podcasts",
			params:         map[string]interface{}{"library_id": "lib-podcasts"},
			expectedMethod: http.MethodGet,
			expectedPath:   "/api/libraries/lib-podcasts/items",
		},
		{
			name:            "fetch feed",
			toolName:        "podcasts",
			params:          map[string]interface{}{"feed": true, "rss_feed": "https://example.com/feed.xml"},
			expectedMethod:  http.MethodPost,
			expectedPath:    "/api/podcasts/feed",
			expectedPayload: map[string]string{"rssFeed": "https://example.com/feed.xml"},
		},
		{
			name:            "parse opml",
			toolName:        "podcasts",
			params:          map[string]interface{}{"opml": true, "opml_text": "<opml></opml>"},
			expectedMethod:  http.MethodPost,
			expectedPath:    "/api/podcasts/opml/parse",
			expectedPayload: map[string]string{"opmlText": "<opml></opml>"},
		},
		{
			name:           "get podcast item",
			toolName:       "podcast",
			params:         map[string]interface{}{"podcast_id": "podcast-item"},
			expectedMethod: http.MethodGet,
			expectedPath:   "/api/items/podcast-item",
		},
		{
			name:           "get downloads",
			toolName:       "podcast",
			params:         map[string]interface{}{"podcast_id": "podcast-item", "downloads": true},
			expectedMethod: http.MethodGet,
			expectedPath:   "/api/podcasts/podcast-item/downloads",
		},
		{
			name:           "search episodes",
			toolName:       "podcast",
			params:         map[string]interface{}{"podcast_id": "podcast-item", "search-episode": true, "title": "Pilot"},
			expectedMethod: http.MethodGet,
			expectedPath:   "/api/podcasts/podcast-item/search-episode",
			expectedQuery:  "title=Pilot",
		},
		{
			name:           "get episode",
			toolName:       "podcast",
			params:         map[string]interface{}{"podcast_id": "podcast-item", "episode_id": "episode-1"},
			expectedMethod: http.MethodGet,
			expectedPath:   "/api/podcasts/podcast-item/episode/episode-1",
		},
		{
			name:           "check new episodes",
			toolName:       "check_podcast_episodes",
			params:         map[string]interface{}{"podcast_id": "podcast-item", "limit": 7},
			expectedMethod: http.MethodGet,
			expectedPath:   "/api/podcasts/podcast-item/checknew",
			expectedQuery:  "limit=7",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newMCPServer()
			recorder := newRecordingServer(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]bool{"ok": true})
			})
			defer recorder.Close()

			params := map[string]interface{}{
				"base_url": recorder.URL,
				"token":    "test-token",
			}
			for key, value := range tt.params {
				params[key] = value
			}

			result := callRegisteredTool(t, s, tt.toolName, params)
			if result.IsError {
				t.Fatalf("expected registered tool success, got error: %#v", result)
			}

			requests := recorder.Requests()
			if len(requests) != 1 {
				t.Fatalf("expected one ABS request, got %d", len(requests))
			}
			request := requests[0]
			if request.Method != tt.expectedMethod {
				t.Fatalf("expected method %s, got %s", tt.expectedMethod, request.Method)
			}
			if request.Path != tt.expectedPath {
				t.Fatalf("expected path %s, got %s", tt.expectedPath, request.Path)
			}
			if request.RawQuery != tt.expectedQuery {
				t.Fatalf("expected query %q, got %q", tt.expectedQuery, request.RawQuery)
			}
			if tt.expectedPayload != nil {
				var payload map[string]string
				if err := json.Unmarshal(request.Body, &payload); err != nil {
					t.Fatalf("decode request body: %v", err)
				}
				for key, expected := range tt.expectedPayload {
					if got := payload[key]; got != expected {
						t.Fatalf("expected payload %s=%q, got %q", key, expected, got)
					}
				}
			} else if len(request.Body) != 0 {
				t.Fatalf("expected empty request body, got %q", string(request.Body))
			}
		})
	}
}

func TestPodcastHandlersReturnErrorsWithoutUnexpectedRequests(t *testing.T) {
	tests := []struct {
		name             string
		handler          func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
		params           map[string]interface{}
		status           int
		expectedRequests int
	}{
		{
			name:             "podcasts missing base URL config",
			handler:          handlePodcasts,
			params:           map[string]interface{}{"base_url": "", "token": "test-token", "library_id": "lib-podcasts"},
			expectedRequests: 0,
		},
		{
			name:             "podcast missing token config",
			handler:          handlePodcast,
			params:           map[string]interface{}{"base_url": "http://127.0.0.1", "token": "", "podcast_id": "podcast-item"},
			expectedRequests: 0,
		},
		{
			name:             "check missing base URL config",
			handler:          handleCheckPodcastEpisodes,
			params:           map[string]interface{}{"base_url": "", "token": "test-token", "podcast_id": "podcast-item"},
			expectedRequests: 0,
		},
		{
			name:             "podcasts missing library id",
			handler:          handlePodcasts,
			params:           map[string]interface{}{},
			expectedRequests: 0,
		},
		{
			name:             "podcasts missing RSS feed",
			handler:          handlePodcasts,
			params:           map[string]interface{}{"feed": true},
			expectedRequests: 0,
		},
		{
			name:             "podcasts missing OPML text",
			handler:          handlePodcasts,
			params:           map[string]interface{}{"opml": true},
			expectedRequests: 0,
		},
		{
			name:             "podcast missing podcast id",
			handler:          handlePodcast,
			params:           map[string]interface{}{},
			expectedRequests: 0,
		},
		{
			name:             "check missing podcast id",
			handler:          handleCheckPodcastEpisodes,
			params:           map[string]interface{}{},
			expectedRequests: 0,
		},
		{
			name:             "podcasts list propagates non-2xx",
			handler:          handlePodcasts,
			params:           map[string]interface{}{"library_id": "lib-podcasts"},
			status:           http.StatusBadGateway,
			expectedRequests: 1,
		},
		{
			name:             "podcasts feed propagates non-2xx",
			handler:          handlePodcasts,
			params:           map[string]interface{}{"feed": true, "rss_feed": "https://example.com/feed.xml"},
			status:           http.StatusBadGateway,
			expectedRequests: 1,
		},
		{
			name:             "podcasts OPML propagates non-2xx",
			handler:          handlePodcasts,
			params:           map[string]interface{}{"opml": true, "opml_text": "<opml></opml>"},
			status:           http.StatusBadGateway,
			expectedRequests: 1,
		},
		{
			name:             "podcast propagates non-2xx",
			handler:          handlePodcast,
			params:           map[string]interface{}{"podcast_id": "podcast-item"},
			status:           http.StatusBadGateway,
			expectedRequests: 1,
		},
		{
			name:             "check propagates non-2xx",
			handler:          handleCheckPodcastEpisodes,
			params:           map[string]interface{}{"podcast_id": "podcast-item"},
			status:           http.StatusBadGateway,
			expectedRequests: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := newRecordingServer(func(w http.ResponseWriter, r *http.Request) {
				status := tt.status
				if status == 0 {
					status = http.StatusOK
				}
				http.Error(w, "ABS failed", status)
			})
			defer recorder.Close()

			params := map[string]interface{}{
				"base_url": recorder.URL,
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
				t.Fatalf("expected tool error, got %#v", result)
			}
			if requests := len(recorder.Requests()); requests != tt.expectedRequests {
				t.Fatalf("expected %d ABS requests, got %d", tt.expectedRequests, requests)
			}
		})
	}
}
