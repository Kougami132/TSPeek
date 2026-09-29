package activity_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"tspeek/internal/activity"
)

func TestStorage_NonExistentFile(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "missing.log")
	storage := activity.NewStorage(logPath)

	res, err := storage.ReadPage(1, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Total != 0 || len(res.Items) != 0 || res.TotalPages != 0 {
		t.Fatalf("expected empty page result, got: %+v", res)
	}
}

func TestStorage_EmptyFile(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "empty.log")
	if err := os.WriteFile(logPath, []byte(""), 0644); err != nil {
		t.Fatalf("failed to write empty file: %v", err)
	}

	storage := activity.NewStorage(logPath)
	res, err := storage.ReadPage(1, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Total != 0 || len(res.Items) != 0 {
		t.Fatalf("expected empty page result, got: %+v", res)
	}
}

func TestStorage_AppendAndReverseRead(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")
	storage := activity.NewStorage(logPath)

	now := time.Now().UTC()
	var events []activity.Event
	for i := 1; i <= 25; i++ {
		events = append(events, activity.Event{
			ID:       int64(i),
			Time:     now.Add(time.Duration(i) * time.Second),
			Action:   "join",
			UID:      fmt.Sprintf("uid_%d", i),
			Nickname: fmt.Sprintf("User_%d", i),
		})
	}

	if err := storage.Append(events); err != nil {
		t.Fatalf("failed to append: %v", err)
	}

	// Read Page 1 (size 10): items 25 down to 16
	p1, err := storage.ReadPage(1, 10)
	if err != nil {
		t.Fatalf("failed to read page 1: %v", err)
	}
	if p1.Total != 25 || p1.TotalPages != 3 || len(p1.Items) != 10 {
		t.Fatalf("page 1 mismatch: total=%d, totalPages=%d, len=%d", p1.Total, p1.TotalPages, len(p1.Items))
	}
	if p1.Items[0].ID != 25 || p1.Items[9].ID != 16 {
		t.Errorf("expected IDs 25 to 16, got %d to %d", p1.Items[0].ID, p1.Items[9].ID)
	}

	// Read Page 3 (size 10): items 5 down to 1
	p3, err := storage.ReadPage(3, 10)
	if err != nil {
		t.Fatalf("failed to read page 3: %v", err)
	}
	if p3.Total != 25 || p3.TotalPages != 3 || len(p3.Items) != 5 {
		t.Fatalf("page 3 mismatch: total=%d, totalPages=%d, len=%d", p3.Total, p3.TotalPages, len(p3.Items))
	}
	if p3.Items[0].ID != 5 || p3.Items[4].ID != 1 {
		t.Errorf("expected IDs 5 to 1, got %d to %d", p3.Items[0].ID, p3.Items[4].ID)
	}
}
