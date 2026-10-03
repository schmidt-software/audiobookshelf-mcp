package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func TestRegisteredImageToolSchemas(t *testing.T) {
	s := newMCPServer()

	itemTool := s.GetTool("item")
	if itemTool == nil {
		t.Fatal("expected item tool to be registered")
	}
	if itemTool.Tool.Description != "Retrieve a single Audiobookshelf item (audiobook or podcast) by ID, optionally with sub-resources" {
		t.Fatalf("unexpected item tool description: %q", itemTool.Tool.Description)
	}
	requireToolProperty(t, s, "item", "item_id", "string", "Item identifier to fetch", true)
	requireToolProperty(t, s, "item", "cover", "boolean", "Return the cover image for the item as MCP image content", false)
	requireToolProperty(t, s, "item", "tone-object", "boolean", "Include tone object for the item", false)

	authorImageTool := s.GetTool("author_image")
	if authorImageTool == nil {
		t.Fatal("expected author_image tool to be registered")
	}
	if authorImageTool.Tool.Description != "Retrieve an author image by ID as MCP image content" {
		t.Fatalf("unexpected author_image tool description: %q", authorImageTool.Tool.Description)
	}
	requireToolProperty(t, s, "author_image", "author_id", "string", "Author identifier", true)
}

func TestRegisteredImageToolsReturnImageContent(t *testing.T) {
	pngBytes := testPNGBytes(t)
	jpegBytes := testJPEGBytes(t)

	recorder := newRecordingServer(func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		switch r.URL.Path {
		case "/api/items/item123/cover":
			w.Header().Set("Content-Type", "image/png; charset=binary")
			w.WriteHeader(http.StatusOK)
			w.Write(pngBytes)
		case "/api/authors/author123/image":
			w.Header().Set("Content-Type", "image/jpeg; q=1")
			w.WriteHeader(http.StatusOK)
			w.Write(jpegBytes)
		default:
			http.NotFound(w, r)
		}
	})
	defer recorder.Close()

	s := newMCPServer()

	tests := []struct {
		name     string
		toolName string
		params   map[string]interface{}
		path     string
		mimeType string
		body     []byte
	}{
		{
			name:     "item cover returns PNG image content",
			toolName: "item",
			params: map[string]interface{}{
				"base_url": recorder.URL,
				"token":    "test-token",
				"item_id":  "item123",
				"cover":    true,
			},
			path:     "/api/items/item123/cover",
			mimeType: "image/png",
			body:     pngBytes,
		},
		{
			name:     "author image returns JPEG image content",
			toolName: "author_image",
			params: map[string]interface{}{
				"base_url":  recorder.URL,
				"token":     "test-token",
				"author_id": "author123",
			},
			path:     "/api/authors/author123/image",
			mimeType: "image/jpeg",
			body:     jpegBytes,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := len(recorder.Requests())
			result := callRegisteredTool(t, s, tt.toolName, tt.params)
			if result == nil || result.IsError {
				t.Fatalf("expected successful result, got %#v", result)
			}

			imageContent := requireImageContent(t, result)
			if imageContent.MIMEType != tt.mimeType {
				t.Fatalf("expected MIME type %q, got %q", tt.mimeType, imageContent.MIMEType)
			}
			decoded, err := base64.StdEncoding.DecodeString(imageContent.Data)
			if err != nil {
				t.Fatalf("image data is not valid base64: %v", err)
			}
			if !bytes.Equal(decoded, tt.body) {
				t.Fatal("decoded image bytes do not match original bytes")
			}

			recorded := requireNewRecordedRequest(t, recorder, before)
			requireRecordedGET(t, recorded, tt.path)
		})
	}
}

