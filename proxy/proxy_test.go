package proxy

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestProxyInjectsScript(t *testing.T) {
	// Create a backend server that returns HTML
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<html><body><h1>Hello</h1></body></html>`))
	}))
	defer backend.Close()

	p, err := New(":0", backend.URL)
	if err != nil {
		t.Fatal(err)
	}

	// Use the proxy's modifyResponse directly via a test reverse proxy
	resp := &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
		Body:       io.NopCloser(strings.NewReader(`<html><body><h1>Hello</h1></body></html>`)),
	}

	err = p.modifyResponse(resp)
	if err != nil {
		t.Fatal(err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	bodyStr := string(body)
	if !strings.Contains(bodyStr, "__hotreload_sse") {
		t.Error("Expected injected SSE script in response body")
	}
	if !strings.Contains(bodyStr, "<h1>Hello</h1>") {
		t.Error("Expected original content to be preserved")
	}
	if !strings.Contains(bodyStr, "</body>") {
		t.Error("Expected closing body tag to be preserved")
	}
}

func TestProxySkipsNonHTML(t *testing.T) {
	originalBody := `{"status": "ok"}`
	resp := &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(originalBody)),
	}

	p, _ := New(":0", "http://localhost:9999")
	err := p.modifyResponse(resp)
	if err != nil {
		t.Fatal(err)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != originalBody {
		t.Errorf("Expected JSON body to be unchanged, got: %s", string(body))
	}
}

func TestProxySSEHandler(t *testing.T) {
	p, err := New(":0", "http://localhost:9999")
	if err != nil {
		t.Fatal(err)
	}

	// Test SSE endpoint
	req := httptest.NewRequest("GET", "/__hotreload_sse", nil)
	w := httptest.NewRecorder()

	// Run SSE handler in background — it blocks until reload or client disconnect
	done := make(chan struct{})
	go func() {
		p.sseHandler(w, req)
		close(done)
	}()

	// Give handler time to register and send initial message
	time.Sleep(100 * time.Millisecond)

	// Verify a client was registered
	p.mu.Lock()
	clientCount := len(p.clients)
	p.mu.Unlock()

	if clientCount != 1 {
		t.Fatalf("Expected 1 SSE client, got %d", clientCount)
	}

	// Broadcast reload — should cause handler to return
	p.BroadcastReload()

	select {
	case <-done:
		// Handler returned as expected
	case <-time.After(2 * time.Second):
		t.Fatal("SSE handler did not return after broadcast")
	}

	body := w.Body.String()
	if !strings.Contains(body, "data: connected") {
		t.Error("Expected 'data: connected' in SSE stream")
	}
	if !strings.Contains(body, "data: reload") {
		t.Error("Expected 'data: reload' in SSE stream")
	}
}

func TestProxyBroadcastWithNoClients(t *testing.T) {
	p, err := New(":0", "http://localhost:9999")
	if err != nil {
		t.Fatal(err)
	}

	// Should not panic with no clients
	p.BroadcastReload()
}

func TestProxyScriptInjectionPosition(t *testing.T) {
	p, _ := New(":0", "http://localhost:9999")

	html := `<html><head><title>Test</title></head><body><p>Content</p></body></html>`
	resp := &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"text/html"}},
		Body:       io.NopCloser(strings.NewReader(html)),
	}

	if err := p.modifyResponse(resp); err != nil {
		t.Fatal(err)
	}

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	// Script should be injected before </body>
	scriptIdx := strings.Index(bodyStr, "<script>")
	bodyCloseIdx := strings.Index(bodyStr, "</body>")

	if scriptIdx == -1 {
		t.Fatal("Script not found in response")
	}
	if bodyCloseIdx == -1 {
		t.Fatal("</body> not found in response")
	}
	if scriptIdx >= bodyCloseIdx {
		t.Error("Script should be injected before </body>")
	}
}

func TestProxyErrorHandler(t *testing.T) {
	p, _ := New(":0", "http://localhost:9999")

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	// Simulate a backend error
	p.errorHandler(w, req, fmt.Errorf("connection refused"))

	if w.Code != http.StatusBadGateway {
		t.Errorf("Expected status 502, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Rebuilding") {
		t.Error("Expected retry page with 'Rebuilding' text")
	}
	if !strings.Contains(body, "<script>") {
		t.Error("Expected auto-retry script in error page")
	}
	if !strings.Contains(body, "window.location.reload") {
		t.Error("Expected auto-reload logic in error page")
	}
}
