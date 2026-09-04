package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

const version = "1.0.0"

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

func handler(maxBody int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
			return
		case "/":
			writeJSON(w, http.StatusOK, map[string]any{
				"service": "request-inspector",
				"version": version,
				"author":  "v01dst",
				"usage":   "any method, any path → JSON dump of your request",
			})
			return
		}

		if r.ContentLength > int64(maxBody) {
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{
				"error": fmt.Sprintf("body too large (max %d bytes)", maxBody),
			})
			return
		}

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
	http.Handle("/", handler(1<<20))
	fmt.Printf("request-inspector listening on :%s\n", port)
	panic(http.ListenAndServe(":"+port, nil))
}
