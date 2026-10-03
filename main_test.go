package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type recordedRequest struct {
	Method   string
	Path     string
	RawQuery string
	Body     []byte
}

type recordingServer struct {
	*httptest.Server
	mu       sync.Mutex
	requests []recordedRequest
}

func newRecordingServer(handler http.HandlerFunc) *recordingServer {
	rs := &recordingServer{}
	rs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		r.Body.Close()
		r.Body = io.NopCloser(bytes.NewReader(body))

		rs.mu.Lock()
		rs.requests = append(rs.requests, recordedRequest{
			Method:   r.Method,
			Path:     r.URL.Path,
			RawQuery: r.URL.RawQuery,
			Body:     append([]byte(nil), body...),
		})
		rs.mu.Unlock()

		handler(w, r)
	}))
	return rs
}

func (rs *recordingServer) Requests() []recordedRequest {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	requests := make([]recordedRequest, len(rs.requests))
	copy(requests, rs.requests)
	return requests
}

func (rs *recordingServer) LastRequest() (recordedRequest, bool) {
	requests := rs.Requests()
	if len(requests) == 0 {
		return recordedRequest{}, false
	}
	return requests[len(requests)-1], true
}

func requireMethod(w http.ResponseWriter, r *http.Request, methods ...string) bool {
	for _, method := range methods {
		if r.Method == method {
			return true
		}
	}
	w.Header().Set("Allow", strings.Join(methods, ", "))
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	return false
}

func callRegisteredTool(t *testing.T, s *server.MCPServer, name string, params map[string]interface{}) *mcp.CallToolResult {
	t.Helper()
	tool := s.GetTool(name)
	if tool == nil {
		t.Fatalf("tool %q is not registered", name)
	}
	request := makeRequest(params)
	request.Params.Name = name
	result, err := tool.Handler(context.Background(), request)
	if err != nil {
		t.Fatalf("tool %q returned protocol error: %v", name, err)
	}
	if result == nil {
		t.Fatalf("tool %q returned nil result", name)
	}
	return result
}

