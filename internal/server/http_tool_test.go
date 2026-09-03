package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestServer() *MCPServer {
	return &MCPServer{logger: slog.Default()}
}

func TestHandleHTTPRequestGET(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom", "value")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "hello world")
	}))
	defer srv.Close()

	res, err := newTestServer().handleHTTPRequest(map[string]interface{}{"url": srv.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error result: %s", resultText(t, res))
	}
	text := resultText(t, res)
	if !strings.Contains(text, "200 OK") {
		t.Errorf("missing status line: %q", text)
	}
	if !strings.Contains(text, "X-Custom: value") {
		t.Errorf("missing response header: %q", text)
	}
	if !strings.Contains(text, "hello world") {
		t.Errorf("missing body: %q", text)
	}
}

func TestHandleHTTPRequestPOSTEcho(t *testing.T) {
	var gotMethod, gotHeader, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotHeader = r.Header.Get("X-Api")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		io.WriteString(w, "ok")
	}))
	defer srv.Close()

	res, err := newTestServer().handleHTTPRequest(map[string]interface{}{
		"url":     srv.URL,
		"method":  "post",
		"headers": map[string]interface{}{"X-Api": "1"},
		"body":    `{"k":"v"}`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error result: %s", resultText(t, res))
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotHeader != "1" {
		t.Errorf("X-Api = %q, want 1", gotHeader)
	}
	if gotBody != `{"k":"v"}` {
		t.Errorf("body = %q", gotBody)
	}
}

func TestHandleHTTPRequestHostHeader(t *testing.T) {
	var gotHost string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHost = r.Host
	}))
	defer srv.Close()

	_, err := newTestServer().handleHTTPRequest(map[string]interface{}{
		"url":     srv.URL,
		"headers": map[string]interface{}{"Host": "example.internal"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotHost != "example.internal" {
		t.Errorf("Host = %q, want example.internal", gotHost)
	}
}

func TestHandleHTTPRequestRedirects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/final" {
			io.WriteString(w, "final page")
			return
		}
		http.Redirect(w, r, "/final", http.StatusFound)
	}))
	defer srv.Close()

	// follow_redirects=false -> should see the 302, not the final page.
	res, err := newTestServer().handleHTTPRequest(map[string]interface{}{
		"url":              srv.URL,
		"follow_redirects": false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := resultText(t, res)
	if !strings.Contains(text, "302") {
		t.Errorf("expected 302 status, got: %q", text)
	}
	if strings.Contains(text, "final page") {
		t.Errorf("should not have followed redirect: %q", text)
	}

	// default (follow) -> should reach the final page.
	res, err = newTestServer().handleHTTPRequest(map[string]interface{}{"url": srv.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text = resultText(t, res)
	if !strings.Contains(text, "final page") {
		t.Errorf("expected to follow redirect to final page, got: %q", text)
	}
}

func TestHandleHTTPRequestInsecureSkipVerify(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "secure ok")
	}))
	defer srv.Close()

	// Default: verification on -> self-signed cert fails.
	res, err := newTestServer().handleHTTPRequest(map[string]interface{}{"url": srv.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Errorf("expected TLS verification error, got: %s", resultText(t, res))
	}

	// Skip verify -> succeeds.
	res, err = newTestServer().handleHTTPRequest(map[string]interface{}{
		"url":                  srv.URL,
		"insecure_skip_verify": true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error result: %s", resultText(t, res))
	}
	if !strings.Contains(resultText(t, res), "secure ok") {
		t.Errorf("missing body: %q", resultText(t, res))
	}
}

func TestHandleHTTPRequestInvalidURL(t *testing.T) {
	res, err := newTestServer().handleHTTPRequest(map[string]interface{}{"url": "http://127.0.0.1:1/nope"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Errorf("expected error result for unreachable host, got: %s", resultText(t, res))
	}
}

func TestHandleHTTPRequestMissingURL(t *testing.T) {
	res, err := newTestServer().handleHTTPRequest(map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Errorf("expected error result for missing url")
	}
}

func TestHandleHTTPRequestInvalidHeaderType(t *testing.T) {
	res, err := newTestServer().handleHTTPRequest(map[string]interface{}{
		"url":     "http://example.com",
		"headers": map[string]interface{}{"X": 1},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Errorf("expected error result for non-string header value")
	}
}
