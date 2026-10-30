package api_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"tspeek/internal/api"
	"tspeek/internal/config"
)

type publicConfigPayload struct {
	ServerHost string `json:"server_host"`
	ServerPort int    `json:"server_port"`
	Branding   struct {
		HeaderTitle string `json:"header_title"`
	} `json:"branding"`
	WebClient struct {
		URL string `json:"url"`
	} `json:"web_client"`
}

func getPublicConfig(t *testing.T, webClientURL string) publicConfigPayload {
	t.Helper()

	server := api.NewServer(api.Options{
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		Branding:     config.BrandingConfig{HeaderTitle: "TSPeek"},
		WebClientURL: webClientURL,
		ServerHost:   "ts.example.com",
		ServerPort:   9987,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/public-config", nil)
	server.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var payload publicConfigPayload
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	return payload
}

func TestPublicConfigExposesWebClientURL(t *testing.T) {
	payload := getPublicConfig(t, "https://web.teamspeak.com/?connect=ts.example.com:9987")

	if payload.WebClient.URL != "https://web.teamspeak.com/?connect=ts.example.com:9987" {
		t.Fatalf("expected web_client.url to be echoed, got %q", payload.WebClient.URL)
	}
	if payload.ServerHost != "ts.example.com" {
		t.Fatalf("expected server_host %q, got %q", "ts.example.com", payload.ServerHost)
	}
	if payload.ServerPort != 9987 {
		t.Fatalf("expected server_port %d, got %d", 9987, payload.ServerPort)
	}
	if payload.Branding.HeaderTitle != "TSPeek" {
		t.Fatalf("expected header_title %q, got %q", "TSPeek", payload.Branding.HeaderTitle)
	}
}

func TestPublicConfigWebClientURLDefaultsToEmpty(t *testing.T) {
	payload := getPublicConfig(t, "")

	if payload.WebClient.URL != "" {
		t.Fatalf("expected web_client.url to be empty, got %q", payload.WebClient.URL)
	}
}