// Mock server that simulates Audiobookshelf API
func setupMockABSServer() *httptest.Server {
	mux := http.NewServeMux()

	// Server endpoints
	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"success": "true"})
	})

	mux.HandleFunc("/healthcheck", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]bool{"healthy": true})
	})

	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"isInit":   true,
			"language": "en-us",
		})
	})

	// Libraries endpoints
	mux.HandleFunc("/api/libraries", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(w, r, http.MethodGet, http.MethodPost) {
			return
		}
		if r.Method == http.MethodPost {
			// Handle POST - create library
			var payload map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, "Invalid JSON", http.StatusBadRequest)
				return
			}

			// Validate required fields
			if _, ok := payload["name"]; !ok {
				http.Error(w, "name is required", http.StatusBadRequest)
				return
			}
			if _, ok := payload["folders"]; !ok {
				http.Error(w, "folders is required", http.StatusBadRequest)
				return
			}

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":      "new-lib-123",
				"name":    payload["name"],
				"folders": payload["folders"],
			})
			return
		}

		// Handle GET - list libraries
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"libraries": []map[string]string{
				{"id": "lib1", "name": "Audiobooks"},
				{"id": "lib2", "name": "Podcasts"},
			},
		})
	})

	mux.HandleFunc("/api/libraries/", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/libraries/"), "/")
		if len(parts) == 0 || parts[0] == "" {
			http.Error(w, "Library ID required", http.StatusBadRequest)
			return
		}

		libraryID := parts[0]

		if len(parts) == 1 {
			// Base library info
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":   libraryID,
				"name": "Test Library",
			})
			return
		}

		// Sub-resources
		subResource := parts[1]
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"libraryId": libraryID,
			"resource":  subResource,
			"data":      []interface{}{},
		})
	})

	// Items endpoints
	mux.HandleFunc("/api/items/", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		itemID := strings.TrimPrefix(r.URL.Path, "/api/items/")
		parts := strings.Split(itemID, "/")

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":   parts[0],
			"type": "book",
		})
	})

	// Authors endpoints
	mux.HandleFunc("/api/authors/", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/api/authors/")
		parts := strings.Split(path, "/")

		if len(parts) > 1 && parts[1] == "image" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("fake-image-data"))
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":   parts[0],
			"name": "Test Author",
		})
	})

	// Series endpoints
	mux.HandleFunc("/api/series/", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		seriesID := strings.TrimPrefix(r.URL.Path, "/api/series/")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":   seriesID,
			"name": "Test Series",
		})
	})

	// Users endpoints
	mux.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"users": []map[string]string{
				{"id": "user1", "username": "admin"},
			},
		})
	})

	mux.HandleFunc("/api/users/online", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"users": []map[string]string{},
		})
	})

	mux.HandleFunc("/api/users/", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/api/users/")
		parts := strings.Split(path, "/")

		if len(parts) == 1 {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":       parts[0],
				"username": "testuser",
			})
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"userId":   parts[0],
			"resource": parts[1],
		})
	})

	// Me endpoints
	mux.HandleFunc("/api/me", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":       "current-user",
			"username": "me",
		})
	})

	mux.HandleFunc("/api/me/listening-sessions", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"sessions": []interface{}{},
		})
	})

	mux.HandleFunc("/api/me/listening-stats", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"totalTime": 0,
		})
	})

	mux.HandleFunc("/api/me/items-in-progress", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"items": []interface{}{},
		})
	})

	mux.HandleFunc("/api/me/progress/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"progress": 0.5,
		})
	})

	// Sessions endpoints
	mux.HandleFunc("/api/sessions", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"sessions": []interface{}{},
		})
	})

	mux.HandleFunc("/api/sessions/", func(w http.ResponseWriter, r *http.Request) {
		sessionID := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": sessionID,
		})
	})

	// Podcasts endpoints
	mux.HandleFunc("/api/podcasts", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"podcasts": []interface{}{},
		})
	})

	mux.HandleFunc("/api/podcasts/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "podcast1",
		})
	})

	// Collections endpoints
	mux.HandleFunc("/api/collections", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"collections": []interface{}{},
		})
	})

	mux.HandleFunc("/api/collections/", func(w http.ResponseWriter, r *http.Request) {
		collectionID := strings.TrimPrefix(r.URL.Path, "/api/collections/")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": collectionID,
		})
	})

	// Playlists endpoints
	mux.HandleFunc("/api/playlists", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"playlists": []interface{}{},
		})
	})

	mux.HandleFunc("/api/playlists/", func(w http.ResponseWriter, r *http.Request) {
		playlistID := strings.TrimPrefix(r.URL.Path, "/api/playlists/")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": playlistID,
		})
	})

	// Backups endpoint
	mux.HandleFunc("/api/backups", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"backups": []interface{}{},
		})
	})

	// Filesystem endpoint
	mux.HandleFunc("/api/filesystem", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"directories": []string{"/audiobooks", "/podcasts"},
		})
	})

	// Authorize endpoint
	mux.HandleFunc("/api/authorize", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"user":   map[string]string{"id": "user1"},
			"server": map[string]string{"version": "2.0.0"},
		})
	})

	// Tags endpoint
	mux.HandleFunc("/api/tags", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"tags": []string{"fiction", "non-fiction"},
		})
	})

	// Genres endpoint
	mux.HandleFunc("/api/genres", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"genres": []string{"Fantasy", "Science Fiction", "Mystery"},
		})
	})

	return httptest.NewServer(mux)
}

// Helper to create a basic request with auth
func makeRequest(params map[string]interface{}) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "",
			Arguments: params,
		},
	}
}

