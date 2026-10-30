package icon_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"testing"

	"tspeek/internal/icon"
)

type mockExecutor struct {
	cmds     []string
	response []string
	err      error
}

func (m *mockExecutor) Exec(ctx context.Context, cmd string) ([]string, error) {
	m.cmds = append(m.cmds, cmd)
	return m.response, m.err
}

func TestGetIcon_Builtin(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	exec := &mockExecutor{}
	svc := icon.NewService(exec, "127.0.0.1", logger)

	item, err := svc.GetIcon(context.Background(), 100)
	if err != nil {
		t.Fatalf("unexpected error for builtin icon: %v", err)
	}
	if item == nil || len(item.Body) == 0 {
		t.Fatalf("expected non-empty builtin icon body")
	}
	if len(exec.cmds) != 0 {
		t.Fatalf("builtin icon should not trigger serverquery command, got %v", exec.cmds)
	}
}

func TestGetIcon_CustomViaSharedConnection(t *testing.T) {
	// Start a mock FT TCP server
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer ln.Close()

	ftPort := ln.Addr().(*net.TCPAddr).Port
	expectedData := []byte("fake-icon-binary-data")

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		keyBuf := make([]byte, 8)
		_, _ = io.ReadFull(conn, keyBuf)
		_, _ = conn.Write(expectedData)
	}()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	exec := &mockExecutor{
		response: []string{
			fmt.Sprintf("clientftfid=1 name=/icon_12345678 size=%d ftkey=testkey1 port=%d", len(expectedData), ftPort),
		},
	}
	svc := icon.NewService(exec, "127.0.0.1", logger)

	item, err := svc.GetIcon(context.Background(), 12345678)
	if err != nil {
		t.Fatalf("failed to get custom icon: %v", err)
	}
	if string(item.Body) != string(expectedData) {
		t.Fatalf("expected body %q, got %q", string(expectedData), string(item.Body))
	}
	if len(exec.cmds) != 1 {
		t.Fatalf("expected 1 exec command, got %d", len(exec.cmds))
	}

	// Fetch again, should hit memory cache without re-executing
	item2, err := svc.GetIcon(context.Background(), 12345678)
	if err != nil {
		t.Fatalf("failed to get cached custom icon: %v", err)
	}
	if string(item2.Body) != string(expectedData) {
		t.Fatalf("expected cached body %q, got %q", string(expectedData), string(item2.Body))
	}
	if len(exec.cmds) != 1 {
		t.Fatalf("expected still 1 exec command due to cache, got %d", len(exec.cmds))
	}
}