func TestRegisteredItemToolTextResponsesStayText(t *testing.T) {
	responses := map[string]string{
		"/api/items/item123":             `{"id":"item123","type":"book"}`,
		"/api/items/item123/tone-object": `{"tone":"warm"}`,
	}
	recorder := newRecordingServer(func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		body, ok := responses[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(body))
	})
	defer recorder.Close()

	s := newMCPServer()
	tests := []struct {
		name     string
		params   map[string]interface{}
		path     string
		expected string
	}{
		{
			name: "item without cover",
			params: map[string]interface{}{
				"base_url": recorder.URL,
				"token":    "test-token",
				"item_id":  "item123",
			},
			path:     "/api/items/item123",
			expected: responses["/api/items/item123"],
		},
		{
			name: "item tone object",
			params: map[string]interface{}{
				"base_url":    recorder.URL,
				"token":       "test-token",
				"item_id":     "item123",
				"tone-object": true,
			},
			path:     "/api/items/item123/tone-object",
			expected: responses["/api/items/item123/tone-object"],
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := len(recorder.Requests())
			result := callRegisteredTool(t, s, "item", tt.params)
			if result.IsError {
				t.Fatalf("expected successful result, got %#v", result)
			}
			textContent := requireTextContent(t, result)
			if textContent.Text != tt.expected {
				t.Fatalf("expected unchanged text %q, got %q", tt.expected, textContent.Text)
			}

			recorded := requireNewRecordedRequest(t, recorder, before)
			requireRecordedGET(t, recorded, tt.path)
		})
	}
}

func TestRegisteredImageToolsDetectMissingContentType(t *testing.T) {
	pngBytes := testPNGBytes(t)
	recorder := newRecordingServer(func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		w.Header().Set("Content-Type", "")
		w.WriteHeader(http.StatusOK)
		w.Write(pngBytes)
	})
	defer recorder.Close()

	result := callRegisteredTool(t, newMCPServer(), "author_image", map[string]interface{}{
		"base_url":  recorder.URL,
		"token":     "test-token",
		"author_id": "author123",
	})
	if result.IsError {
		t.Fatalf("expected successful result, got %#v", result)
	}
	imageContent := requireImageContent(t, result)
	if imageContent.MIMEType != "image/png" {
		t.Fatalf("expected detected MIME type image/png, got %q", imageContent.MIMEType)
	}
	decoded, err := base64.StdEncoding.DecodeString(imageContent.Data)
	if err != nil {
		t.Fatalf("image data is not valid base64: %v", err)
	}
	if !bytes.Equal(decoded, pngBytes) {
		t.Fatal("decoded image bytes do not match original bytes")
	}
}

func TestRegisteredImageToolsReturnTextForNonImages(t *testing.T) {
	jsonBody := `{"error":"not found"}`
	recorder := newRecordingServer(func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(jsonBody))
	})
	defer recorder.Close()

	s := newMCPServer()
	tests := []struct {
		name     string
		toolName string
		params   map[string]interface{}
		path     string
	}{
		{
			name:     "item cover",
			toolName: "item",
			params: map[string]interface{}{
				"base_url": recorder.URL,
				"token":    "test-token",
				"item_id":  "item123",
				"cover":    true,
			},
			path: "/api/items/item123/cover",
		},
		{
			name:     "item without cover",
			toolName: "item",
			params: map[string]interface{}{
				"base_url": recorder.URL,
				"token":    "test-token",
				"item_id":  "item123",
			},
			path: "/api/items/item123",
		},
		{
			name:     "author image",
			toolName: "author_image",
			params: map[string]interface{}{
				"base_url":  recorder.URL,
				"token":     "test-token",
				"author_id": "author123",
			},
			path: "/api/authors/author123/image",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := len(recorder.Requests())
			result := callRegisteredTool(t, s, tt.toolName, tt.params)
			if result == nil || result.IsError {
				t.Fatalf("expected successful result, got %#v", result)
			}
			textContent := requireTextContent(t, result)
			if textContent.Text != jsonBody {
				t.Fatalf("expected text %q, got %q", jsonBody, textContent.Text)
			}

			recorded := requireNewRecordedRequest(t, recorder, before)
			requireRecordedGET(t, recorded, tt.path)
		})
	}
}