func TestGetEnvOrParam(t *testing.T) {
	tests := []struct {
		name       string
		paramValue string
		envKey     string
		envValue   string
		expected   string
	}{
		{
			name:       "prefer param over env",
			paramValue: "param_value",
			envKey:     "TEST_KEY",
			envValue:   "env_value",
			expected:   "param_value",
		},
		{
			name:       "use env when param empty",
			paramValue: "",
			envKey:     "TEST_KEY",
			envValue:   "env_value",
			expected:   "env_value",
		},
		{
			name:       "return empty when both empty",
			paramValue: "",
			envKey:     "TEST_KEY",
			envValue:   "",
			expected:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(tt.envKey, tt.envValue)
				defer os.Unsetenv(tt.envKey)
			}

			result := getEnvOrParam(tt.paramValue, tt.envKey)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestGetABSConfig(t *testing.T) {
	tests := []struct {
		name        string
		params      map[string]interface{}
		envBaseURL  string
		envToken    string
		expectError bool
		expectedURL string
	}{
		{
			name: "valid params",
			params: map[string]interface{}{
				"base_url": "https://abs.example.com",
				"token":    "test-token",
			},
			expectError: false,
			expectedURL: "https://abs.example.com/api",
		},
		{
			name:        "use env vars",
			params:      map[string]interface{}{},
			envBaseURL:  "https://env.example.com",
			envToken:    "env-token",
			expectError: false,
			expectedURL: "https://env.example.com/api",
		},
		{
			name: "missing base_url",
			params: map[string]interface{}{
				"token": "test-token",
			},
			expectError: true,
		},
		{
			name: "missing token",
			params: map[string]interface{}{
				"base_url": "https://abs.example.com",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envBaseURL != "" {
				os.Setenv("ABS_BASE_URL", tt.envBaseURL)
				defer os.Unsetenv("ABS_BASE_URL")
			}
			if tt.envToken != "" {
				os.Setenv("ABS_API_KEY", tt.envToken)
				defer os.Unsetenv("ABS_API_KEY")
			}

			request := makeRequest(tt.params)
			baseURL, token, err := getABSConfig(request)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if baseURL != tt.expectedURL {
					t.Errorf("expected URL %q, got %q", tt.expectedURL, baseURL)
				}
				if token == "" {
					t.Error("expected non-empty token")
				}
			}
		})
	}
}

func TestABSGET(t *testing.T) {
	mockServer := setupMockABSServer()
	defer mockServer.Close()

	tests := []struct {
		name        string
		path        string
		expectError bool
		checkBody   func([]byte) error
	}{
		{
			name:        "successful GET request",
			path:        "/ping",
			expectError: false,
			checkBody: func(body []byte) error {
				if !strings.Contains(string(body), "success") {
					return fmt.Errorf("expected 'success' in body, got: %s", string(body))
				}
				return nil
			},
		},
		{
			name:        "libraries endpoint",
			path:        "/api/libraries",
			expectError: false,
			checkBody: func(body []byte) error {
				if !strings.Contains(string(body), "libraries") {
					return fmt.Errorf("expected 'libraries' in body, got: %s", string(body))
				}
				return nil
			},
		},
		{
			name:        "404 endpoint",
			path:        "/api/nonexistent",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := absGET(context.Background(), mockServer.URL, "test-token", tt.path)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if tt.checkBody != nil {
					if err := tt.checkBody(body); err != nil {
						t.Error(err)
					}
				}
			}
		})
	}
}

