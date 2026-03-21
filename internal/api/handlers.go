package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, HealthResponse{Status: "ok"})
}

func (s *Server) handleReady(w http.ResponseWriter, _ *http.Request) {
	if !s.store.Ready() {
		writeJSON(w, http.StatusServiceUnavailable, HealthResponse{
			Status: "not_ready",
			Error:  "not_ready",
		})
		return
	}
	writeJSON(w, http.StatusOK, HealthResponse{Status: "ok"})
}

func (s *Server) handlePublicConfig(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, PublicConfigResponse{
		ServerHost: s.serverHost,
		ServerPort: s.serverPort,
	})
}

func (s *Server) handleSnapshot(w http.ResponseWriter, _ *http.Request) {
	latest, ok := s.store.Current()
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, APIError{Error: "snapshot_not_ready"})
		return
	}
	writeJSON(w, http.StatusOK, latest)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(true)
	_ = encoder.Encode(payload)
}

func (s *Server) handleIcon(w http.ResponseWriter, r *http.Request) {
	raw := strings.TrimPrefix(r.URL.Path, "/api/v1/icons/")
	if raw == "" {
		http.NotFound(w, r)
		return
	}
	iconID, err := strconv.ParseUint(raw, 10, 32)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	item, err := s.icons.GetIcon(r.Context(), uint32(iconID))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", item.ContentType)
	w.Header().Set("Cache-Control", "public, max-age=86400, immutable")
	w.Header().Set("Content-Length", strconv.Itoa(len(item.Body)))
	w.Write(item.Body)
}
