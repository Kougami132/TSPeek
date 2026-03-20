package api

import (
	"encoding/json"
	"net/http"
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
		RefreshInterval:        s.refreshInterval.String(),
		RefreshIntervalSeconds: int(s.refreshInterval / 1e9), // time.Duration 纳秒 → 秒
		ShowQueryClients:       s.showQueryClients,
		ServerHost:             s.serverHost,
		ServerPort:             s.serverPort,
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