func TestEndpointHandlers(t *testing.T) {
	mockServer := setupMockABSServer()
	defer mockServer.Close()

	// Remove /api from mock server URL since getABSConfig adds it
	baseURL := strings.TrimSuffix(mockServer.URL, "/api")

	tests := []struct {
		name        string
		handler     func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
		params      map[string]interface{}
		expectError bool
		checkResult func(*mcp.CallToolResult) error
	}{
		{
			name:    "libraries handler",
			handler: createSimpleGETHandler("/libraries"),
			params: map[string]interface{}{
				"base_url": baseURL,
				"token":    "test-token",
			},
			expectError: false,
			checkResult: func(result *mcp.CallToolResult) error {
				if len(result.Content) == 0 {
					return fmt.Errorf("expected content, got empty")
				}
				return nil
			},
		},
		{
			name:    "library by ID handler",
			handler: createGETByIDHandler("/libraries/%s", "library_id"),
			params: map[string]interface{}{
				"base_url":   baseURL,
				"token":      "test-token",
				"library_id": "lib123",
			},
			expectError: false,
			checkResult: func(result *mcp.CallToolResult) error {
				if len(result.Content) == 0 {
					return fmt.Errorf("expected content, got empty")
				}
				return nil
			},
		},
		{
			name:    "author handler",
			handler: createGETByIDHandler("/authors/%s", "author_id"),
			params: map[string]interface{}{
				"base_url":  baseURL,
				"token":     "test-token",
				"author_id": "author123",
			},
			expectError: false,
		},
		{
			name:    "series handler",
			handler: createGETByIDHandler("/series/%s", "series_id"),
			params: map[string]interface{}{
				"base_url":  baseURL,
				"token":     "test-token",
				"series_id": "series123",
			},
			expectError: false,
		},
		{
			name:    "users handler",
			handler: createSimpleGETHandler("/users"),
			params: map[string]interface{}{
				"base_url": baseURL,
				"token":    "test-token",
			},
			expectError: false,
		},
		{
			name:    "tags handler",
			handler: createSimpleGETHandler("/tags"),
			params: map[string]interface{}{
				"base_url": baseURL,
				"token":    "test-token",
			},
			expectError: false,
		},
		{
			name:    "genres handler",
			handler: createSimpleGETHandler("/genres"),
			params: map[string]interface{}{
				"base_url": baseURL,
				"token":    "test-token",
			},
			expectError: false,
		},
		{
			name:    "missing required ID parameter",
			handler: createGETByIDHandler("/libraries/%s", "library_id"),
			params: map[string]interface{}{
				"base_url": baseURL,
				"token":    "test-token",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := makeRequest(tt.params)
			result, err := tt.handler(context.Background(), request)

			if tt.expectError {
				if err != nil || (result != nil && result.IsError) {
					// Expected error
					return
				}
				t.Error("expected error, got success")
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result == nil {
					t.Error("expected result, got nil")
				}
				if result.IsError {
					t.Errorf("result returned error: %v", result)
				}
				if tt.checkResult != nil {
					if err := tt.checkResult(result); err != nil {
						t.Error(err)
					}
				}
			}
		})
	}
}

func TestSubResourceHandlers(t *testing.T) {
	tests := []struct {
		name           string
		basePath       string
		idParamName    string
		subResources   []string
		params         map[string]interface{}
		expectedPath   string
		expectedInPath string
	}{
		{
			name:         "library with items sub-resource",
			basePath:     "/libraries/%s",
			idParamName:  "library_id",
			subResources: []string{"items", "authors", "series"},
			params: map[string]interface{}{
				"token":      "test-token",
				"library_id": "lib123",
				"items":      true,
			},
			expectedPath:   "/api/libraries/lib123/items",
			expectedInPath: "items",
		},
		{
			name:         "library without sub-resource",
			basePath:     "/libraries/%s",
			idParamName:  "library_id",
			subResources: []string{"items", "authors"},
			params: map[string]interface{}{
				"token":      "test-token",
				"library_id": "lib123",
			},
			expectedPath:   "/api/libraries/lib123",
			expectedInPath: "lib123",
		},
		{
			name:         "user with listening-sessions",
			basePath:     "/users/%s",
			idParamName:  "user_id",
			subResources: []string{"listening-sessions", "listening-stats"},
			params: map[string]interface{}{
				"token":              "test-token",
				"user_id":            "user123",
				"listening-sessions": true,
			},
			expectedPath:   "/api/users/user123/listening-sessions",
			expectedInPath: "listening-sessions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := newRecordingServer(func(w http.ResponseWriter, r *http.Request) {
				if !requireMethod(w, r, http.MethodGet) {
					return
				}
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"path": r.URL.Path,
				})
			})
			defer recorder.Close()

			params := make(map[string]interface{}, len(tt.params)+1)
			for key, value := range tt.params {
				params[key] = value
			}
			params["base_url"] = recorder.URL

			handler := createGETByIDWithSubResourceHandler(tt.basePath, tt.idParamName, tt.subResources)
			request := makeRequest(params)
			result, err := handler(context.Background(), request)

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if result == nil {
				t.Error("expected result, got nil")
			}
			if result.IsError {
				t.Errorf("result returned error: %v", result)
			}
			if len(result.Content) == 0 {
				t.Error("expected content, got empty")
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
			if !strings.Contains(recorded.Path, tt.expectedInPath) {
				t.Fatalf("expected path %q to contain %q", recorded.Path, tt.expectedInPath)
			}
			if recorded.RawQuery != "" {
				t.Fatalf("expected empty query, got %q", recorded.RawQuery)
			}
		})
	}
}

