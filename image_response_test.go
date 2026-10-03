package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestImageEndpointHandlersReturnImageContent(t *testing.T) {
	pngBytes := testPNGBytes(t)
	jpegBytes := testJPEGBytes(t)

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/items/item123/cover":
			w.Header().Set("Content-Type", "image/png; charset=binary")
			w.WriteHeader(http.StatusOK)
			w.Write(pngBytes)
		case "/api/authors/author123/image":
			w.Header().Set("Content-Type", "image/jpeg")
			w.WriteHeader(http.StatusOK)
			w.Write(jpegBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	defer testServer.Close()

	baseURL := strings.TrimSuffix(testServer.URL, "/api")

	tests := []struct {
		name     string
		handler  func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
		params   map[string]interface{}
		mimeType string
		body     []byte
	}{
		{
			name:    "item cover returns PNG image content",
			handler: createGETByIDWithBinarySubResourceHandler("/items/%s", "item_id", []string{"cover", "tone-object"}, map[string]bool{"cover": true}),
			params: map[string]interface{}{
				"base_url": baseURL,
				"token":    "test-token",
				"item_id":  "item123",
				"cover":    true,
			},
			mimeType: "image/png",
			body:     pngBytes,
		},
		{
			name:    "author image returns JPEG image content",
			handler: createGETByIDImageHandler("/authors/%s/image", "author_id"),
			params: map[string]interface{}{
				"base_url":  baseURL,
				"token":     "test-token",
				"author_id": "author123",
			},
			mimeType: "image/jpeg",
			body:     jpegBytes,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.handler(context.Background(), makeRequest(tt.params))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
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
		})
	}
}

func TestImageEndpointHandlersReturnTextForNonImages(t *testing.T) {
	jsonBody := `{"error":"not found"}`
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(jsonBody))
	}))
	defer testServer.Close()

	baseURL := strings.TrimSuffix(testServer.URL, "/api")
	tests := []struct {
		name    string
		handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
		params  map[string]interface{}
	}{
		{
			name:    "item cover",
			handler: createGETByIDWithBinarySubResourceHandler("/items/%s", "item_id", []string{"cover", "tone-object"}, map[string]bool{"cover": true}),
			params: map[string]interface{}{
				"base_url": baseURL,
				"token":    "test-token",
				"item_id":  "item123",
				"cover":    true,
			},
		},
		{
			name:    "author image",
			handler: createGETByIDImageHandler("/authors/%s/image", "author_id"),
			params: map[string]interface{}{
				"base_url":  baseURL,
				"token":     "test-token",
				"author_id": "author123",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.handler(context.Background(), makeRequest(tt.params))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result == nil || result.IsError {
				t.Fatalf("expected successful result, got %#v", result)
			}
			if len(result.Content) != 1 {
				t.Fatalf("expected one text content item, got %d", len(result.Content))
			}
			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatalf("expected text content, got %T", result.Content[0])
			}
			if textContent.Text != jsonBody {
				t.Fatalf("expected text %q, got %q", jsonBody, textContent.Text)
			}
		})
	}
}

func TestBinaryAwareResultDetectsImageMIMETypeFallback(t *testing.T) {
	pngBytes := testPNGBytes(t)
	result := newBinaryAwareToolResult(&absResponse{Body: pngBytes})

	imageContent := requireImageContent(t, result)
	if imageContent.MIMEType != "image/png" {
		t.Fatalf("expected detected MIME type image/png, got %q", imageContent.MIMEType)
	}
}

func TestItemJSONResponseStaysText(t *testing.T) {
	jsonBody := `{"id":"item123","type":"book"}`
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/items/item123" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(jsonBody))
	}))
	defer testServer.Close()

	baseURL := strings.TrimSuffix(testServer.URL, "/api")
	result, err := createGETByIDWithBinarySubResourceHandler("/items/%s", "item_id", []string{"cover", "tone-object"}, map[string]bool{"cover": true})(context.Background(), makeRequest(map[string]interface{}{
		"base_url": baseURL,
		"token":    "test-token",
		"item_id":  "item123",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("expected successful result, got %#v", result)
	}
	if len(result.Content) != 1 {
		t.Fatalf("expected one text content item, got %d", len(result.Content))
	}
	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected text content, got %T", result.Content[0])
	}
	if textContent.Text != jsonBody {
		t.Fatalf("expected unchanged JSON text %q, got %q", jsonBody, textContent.Text)
	}
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
