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
	h := handler(1<<20, NewStats())

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
	h := handler(16, NewStats())
	req := httptest.NewRequest("POST", "/x", strings.NewReader(strings.Repeat("a", 100)))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 413 {
		t.Fatalf("want 413, got %d", rec.Code)
	}
}

func TestStatsTracking(t *testing.T) {
	stats := NewStats()
	h := handler(1<<20, stats)

	for i := 0; i < 3; i++ {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/x", nil))
	}
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/y", nil))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/stats", nil))
	if rec.Code != 200 {
		t.Fatalf("stats = %d", rec.Code)
	}
	var snap map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatal(err)
	}
	if snap["totalRequests"].(float64) != 5 {
		t.Errorf("totalRequests = %v, want 5", snap["totalRequests"])
	}
	if snap["uniqueIps"].(float64) != 1 {
		t.Errorf("uniqueIps = %v", snap["uniqueIps"])
	}
	if snap["perMethod"].(map[string]any)["POST"].(float64) != 3 {
		t.Errorf("perMethod POST = %v", snap["perMethod"])
	}
}

func TestParseUA(t *testing.T) {
	cases := []struct {
		ua      string
		browser string
		os      string
		device  string
	}{
		{"Mozilla/5.0 (Windows NT 10.0) AppleWebKit/537.36 Chrome/120.0 Safari/537.36 Edg/120.0", "Edge", "Windows", "desktop"},
		{"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15) Version/17.0 Safari/605.1.15", "Safari", "macOS", "desktop"},
		{"Mozilla/5.0 (Linux; Android 14; Pixel 8) Chrome/120.0 Mobile", "Chrome", "Android", "mobile"},
		{"Mozilla/5.0 (iPhone; CPU iPhone OS 17_0) Version/17.0 Mobile Safari", "Safari", "iOS", "mobile"},
		{"curl/8.18.0", "curl", "unknown", "other"},
		{"Mozilla/5.0 (compatible; Googlebot/2.1)", "bot", "unknown", "other"},
		{"", "unknown", "unknown", "desktop"},
	}
	for _, c := range cases {
		got := parseUA(c.ua)
		if got.Browser != c.browser || got.OS != c.os || got.Device != c.device {
			t.Errorf("parseUA(%q) = %+v, want %s/%s/%s", c.ua, got, c.browser, c.os, c.device)
		}
	}
}

func TestDescribeIncludesUA(t *testing.T) {
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("User-Agent", "curl/8.18.0")
	info := describe(req, false, 0)
	if info.UA.Browser != "curl" {
		t.Fatalf("ua = %+v", info.UA)
	}
}