func TestAuthorizationHeader(t *testing.T) {
	// Create a test server that checks the Authorization header
	called := false
	var receivedAuth string

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer testServer.Close()

	expectedToken := "test-bearer-token"
	_, err := absGET(context.Background(), testServer.URL, expectedToken, "/test")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !called {
		t.Error("test server was not called")
	}

	expectedAuth := "Bearer " + expectedToken
	if receivedAuth != expectedAuth {
		t.Errorf("expected Authorization header %q, got %q", expectedAuth, receivedAuth)
	}
}

func TestContextCancellation(t *testing.T) {
	// Create a server that delays response
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer testServer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := absGET(ctx, testServer.URL, "token", "/test")
	if err == nil {
		t.Error("expected error from cancelled context, got nil")
	}
}

func TestABSPOST(t *testing.T) {
	mockServer := setupMockABSServer()
	defer mockServer.Close()

	tests := []struct {
		name        string
		path        string
		payload     map[string]interface{}
		expectError bool
		checkBody   func([]byte) error
	}{
		{
			name: "successful POST with payload",
			path: "/api/libraries",
			payload: map[string]interface{}{
				"name": "Test Library",
				"folders": []map[string]interface{}{
					{"fullPath": "/audiobooks"},
				},
			},
			expectError: false,
			checkBody: func(body []byte) error {
				if !strings.Contains(string(body), "new-lib-123") {
					return fmt.Errorf("expected 'new-lib-123' in body, got: %s", string(body))
				}
				if !strings.Contains(string(body), "Test Library") {
					return fmt.Errorf("expected 'Test Library' in body, got: %s", string(body))
				}
				return nil
			},
		},
		{
			name:        "POST with nil payload",
			path:        "/api/libraries",
			payload:     nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := absPOST(context.Background(), mockServer.URL, "test-token", tt.path, tt.payload)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if tt.checkBody != nil {
					if err := tt.checkBody(body); err != nil {
						t.Error(err)
					}
				}
			}
		})
	}
}

