package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"tspeek/internal/activity"
	"tspeek/internal/store"
)

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	updates, cancel := s.store.Subscribe()
	defer cancel()

	var actUpdates <-chan []activity.Event
	if s.activities != nil {
		ch, actCancel := s.activities.Subscribe()
		defer actCancel()
		actUpdates = ch
	}

	if current, ok := s.store.Current(); ok {
		if err := writeSSESnapshot(w, flusher, current); err != nil {
			return
		}
	} else {
		if _, err := io.WriteString(w, ": waiting for first snapshot\n\n"); err != nil {
			return
		}
		flusher.Flush()
	}

	keepAlive := time.NewTicker(15 * time.Second)
	defer keepAlive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case next, ok := <-updates:
			if !ok {
				return
			}
			if err := writeSSESnapshot(w, flusher, next); err != nil {
				return
			}
		case actEvents, ok := <-actUpdates:
			if !ok {
				return
			}
			if err := writeSSEActivities(w, flusher, actEvents); err != nil {
				return
			}
		case <-keepAlive.C:
			if _, err := io.WriteString(w, ": keepalive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func writeSSESnapshot(w http.ResponseWriter, flusher http.Flusher, latest store.Snapshot) error {
	payload, err := json.Marshal(latest)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "id: %d\nevent: snapshot\ndata: %s\n\n", latest.Meta.Sequence, payload); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

func writeSSEActivities(w http.ResponseWriter, flusher http.Flusher, events []activity.Event) error {
	payload, err := json.Marshal(events)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "event: activity\ndata: %s\n\n", payload); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}
