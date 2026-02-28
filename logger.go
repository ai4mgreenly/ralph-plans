package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type requestLogger struct {
	f          *os.File
	mu         sync.Mutex
	corsOrigin string
}

type logEntry struct {
	Time       string `json:"time"`
	Method     string `json:"method"`
	Path       string `json:"path"`
	Status     int    `json:"status"`
	GoalID     int64  `json:"goal_id,omitempty"`
	DurationMs int64  `json:"duration_ms"`
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (rl *requestLogger) wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", rl.corsOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(204)
			return
		}

		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(sw, r)

		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			return
		}

		entry := logEntry{
			Time:       start.UTC().Format(time.RFC3339),
			Method:     r.Method,
			Path:       r.URL.Path,
			Status:     sw.status,
			DurationMs: time.Since(start).Milliseconds(),
		}

		// Extract goal_id from path: /goals/{id}/...
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) >= 2 && parts[0] == "goals" {
			if id, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
				entry.GoalID = id
			}
		}

		rl.mu.Lock()
		defer rl.mu.Unlock()
		data, err := json.Marshal(entry)
		if err == nil {
			fmt.Fprintf(rl.f, "%s\n", data)
		}
	})
}
