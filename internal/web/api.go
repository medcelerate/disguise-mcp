package web

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/medcelerate/disguise-mcp/internal/disguise"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// GET /api/status — current target + reachability.
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	writeJSON(w, http.StatusOK, s.app.Status(ctx))
}

// GET /api/target returns the current target; POST /api/target repoints it.
func (s *Server) handleTarget(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.app.Target())

	case http.MethodPost:
		var t disguise.Target
		if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		if t.Host == "" {
			writeError(w, http.StatusBadRequest, "host is required")
			return
		}
		if err := s.app.SetTarget(t); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
		defer cancel()
		writeJSON(w, http.StatusOK, s.app.Status(ctx))

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}
