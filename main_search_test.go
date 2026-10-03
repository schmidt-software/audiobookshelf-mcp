package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func assertToolHasProperties(t *testing.T, toolName string, required []string, propertyTypes map[string]string) {
	t.Helper()
	tool := newMCPServer().GetTool(toolName)
	if tool == nil {
		t.Fatalf("expected tool %q to be registered", toolName)
	}

	for _, name := range required {
		found := false
		for _, requiredName := range tool.Tool.InputSchema.Required {
			if requiredName == name {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected %q to be required for tool %q; required=%v", name, toolName, tool.Tool.InputSchema.Required)
		}
	}

	for name, expectedType := range propertyTypes {
		property, ok := tool.Tool.InputSchema.Properties[name]
		if !ok {
			t.Fatalf("expected schema property %q for tool %q", name, toolName)
		}
		propertyMap, ok := property.(map[string]any)
		if !ok {
			t.Fatalf("expected schema property %q to be map[string]any, got %T", name, property)
		}
		if propertyMap["type"] != expectedType {
			t.Fatalf("expected schema property %q type %q, got %#v", name, expectedType, propertyMap["type"])
		}
	}
}

func assertNoRequests(t *testing.T, recorder *recordingServer) {
	t.Helper()
	if requests := recorder.Requests(); len(requests) != 0 {
		t.Fatalf("expected no outbound request, got %d", len(requests))
	}
}

func TestLibraryToolSearchSchema(t *testing.T) {
	assertToolHasProperties(t, "library", []string{"library_id"}, map[string]string{
		"library_id": "string",
		"search":     "boolean",
		"query":      "string",
		"limit":      "number",
	})
}

func TestPodcastToolSearchEpisodeSchema(t *testing.T) {
	assertToolHasProperties(t, "podcast", []string{"podcast_id"}, map[string]string{
		"podcast_id":     "string",
		"search-episode": "boolean",
		"title":          "string",
	})
}

func TestRegisteredLibrarySearchTool(t *testing.T) {
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
			s := newMCPServer()
			recorder := newRecordingServer(func(w http.ResponseWriter, r *http.Request) {
				if !requireMethod(w, r, http.MethodGet) {
					return
				}
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
			})
			defer recorder.Close()

			params := map[string]interface{}{
				"base_url":   recorder.URL,
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

			result := callRegisteredTool(t, s, "library", params)
			if tt.expectError {
				if !result.IsError {
					t.Fatalf("expected MCP tool error, got %#v", result)
				}
				assertNoRequests(t, recorder)
				return
			}

			if result.IsError {
				t.Fatalf("result returned error: %#v", result)
			}
			recorded, ok := recorder.LastRequest()
			if !ok {
				t.Fatal("expected recorded request")
			}
			if recorded.Method != http.MethodGet {
				t.Fatalf("expected GET request, got %s", recorded.Method)
			}
			if recorded.Path != "/api/libraries/lib123/search" {
				t.Fatalf("expected search path, got %q", recorded.Path)
			}
			values, err := url.ParseQuery(recorded.RawQuery)
			if err != nil {
				t.Fatalf("parse raw query %q: %v", recorded.RawQuery, err)
			}
			if values.Get("q") != tt.expectedQuery {
				t.Fatalf("expected recorded q %q, got %q", tt.expectedQuery, values.Get("q"))
			}
			if values.Get("limit") != tt.expectedLimit {
				t.Fatalf("expected recorded limit %q, got %q", tt.expectedLimit, values.Get("limit"))
			}
		})
	}
}

