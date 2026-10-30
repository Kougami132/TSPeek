package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"tspeek/internal/config"
)

func TestConfigActivityLogDefault(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	content := `
port: 8080
serverquery:
  host: "127.0.0.1"
  query_port: 10011
  username: "user"
  password: "pw"
  server_port: 9987
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ActivityLog != "activity.log" {
		t.Fatalf("expected default activity_log to be %q, got %q", "activity.log", cfg.ActivityLog)
	}
}

func TestConfigActivityLogCustom(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	content := `
port: 8080
activity_log: "custom/path.log"
serverquery:
  host: "127.0.0.1"
  query_port: 10011
  username: "user"
  password: "pw"
  server_port: 9987
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ActivityLog != "custom/path.log" {
		t.Fatalf("expected activity_log to be %q, got %q", "custom/path.log", cfg.ActivityLog)
	}
}

func TestConfigNickname(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	content := `
port: 8080
serverquery:
  host: "127.0.0.1"
  query_port: 10011
  username: "user"
  password: "pw"
  server_port: 9987
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ServerQuery.Nickname != "TSPeek" {
		t.Fatalf("expected default nickname to be %q, got %q", "TSPeek", cfg.ServerQuery.Nickname)
	}

	customContent := `
port: 8080
serverquery:
  host: "127.0.0.1"
  query_port: 10011
  username: "user"
  password: "pw"
  server_port: 9987
  nickname: "MyBot"
`
	if err := os.WriteFile(configPath, []byte(customContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err = config.Load(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ServerQuery.Nickname != "MyBot" {
		t.Fatalf("expected custom nickname to be %q, got %q", "MyBot", cfg.ServerQuery.Nickname)
	}
}