func TestRegisteredImageToolsValidateBeforeRequest(t *testing.T) {
	t.Setenv("ABS_BASE_URL", "")
	t.Setenv("ABS_API_KEY", "")

	recorder := newRecordingServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("server should not be called for validation errors")
	})
	defer recorder.Close()

	s := newMCPServer()
	tests := []struct {
		name     string
		toolName string
		params   map[string]interface{}
	}{
		{
			name:     "item missing base_url",
			toolName: "item",
			params: map[string]interface{}{
				"token":   "test-token",
				"item_id": "item123",
				"cover":   true,
			},
		},
		{
			name:     "item missing token",
			toolName: "item",
			params: map[string]interface{}{
				"base_url": recorder.URL,
				"item_id":  "item123",
				"cover":    true,
			},
		},
		{
			name:     "item missing ID",
			toolName: "item",
			params: map[string]interface{}{
				"base_url": recorder.URL,
				"token":    "test-token",
				"cover":    true,
			},
		},
		{
			name:     "item invalid ID type",
			toolName: "item",
			params: map[string]interface{}{
				"base_url": recorder.URL,
				"token":    "test-token",
				"item_id":  123,
				"cover":    true,
			},
		},
		{
			name:     "author_image missing base_url",
			toolName: "author_image",
			params: map[string]interface{}{
				"token":     "test-token",
				"author_id": "author123",
			},
		},
		{
			name:     "author_image missing token",
			toolName: "author_image",
			params: map[string]interface{}{
				"base_url":  recorder.URL,
				"author_id": "author123",
			},
		},
		{
			name:     "author_image missing ID",
			toolName: "author_image",
			params: map[string]interface{}{
				"base_url": recorder.URL,
				"token":    "test-token",
			},
		},
		{
			name:     "author_image invalid ID type",
			toolName: "author_image",
			params: map[string]interface{}{
				"base_url":  recorder.URL,
				"token":     "test-token",
				"author_id": 123,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := callRegisteredTool(t, s, tt.toolName, tt.params)
			if !result.IsError {
				t.Fatalf("expected validation error, got %#v", result)
			}
			if requests := recorder.Requests(); len(requests) != 0 {
				t.Fatalf("expected no outbound requests, got %d", len(requests))
			}
		})
	}
}

func TestRegisteredImageToolsReturnErrorsForABSFailures(t *testing.T) {
	recorder := newRecordingServer(func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		http.Error(w, `{"error":"boom"}`, http.StatusBadGateway)
	})
	defer recorder.Close()

	s := newMCPServer()
	tests := []struct {
		name     string
		toolName string
		params   map[string]interface{}
		path     string
	}{
		{
			name:     "item cover",
			toolName: "item",
			params: map[string]interface{}{
				"base_url": recorder.URL,
				"token":    "test-token",
				"item_id":  "item123",
				"cover":    true,
			},
			path: "/api/items/item123/cover",
		},
		{
			name:     "item without cover",
			toolName: "item",
			params: map[string]interface{}{
				"base_url": recorder.URL,
				"token":    "test-token",
				"item_id":  "item123",
			},
			path: "/api/items/item123",
		},
		{
			name:     "author image",
			toolName: "author_image",
			params: map[string]interface{}{
				"base_url":  recorder.URL,
				"token":     "test-token",
				"author_id": "author123",
			},
			path: "/api/authors/author123/image",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := len(recorder.Requests())
			result := callRegisteredTool(t, s, tt.toolName, tt.params)
			if !result.IsError {
				t.Fatalf("expected ABS failure to return tool error, got %#v", result)
			}
			textContent := requireTextContent(t, result)
			if !strings.Contains(textContent.Text, "ABS API returned 502 Bad Gateway") {
				t.Fatalf("expected ABS status in error text, got %q", textContent.Text)
			}

			recorded := requireNewRecordedRequest(t, recorder, before)
			requireRecordedGET(t, recorded, tt.path)
		})
	}
}

func TestABSGETRawErrorPaths(t *testing.T) {
	t.Run("build request error", func(t *testing.T) {
		_, err := absGETRaw(contextBackground(), "http://[::1", "test-token", "/cover")
		if err == nil || !strings.Contains(err.Error(), "build request") {
			t.Fatalf("expected build request error, got %v", err)
		}
	})

	t.Run("read response error", func(t *testing.T) {
		originalClient := httpClient
		httpClient = &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Header:     make(http.Header),
					Body:       errReadCloser{},
				}, nil
			}),
		}
		t.Cleanup(func() {
			httpClient = originalClient
		})

		_, err := absGETRaw(contextBackground(), "http://example.test", "test-token", "/cover")
		if err == nil || !strings.Contains(err.Error(), "read response") {
			t.Fatalf("expected read response error, got %v", err)
		}
	})
}

func testPNGBytes(t *testing.T) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	img.Set(1, 0, color.RGBA{G: 255, A: 255})
	img.Set(0, 1, color.RGBA{B: 255, A: 255})
	img.Set(1, 1, color.RGBA{R: 255, G: 255, A: 255})

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode PNG: %v", err)
	}
	return buf.Bytes()
}

