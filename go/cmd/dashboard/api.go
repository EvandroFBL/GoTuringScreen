package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	"turing-screen/render"
)

// MessageHandler exposes the message queue over HTTP.
type MessageHandler struct {
	q *render.MessageQueue
}

func (h *MessageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// CORS headers for local access
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/")

	switch {
	case r.Method == "GET" && path == "messages":
		h.list(w, r)
	case r.Method == "POST" && path == "message":
		h.add(w, r)
	case r.Method == "DELETE" && strings.HasPrefix(path, "message/"):
		id := strings.TrimPrefix(path, "message/")
		h.remove(w, r, id)
	case r.Method == "GET" && path == "health":
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

func (h *MessageHandler) list(w http.ResponseWriter, r *http.Request) {
	msgs := h.q.List()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"messages": msgs})
}

func (h *MessageHandler) add(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Text  string `json:"text"`
		Color string `json:"color"`
		TTL   int    `json:"ttl"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	if body.Text == "" {
		http.Error(w, "text is required", http.StatusBadRequest)
		return
	}
	msg := h.q.Add(body.Text, body.Color, body.TTL)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"message": msg})
}

func (h *MessageHandler) remove(w http.ResponseWriter, r *http.Request, id string) {
	if h.q.Remove(id) {
		w.WriteHeader(http.StatusNoContent)
	} else {
		http.Error(w, "not found", http.StatusNotFound)
	}
}

// StartAPI starts the HTTP API server on the given port.
func StartAPI(q *render.MessageQueue, port int) *http.Server {
	mux := http.NewServeMux()
	h := &MessageHandler{q: q}
	mux.Handle("/", h)

	addr := fmt.Sprintf(":%d", port)
	srv := &http.Server{Addr: addr, Handler: mux}

	go func() {
		log.Printf("[API] listening on http://localhost:%d", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[API] error: %v", err)
		}
	}()

	return srv
}

// Expose message queue via a package-level var so the CLI flag can push messages.
var (
	msgQueue   *render.MessageQueue
	msgQueueMu sync.RWMutex
)

// SetMessageQueue sets the global message queue reference.
func SetMessageQueue(q *render.MessageQueue) {
	msgQueueMu.Lock()
	defer msgQueueMu.Unlock()
	msgQueue = q
}
