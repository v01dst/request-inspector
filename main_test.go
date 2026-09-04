package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDescribesRequest(t *testing.T) {
	req := httptest.NewRequest("POST", "/some/path?x=1&y=2", strings.NewReader("hello body"))
	req.Header.Set("User-Agent", "test-agent/1.0")
	req.Header.Set("X-Custom", "abc")
	req.AddCookie(&http.Cookie{Name: "session", Value: "s3cr3t"})
	req.Header.Set("Cookie", "session=s3cr3t")

	info := describe(req, true, 1<<20)

	if info.Method != "POST" {
		t.Fatalf("method = %q", info.Method)
	}
	if info.Path != "/some/path" {
		t.Fatalf("path = %q", info.Path)
	}
	if info.Query != "x=1&y=2" {
		t.Fatalf("query = %q", info.Query)
	}
	if info.Body != "hello body" {
		t.Fatalf("body = %q", info.Body)
	}
	if info.BodyBytes != 10 {
		t.Fatalf("bodyBytes = %d", info.BodyBytes)
	}
	if info.UserAgent != "test-agent/1.0" {
		t.Fatalf("ua = %q", info.UserAgent)
	}
	if info.Headers["x-custom"] != "abc" {
		t.Fatalf("headers = %v", info.Headers)
	}
	if info.Cookies["session"] != "s3cr3t" {
		t.Fatalf("cookies = %v", info.Cookies)
	}
	if info.TLS {
		t.Fatal("httptest request should not be TLS")
	}
}

func TestXForwardedForPreferred(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.9, 10.0.0.2")

	info := describe(req, false, 0)
	if info.RemoteIP != "203.0.113.9" {
		t.Fatalf("remoteIp = %q", info.RemoteIP)
	}
}

func TestBinaryBodyFlagged(t *testing.T) {
	body := bytes.Repeat([]byte{0x00, 0x01, 0x02}, 8)
	req := httptest.NewRequest("POST", "/upload", bytes.NewReader(body))

	info := describe(req, true, 1<<20)
	if !strings.Contains(info.Body, "binary") {
		t.Fatalf("body = %q, want binary marker", info.Body)
	}
}

func TestHandlerEndpoints(t *testing.T) {
	h := handler(1 << 20)

	// health
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/health", nil))
	if rec.Code != 200 {
		t.Fatalf("health = %d", rec.Code)
	}

	// root
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	var root map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &root); err != nil {
		t.Fatal(err)
	}
	if root["service"] != "request-inspector" {
		t.Fatalf("root = %v", root)
	}

	// echo
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("PUT", "/anything/here", nil))
	if rec.Code != 200 {
		t.Fatalf("echo = %d", rec.Code)
	}
	var info RequestInfo
	if err := json.Unmarshal(rec.Body.Bytes(), &info); err != nil {
		t.Fatal(err)
	}
	if info.Method != "PUT" || info.Path != "/anything/here" {
		t.Fatalf("echo info = %+v", info)
	}
}

func TestBodyTooLarge(t *testing.T) {
	h := handler(16)
	req := httptest.NewRequest("POST", "/x", strings.NewReader(strings.Repeat("a", 100)))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 413 {
		t.Fatalf("want 413, got %d", rec.Code)
	}
}
