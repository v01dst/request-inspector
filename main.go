package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Stats tracks served requests (in-memory, atomic via mutex).
type Stats struct {
	mu        sync.Mutex
	total     int
	perIP     map[string]int
	perMethod map[string]int
	startedAt time.Time
}

func NewStats() *Stats {
	return &Stats{
		perIP:     map[string]int{},
		perMethod: map[string]int{},
		startedAt: time.Now(),
	}
}

func (s *Stats) record(ip, method string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.total++
	s.perIP[ip]++
	s.perMethod[method]++
}

func (s *Stats) snapshot() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return map[string]any{
		"totalRequests": s.total,
		"uniqueIps":     len(s.perIP),
		"topIps":        topN(s.perIP, 5),
		"perMethod":     s.perMethod,
		"uptimeSec":     int(time.Since(s.startedAt).Seconds()),
	}
}

func topN(m map[string]int, n int) map[string]int {
	out := map[string]int{}
	type kv struct {
		k string
		v int
	}
	list := make([]kv, 0, len(m))
	for k, v := range m {
		list = append(list, kv{k, v})
	}
	for i := 0; i < len(list); i++ {
		for j := i + 1; j < len(list); j++ {
			if list[j].v > list[i].v {
				list[i], list[j] = list[j], list[i]
			}
		}
	}
	for i, item := range list {
		if i >= n {
			break
		}
		out[item.k] = item.v
	}
	return out
}

const version = "1.0.0"

// UAParse extracts a coarse browser/os/device label from a user-agent string.
type UAParse struct {
	Browser string `json:"browser"`
	OS      string `json:"os"`
	Device  string `json:"device"`
}

func parseUA(ua string) UAParse {
	browser, os, device := "unknown", "unknown", "desktop"
	switch {
	case strings.Contains(ua, "Edg/"):
		browser = "Edge"
	case strings.Contains(ua, "OPR/") || strings.Contains(ua, "Opera"):
		browser = "Opera"
	case strings.Contains(ua, "Chrome/"):
		browser = "Chrome"
	case strings.Contains(ua, "Safari") && strings.Contains(ua, "Version/"):
		browser = "Safari"
	case strings.Contains(ua, "Firefox/"):
		browser = "Firefox"
	case strings.Contains(ua, "curl/"):
		browser = "curl"
	case strings.Contains(ua, "Wget/"):
		browser = "wget"
	case strings.Contains(ua, "bot") || strings.Contains(ua, "Bot") || strings.Contains(ua, "spider"):
		browser = "bot"
	}
	switch {
	case strings.Contains(ua, "Windows"):
		os = "Windows"
	case strings.Contains(ua, "Android"):
		os = "Android"
	case strings.Contains(ua, "iPhone") || strings.Contains(ua, "iPad"):
		os = "iOS"
	case strings.Contains(ua, "Mac OS X") || strings.Contains(ua, "Macintosh"):
		os = "macOS"
	case strings.Contains(ua, "Linux"):
		os = "Linux"
	}
	if strings.Contains(ua, "Mobile") || strings.Contains(ua, "iPhone") || strings.Contains(ua, "Android") {
		device = "mobile"
	}
	if strings.Contains(ua, "iPad") || strings.Contains(ua, "Tablet") {
		device = "tablet"
	}
	if browser == "curl" || browser == "wget" || browser == "bot" {
		device = "other"
	}
	return UAParse{Browser: browser, OS: os, Device: device}
}

// RequestInfo captures everything visible about an incoming request.
type RequestInfo struct {
	Method    string            `json:"method"`
	Path      string            `json:"path"`
	Query     string            `json:"query"`
	Proto     string            `json:"proto"`
	Host      string            `json:"host"`
	RemoteIP  string            `json:"remoteIp"`
	TLS       bool              `json:"tls"`
	Headers   map[string]string `json:"headers"`
	BodyBytes int               `json:"bodyBytes"`
	Body      string            `json:"body"`
	UserAgent string            `json:"userAgent"`
	Cookies   map[string]string `json:"cookies"`
	UA        UAParse           `json:"ua"`
	Timestamp string            `json:"timestamp"`
}

func describe(r *http.Request, includeBody bool, maxBody int) RequestInfo {
	headers := make(map[string]string)
	for name, values := range r.Header {
		headers[strings.ToLower(name)] = strings.Join(values, ", ")
	}

	cookies := make(map[string]string)
	for _, c := range r.Cookies() {
		cookies[c.Name] = c.Value
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	if fwd := r.Header.Get("x-forwarded-for"); fwd != "" {
		ip = strings.TrimSpace(strings.Split(fwd, ",")[0])
	}

	body := ""
	bodyBytes := 0
	if includeBody && r.Body != nil {
		buf := make([]byte, maxBody)
		n, _ := r.Body.Read(buf)
		if n > 0 {
			bodyBytes = n
			if isProbablyBinary(buf[:n]) {
				body = fmt.Sprintf("<binary %d bytes>", n)
			} else {
				body = string(buf[:n])
			}
		}
	}

	tls := r.TLS != nil

	return RequestInfo{
		Method:    r.Method,
		Path:      r.URL.Path,
		Query:     r.URL.RawQuery,
		Proto:     r.Proto,
		Host:      r.Host,
		RemoteIP:  ip,
		TLS:       tls,
		Headers:   headers,
		BodyBytes: bodyBytes,
		Body:      body,
		UserAgent: r.UserAgent(),
		Cookies:   cookies,
		UA:        parseUA(r.UserAgent()),
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
	}
}

func isProbablyBinary(b []byte) bool {
	for _, c := range b {
		if c == 0 {
			return true
		}
	}
	return false
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func handler(maxBody int, stats *Stats) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}
		if fwd := r.Header.Get("x-forwarded-for"); fwd != "" {
			ip = strings.TrimSpace(strings.Split(fwd, ",")[0])
		}

		switch r.URL.Path {
		case "/health":
			stats.record(ip, r.Method)
			writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
			return
		case "/":
			stats.record(ip, r.Method)
			writeJSON(w, http.StatusOK, map[string]any{
				"service": "request-inspector",
				"version": version,
				"author":  "v01dst",
				"usage":   "any method, any path → JSON dump of your request",
			})
			return
		case "/stats":
			stats.record(ip, r.Method)
			writeJSON(w, http.StatusOK, stats.snapshot())
			return
		}

		if r.ContentLength > int64(maxBody) {
			stats.record(ip, r.Method)
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{
				"error": fmt.Sprintf("body too large (max %d bytes)", maxBody),
			})
			return
		}

		stats.record(ip, r.Method)
		info := describe(r, true, maxBody)
		w.Header().Set("x-inspected-by", "request-inspector/"+version)
		writeJSON(w, http.StatusOK, info)
	}
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	http.Handle("/", handler(1<<20, NewStats()))
	fmt.Printf("request-inspector listening on :%s\n", port)
	panic(http.ListenAndServe(":"+port, nil))
}