func TestRegisteredLibrarySubResourcePaths(t *testing.T) {
	tests := []struct {
		name         string
		params       map[string]interface{}
		expectedPath string
	}{
		{name: "base library", params: map[string]interface{}{}, expectedPath: "/api/libraries/lib123"},
		{name: "items", params: map[string]interface{}{"items": true}, expectedPath: "/api/libraries/lib123/items"},
		{name: "authors", params: map[string]interface{}{"authors": true}, expectedPath: "/api/libraries/lib123/authors"},
		{name: "series", params: map[string]interface{}{"series": true}, expectedPath: "/api/libraries/lib123/series"},
		{name: "collections", params: map[string]interface{}{"collections": true}, expectedPath: "/api/libraries/lib123/collections"},
		{name: "playlists", params: map[string]interface{}{"playlists": true}, expectedPath: "/api/libraries/lib123/playlists"},
		{name: "personalized", params: map[string]interface{}{"personalized": true}, expectedPath: "/api/libraries/lib123/personalized"},
		{name: "filterdata", params: map[string]interface{}{"filterdata": true}, expectedPath: "/api/libraries/lib123/filterdata"},
		{name: "stats", params: map[string]interface{}{"stats": true}, expectedPath: "/api/libraries/lib123/stats"},
		{name: "episode-downloads", params: map[string]interface{}{"episode-downloads": true}, expectedPath: "/api/libraries/lib123/episode-downloads"},
		{name: "recent-episodes", params: map[string]interface{}{"recent-episodes": true}, expectedPath: "/api/libraries/lib123/recent-episodes"},
		{name: "first sub-resource wins", params: map[string]interface{}{"items": true, "authors": true, "search": true, "query": "ignored"}, expectedPath: "/api/libraries/lib123/items"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newMCPServer()
			recorder := newRecordingServer(func(w http.ResponseWriter, r *http.Request) {
				if !requireMethod(w, r, http.MethodGet) {
					return
				}
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{"path": r.URL.Path})
			})
			defer recorder.Close()

			params := map[string]interface{}{
				"base_url":   recorder.URL,
				"token":      "test-token",
				"library_id": "lib123",
			}
			for key, value := range tt.params {
				params[key] = value
			}

			result := callRegisteredTool(t, s, "library", params)
			if result.IsError {
				t.Fatalf("result returned error: %#v", result)
			}
			recorded, ok := recorder.LastRequest()
			if !ok {
				t.Fatal("expected recorded request")
			}
			if recorded.Method != http.MethodGet {
				t.Fatalf("expected GET request, got %s", recorded.Method)
			}
			if recorded.Path != tt.expectedPath {
				t.Fatalf("expected path %q, got %q", tt.expectedPath, recorded.Path)
			}
			if recorded.RawQuery != "" {
				t.Fatalf("expected empty query, got %q", recorded.RawQuery)
			}
		})
	}
}

func TestRegisteredLibraryErrorPaths(t *testing.T) {
	t.Run("missing base_url", func(t *testing.T) {
		result := callRegisteredTool(t, newMCPServer(), "library", map[string]interface{}{
			"token":      "test-token",
			"library_id": "lib123",
		})
		if !result.IsError {
			t.Fatal("expected missing base_url to return a tool error")
		}
	})

	t.Run("missing token", func(t *testing.T) {
		recorder := newRecordingServer(func(w http.ResponseWriter, r *http.Request) {
			t.Fatalf("server should not be called when token is missing")
		})
		defer recorder.Close()
		result := callRegisteredTool(t, newMCPServer(), "library", map[string]interface{}{
			"base_url":   recorder.URL,
			"library_id": "lib123",
		})
		if !result.IsError {
			t.Fatal("expected missing token to return a tool error")
		}
		assertNoRequests(t, recorder)
	})

	t.Run("missing library_id", func(t *testing.T) {
		recorder := newRecordingServer(func(w http.ResponseWriter, r *http.Request) {
			t.Fatalf("server should not be called when library_id is missing")
		})
		defer recorder.Close()
		result := callRegisteredTool(t, newMCPServer(), "library", map[string]interface{}{
			"base_url": recorder.URL,
			"token":    "test-token",
		})
		if !result.IsError {
			t.Fatal("expected missing library_id to return a tool error")
		}
		assertNoRequests(t, recorder)
	})

	t.Run("ABS error response", func(t *testing.T) {
		recorder := newRecordingServer(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "library failed", http.StatusBadGateway)
		})
		defer recorder.Close()
		result := callRegisteredTool(t, newMCPServer(), "library", map[string]interface{}{
			"base_url":   recorder.URL,
			"token":      "test-token",
			"library_id": "lib123",
		})
		if !result.IsError {
			t.Fatal("expected ABS failure to return a tool error")
		}
		if requests := recorder.Requests(); len(requests) != 1 {
			t.Fatalf("expected one outbound request, got %d", len(requests))
		}
	})
}

func TestRegisteredPodcastSearchEpisodeTool(t *testing.T) {
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
			s := newMCPServer()
			recorder := newRecordingServer(func(w http.ResponseWriter, r *http.Request) {
				if !requireMethod(w, r, http.MethodGet) {
					return
				}
				if r.URL.Path != "/api/podcasts/podcast123/search-episode" {
					t.Errorf("expected path %q, got %q", "/api/podcasts/podcast123/search-episode", r.URL.Path)
				}
				if got := r.URL.Query().Get("title"); got != tt.expectedTitle {
					t.Errorf("expected title %q, got %q", tt.expectedTitle, got)
				}
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{"episodes": []interface{}{}})
			})
			defer recorder.Close()

			params := map[string]interface{}{
				"base_url":       recorder.URL,
				"token":          "test-token",
				"podcast_id":     "podcast123",
				"search-episode": true,
			}
			if tt.title != nil {
				params["title"] = tt.title
			}

			result := callRegisteredTool(t, s, "podcast", params)
			if tt.expectError {
				if !result.IsError {
					t.Fatalf("expected MCP tool error, got %#v", result)
				}
				assertNoRequests(t, recorder)
				return
			}

			if result.IsError {
				t.Fatalf("result returned error: %#v", result)
			}
			recorded, ok := recorder.LastRequest()
			if !ok {
				t.Fatal("expected recorded request")
			}
			if recorded.Method != http.MethodGet {
				t.Fatalf("expected GET request, got %s", recorded.Method)
			}
			if recorded.Path != "/api/podcasts/podcast123/search-episode" {
				t.Fatalf("expected search-episode path, got %q", recorded.Path)
			}
			values, err := url.ParseQuery(recorded.RawQuery)
			if err != nil {
				t.Fatalf("parse raw query %q: %v", recorded.RawQuery, err)
			}
			if values.Get("title") != tt.expectedTitle {
				t.Fatalf("expected recorded title %q, got %q", tt.expectedTitle, values.Get("title"))
			}
		})
	}
}

