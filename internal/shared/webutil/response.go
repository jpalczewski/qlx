package webutil

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

func IsJSON(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "application/json")
}

// WantsJSON returns true if the client prefers JSON responses.
var WantsJSON = IsJSON

func IsHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// SaveOrFail persists store and returns 500 on failure.
func SaveOrFail(w http.ResponseWriter, save func() error) bool {
	if err := save(); err != nil {
		LogError("save failed: %v", err)
		http.Error(w, "persist error", http.StatusInternalServerError)
		return false
	}
	return true
}

// Logging

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[90m"
)

func LogRequest(method, path string, status int, duration time.Duration) {
	statusColor := colorGreen
	if status >= 400 && status < 500 {
		statusColor = colorYellow
	} else if status >= 500 {
		statusColor = colorRed
	}

	methodColor := colorCyan
	switch method {
	case "POST", "PUT", "PATCH":
		methodColor = colorYellow
	case "DELETE":
		methodColor = colorRed
	}

	fmt.Fprintf(os.Stderr, "%s%-7s%s %s %s%d%s %s%s%s\n",
		methodColor, method, colorReset,
		path,
		statusColor, status, colorReset,
		colorGray, duration.Round(time.Microsecond), colorReset,
	)
}

var (
	TraceEnabled bool
	traceFile    *os.File
)

// SetTraceFile sets the file for trace output. Call with nil to disable file tracing.
func SetTraceFile(f *os.File) {
	traceFile = f
}

func LogError(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "%s[ERROR]%s %s\n", colorRed, colorReset, fmt.Sprintf(format, args...))
}

func LogInfo(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "%s[INFO]%s  %s\n", colorCyan, colorReset, fmt.Sprintf(format, args...))
}

func LogTrace(format string, args ...any) {
	if !TraceEnabled {
		return
	}
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s[TRACE]%s %s\n", colorGray, colorReset, msg)
	if traceFile != nil {
		_, _ = fmt.Fprintf(traceFile, "%s [TRACE] %s\n", time.Now().Format("15:04:05.000"), msg)
	}
}

// HexDump formats bytes as hex string, max maxBytes shown.
func HexDump(data []byte, maxBytes int) string {
	if len(data) <= maxBytes {
		return fmt.Sprintf("%x", data)
	}
	return fmt.Sprintf("%x... (%d bytes)", data[:maxBytes], len(data))
}

// LoggingMiddleware wraps an http.Handler with colored request logging.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// Flush forwards to the underlying ResponseWriter if it supports http.Flusher (needed for SSE).
func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(sw, r)
		duration := time.Since(start)
		LogRequest(r.Method, r.URL.Path, sw.status, duration)
		if sw.status >= 500 {
			LogError("%s %s → %d (%s)", r.Method, r.URL.Path, sw.status, duration.Round(time.Microsecond))
		}
	})
}