func TestCreateLibraryHandler(t *testing.T) {
	s := newMCPServer()

	t.Run("create library successfully", func(t *testing.T) {
		recorder := newRecordingServer(func(w http.ResponseWriter, r *http.Request) {
			if !requireMethod(w, r, http.MethodPost) {
				return
			}
			if r.URL.Path != "/api/libraries" {
				http.NotFound(w, r)
				return
			}

			var payload map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, "invalid json", http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":      "new-lib-123",
				"name":    payload["name"],
				"folders": payload["folders"],
			})
		})
		defer recorder.Close()

		result := callRegisteredTool(t, s, "create_library", map[string]interface{}{
			"base_url":   recorder.URL,
			"token":      "test-token",
			"name":       "My Audiobooks",
			"folders":    "/audiobooks, /more-audiobooks",
			"media_type": "book",
			"icon":       "audiobooks",
			"provider":   "audible",
		})
		if result.IsError {
			t.Fatalf("result returned error: %v", result)
		}
		if len(result.Content) == 0 {
			t.Fatal("expected content, got empty")
		}
		if textContent, ok := result.Content[0].(mcp.TextContent); ok {
			if !strings.Contains(textContent.Text, "My Audiobooks") {
				t.Fatalf("expected response to contain library name, got: %s", textContent.Text)
			}
		} else {
			t.Fatalf("expected text content, got %T", result.Content[0])
		}

		recorded, ok := recorder.LastRequest()
		if !ok {
			t.Fatal("expected recorded request")
		}
		if recorded.Method != http.MethodPost {
			t.Fatalf("expected POST request, got %s", recorded.Method)
		}
		if recorded.Path != "/api/libraries" {
			t.Fatalf("expected /api/libraries path, got %s", recorded.Path)
		}
		if recorded.RawQuery != "" {
			t.Fatalf("expected empty query, got %q", recorded.RawQuery)
		}

		var payload map[string]interface{}
		if err := json.Unmarshal(recorded.Body, &payload); err != nil {
			t.Fatalf("decode recorded payload: %v", err)
		}
		if payload["name"] != "My Audiobooks" {
			t.Fatalf("expected name payload, got %#v", payload["name"])
		}
		if payload["mediaType"] != "book" {
			t.Fatalf("expected mediaType book, got %#v", payload["mediaType"])
		}
		if payload["icon"] != "audiobooks" {
			t.Fatalf("expected icon audiobooks, got %#v", payload["icon"])
		}
		if payload["provider"] != "audible" {
			t.Fatalf("expected provider audible, got %#v", payload["provider"])
		}
		folders, ok := payload["folders"].([]interface{})
		if !ok || len(folders) != 2 {
			t.Fatalf("expected two folders, got %#v", payload["folders"])
		}
		for i, expected := range []string{"/audiobooks", "/more-audiobooks"} {
			folder, ok := folders[i].(map[string]interface{})
			if !ok {
				t.Fatalf("folder %d has unexpected type %T", i, folders[i])
			}
			if folder["fullPath"] != expected {
				t.Fatalf("expected folder %d fullPath %q, got %#v", i, expected, folder["fullPath"])
			}
		}
	})

	t.Run("missing required media_type parameter", func(t *testing.T) {
		recorder := newRecordingServer(func(w http.ResponseWriter, r *http.Request) {
			t.Fatalf("server should not be called when media_type is missing")
		})
		defer recorder.Close()

		result := callRegisteredTool(t, s, "create_library", map[string]interface{}{
			"base_url": recorder.URL,
			"token":    "test-token",
			"name":     "My Library",
			"folders":  "/audiobooks",
		})
		if !result.IsError {
			t.Fatal("expected missing media_type to return a tool error")
		}
		if requests := recorder.Requests(); len(requests) != 0 {
			t.Fatalf("expected no outbound request, got %d", len(requests))
		}
	})

	t.Run("missing required name parameter", func(t *testing.T) {
		result := callRegisteredTool(t, s, "create_library", map[string]interface{}{
			"base_url":   "http://example.invalid",
			"token":      "test-token",
			"folders":    "/audiobooks",
			"media_type": "book",
		})
		if !result.IsError {
			t.Fatal("expected missing name to return a tool error")
		}
	})

	t.Run("missing required folders parameter", func(t *testing.T) {
		result := callRegisteredTool(t, s, "create_library", map[string]interface{}{
			"base_url":   "http://example.invalid",
			"token":      "test-token",
			"name":       "My Library",
			"media_type": "book",
		})
		if !result.IsError {
			t.Fatal("expected missing folders to return a tool error")
		}
	})
}

func TestRegisteredToolNames(t *testing.T) {
	s := newMCPServer()
	tools := s.ListTools()

	expected := []string{
		"libraries",
		"library",
		"create_library",
		"item",
		"author",
		"me",
		"sessions",
		"session",
		"podcasts",
		"podcast",
		"collections",
		"collection",
		"create_collection",
		"add_to_collection",
		"playlists",
		"playlist",
		"create_playlist",
		"add_to_playlist",
		"check_podcast_episodes",
		"create_backup",
		"update_progress",
		"ping",
		"healthcheck",
		"status",
		"users",
		"users_online",
		"user",
		"series",
		"author_image",
		"backups",
		"filesystem",
		"authorize",
		"tags",
		"genres",
	}

	if len(tools) != len(expected) {
		t.Fatalf("expected %d registered tools, got %d", len(expected), len(tools))
	}
	for _, name := range expected {
		tool, ok := tools[name]
		if !ok {
			t.Fatalf("expected tool %q to be registered", name)
		}
		if tool.Tool.Name != name {
			t.Fatalf("expected registered tool %q to report name %q, got %q", name, name, tool.Tool.Name)
		}
		if tool.Handler == nil {
			t.Fatalf("expected tool %q to have a handler", name)
		}
	}
}