func TestRegisteredPodcastSubResourcePaths(t *testing.T) {
	tests := []struct {
		name         string
		params       map[string]interface{}
		expectedPath string
	}{
		{name: "base podcast", params: map[string]interface{}{}, expectedPath: "/api/podcasts/podcast123"},
		{name: "downloads", params: map[string]interface{}{"downloads": true}, expectedPath: "/api/podcasts/podcast123/downloads"},
		{name: "episode_id", params: map[string]interface{}{"episode_id": "episode456"}, expectedPath: "/api/podcasts/podcast123/episode/episode456"},
		{name: "first sub-resource wins", params: map[string]interface{}{"downloads": true, "search-episode": true, "title": "ignored", "episode_id": "episode456"}, expectedPath: "/api/podcasts/podcast123/downloads"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := newRecordingServer(func(w http.ResponseWriter, r *http.Request) {
				if !requireMethod(w, r, http.MethodGet) {
					return
				}
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{"path": r.URL.Path})
			})
			defer recorder.Close()

			params := map[string]interface{}{
				"base_url":   recorder.URL,
				"token":      "test-token",
				"podcast_id": "podcast123",
			}
			for key, value := range tt.params {
				params[key] = value
			}

			result := callRegisteredTool(t, newMCPServer(), "podcast", params)
			if result.IsError {
				t.Fatalf("result returned error: %#v", result)
			}
			recorded, ok := recorder.LastRequest()
			if !ok {
				t.Fatal("expected recorded request")
			}
			if recorded.Method != http.MethodGet {
				t.Fatalf("expected GET request, got %s", recorded.Method)
			}
			if recorded.Path != tt.expectedPath {
				t.Fatalf("expected path %q, got %q", tt.expectedPath, recorded.Path)
			}
			if recorded.RawQuery != "" {
				t.Fatalf("expected empty query, got %q", recorded.RawQuery)
			}
		})
	}
}

func TestRegisteredPodcastErrorPaths(t *testing.T) {
	t.Run("missing base_url", func(t *testing.T) {
		result := callRegisteredTool(t, newMCPServer(), "podcast", map[string]interface{}{
			"token":      "test-token",
			"podcast_id": "podcast123",
		})
		if !result.IsError {
			t.Fatal("expected missing base_url to return a tool error")
		}
	})

	t.Run("missing token", func(t *testing.T) {
		recorder := newRecordingServer(func(w http.ResponseWriter, r *http.Request) {
			t.Fatalf("server should not be called when token is missing")
		})
		defer recorder.Close()
		result := callRegisteredTool(t, newMCPServer(), "podcast", map[string]interface{}{
			"base_url":   recorder.URL,
			"podcast_id": "podcast123",
		})
		if !result.IsError {
			t.Fatal("expected missing token to return a tool error")
		}
		assertNoRequests(t, recorder)
	})

	t.Run("missing podcast_id", func(t *testing.T) {
		recorder := newRecordingServer(func(w http.ResponseWriter, r *http.Request) {
			t.Fatalf("server should not be called when podcast_id is missing")
		})
		defer recorder.Close()
		result := callRegisteredTool(t, newMCPServer(), "podcast", map[string]interface{}{
			"base_url": recorder.URL,
			"token":    "test-token",
		})
		if !result.IsError {
			t.Fatal("expected missing podcast_id to return a tool error")
		}
		assertNoRequests(t, recorder)
	})

	t.Run("ABS error response", func(t *testing.T) {
		recorder := newRecordingServer(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "podcast failed", http.StatusBadGateway)
		})
		defer recorder.Close()
		result := callRegisteredTool(t, newMCPServer(), "podcast", map[string]interface{}{
			"base_url":   recorder.URL,
			"token":      "test-token",
			"podcast_id": "podcast123",
		})
		if !result.IsError {
			t.Fatal("expected ABS failure to return a tool error")
		}
		if requests := recorder.Requests(); len(requests) != 1 {
			t.Fatalf("expected one outbound request, got %d", len(requests))
		}
	})
}

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