func testJPEGBytes(t *testing.T) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	img.Set(1, 0, color.RGBA{G: 255, A: 255})
	img.Set(0, 1, color.RGBA{B: 255, A: 255})
	img.Set(1, 1, color.RGBA{R: 255, G: 255, A: 255})

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("encode JPEG: %v", err)
	}
	return buf.Bytes()
}

func requireImageContent(t *testing.T, result *mcp.CallToolResult) mcp.ImageContent {
	t.Helper()

	for _, content := range result.Content {
		if imageContent, ok := content.(mcp.ImageContent); ok {
			return imageContent
		}
	}
	t.Fatalf("expected image content, got %#v", result.Content)
	return mcp.ImageContent{}
}

func requireTextContent(t *testing.T, result *mcp.CallToolResult) mcp.TextContent {
	t.Helper()

	if len(result.Content) != 1 {
		t.Fatalf("expected one text content item, got %d", len(result.Content))
	}
	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected text content, got %T", result.Content[0])
	}
	return textContent
}

func requireNewRecordedRequest(t *testing.T, recorder *recordingServer, before int) recordedRequest {
	t.Helper()

	requests := recorder.Requests()
	if len(requests) != before+1 {
		t.Fatalf("expected one new recorded request, got %d new requests", len(requests)-before)
	}
	return requests[before]
}

func requireRecordedGET(t *testing.T, recorded recordedRequest, expectedPath string) {
	t.Helper()

	if recorded.Method != http.MethodGet {
		t.Fatalf("expected GET request, got %s", recorded.Method)
	}
	if recorded.Path != expectedPath {
		t.Fatalf("expected path %q, got %q", expectedPath, recorded.Path)
	}
	if recorded.RawQuery != "" {
		t.Fatalf("expected empty query, got %q", recorded.RawQuery)
	}
	if len(recorded.Body) != 0 {
		t.Fatalf("expected empty request body, got %q", string(recorded.Body))
	}
}

func requireToolProperty(t *testing.T, s *server.MCPServer, toolName, propertyName, propertyType, description string, required bool) {
	t.Helper()

	tool := s.GetTool(toolName)
	if tool == nil {
		t.Fatalf("tool %q is not registered", toolName)
	}
	property, ok := tool.Tool.InputSchema.Properties[propertyName]
	if !ok {
		t.Fatalf("tool %q schema is missing property %q", toolName, propertyName)
	}
	propertySchema, ok := property.(map[string]interface{})
	if !ok {
		t.Fatalf("tool %q property %q has unexpected schema type %T", toolName, propertyName, property)
	}
	if propertySchema["type"] != propertyType {
		t.Fatalf("tool %q property %q expected type %q, got %#v", toolName, propertyName, propertyType, propertySchema["type"])
	}
	if propertySchema["description"] != description {
		t.Fatalf("tool %q property %q expected description %q, got %#v", toolName, propertyName, description, propertySchema["description"])
	}
	if hasRequired(tool.Tool.InputSchema.Required, propertyName) != required {
		t.Fatalf("tool %q property %q required=%v, want %v", toolName, propertyName, hasRequired(tool.Tool.InputSchema.Required, propertyName), required)
	}
}

func hasRequired(required []string, propertyName string) bool {
	for _, name := range required {
		if name == propertyName {
			return true
		}
	}
	return false
}

func TestImageToolResultsMarshalExpectedContent(t *testing.T) {
	result := newBinaryAwareToolResult(&absResponse{
		Body:        []byte("not an image"),
		ContentType: "text/plain",
	})
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal text result: %v", err)
	}
	if !bytes.Contains(data, []byte(`"type":"text"`)) {
		t.Fatalf("expected marshaled text result, got %s", string(data))
	}

	pngBytes := []byte("\x89PNG\r\n\x1a\n")
	imageResult := newBinaryAwareToolResult(&absResponse{
		Body:        pngBytes,
		ContentType: "image/png; =bad",
	})
	imageContent := requireImageContent(t, imageResult)
	if imageContent.MIMEType != "image/png" {
		t.Fatalf("expected malformed parameter fallback to strip image/png, got %q", imageContent.MIMEType)
	}
}

type errReadCloser struct{}

func (errReadCloser) Read([]byte) (int, error) {
	return 0, errReadFailed
}

func (errReadCloser) Close() error {
	return nil
}

var errReadFailed = &readFailedError{}

type readFailedError struct{}

func (*readFailedError) Error() string {
	return "read failed"
}

func contextBackground() context.Context {
	return context.Background()
}
