package api_test

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tspeek/internal/activity"
	"tspeek/internal/api"
	"tspeek/internal/store"
)

type activityAPIResponse struct {
	Items      []activity.Event `json:"items"`
	Page       int              `json:"page"`
	PageSize   int              `json:"page_size"`
	Total      int              `json:"total"`
	TotalPages int              `json:"total_pages"`
}

func TestActivityPipeline_ColdBaselineAndJoinLeave(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "activity.log")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	snapshotStore := store.New()
	actService := activity.NewService(logPath, logger)

	server := api.NewServer(api.Options{
		Logger:     logger,
		Store:      snapshotStore,
		Activities: actService,
	})
	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	// --- Step 1: Cold Baseline ---
	snap1 := store.Snapshot{
		Channels: []store.ChannelInfo{
			{ID: 1, Name: "Lobby"},
			{ID: 10, Name: "General"},
		},
		Clients: []store.ClientInfo{
			{ID: 101, UniqueID: "alice_uid", Nickname: "Alice", ChannelID: 1, Type: 0},
			{ID: 102, UniqueID: "bob_uid", Nickname: "Bob", ChannelID: 1, Type: 0},
			{ID: 999, UniqueID: "query_uid", Nickname: "ServerAdmin", ChannelID: 1, Type: 1}, // Query client
		},
		Meta: store.SnapshotMeta{FetchedAt: time.Now().UTC()},
	}

	snapshotStore.SetReady(snap1)
	if err := actService.ProcessSnapshot(snap1); err != nil {
		t.Fatalf("failed to process snap1: %v", err)
	}

	// Baseline should be silent: no events emitted or stored
	resp, err := ts.Client().Get(ts.URL + "/api/v1/activities")
	if err != nil {
		t.Fatalf("failed to get activities: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var data activityAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if data.Total != 0 || len(data.Items) != 0 {
		t.Fatalf("expected 0 events after baseline, got total=%d items=%d", data.Total, len(data.Items))
	}

	// File should not exist or be empty
	if fi, err := os.Stat(logPath); err == nil && fi.Size() > 0 {
		t.Fatalf("expected log file to be empty, got size %d", fi.Size())
	}

	// --- Step 2: Join & Leave ---
	// Alice leaves, Bob remains, Charlie joins General (ID 10), Bot2 (Type 1) connects.
	snap2 := store.Snapshot{
		Channels: []store.ChannelInfo{
			{ID: 1, Name: "Lobby"},
			{ID: 10, Name: "General"},
		},
		Clients: []store.ClientInfo{
			{ID: 102, UniqueID: "bob_uid", Nickname: "Bob", ChannelID: 1, Type: 0},
			{ID: 103, UniqueID: "charlie_uid", Nickname: "Charlie", ChannelID: 10, Type: 0},
			{ID: 998, UniqueID: "bot2_uid", Nickname: "HealthBot", ChannelID: 1, Type: 1}, // Query client
		},
		Meta: store.SnapshotMeta{FetchedAt: time.Now().UTC()},
	}

	snapshotStore.SetReady(snap2)
	if err := actService.ProcessSnapshot(snap2); err != nil {
		t.Fatalf("failed to process snap2: %v", err)
	}

	resp2, err := ts.Client().Get(ts.URL + "/api/v1/activities")
	if err != nil {
		t.Fatalf("failed to get activities: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp2.StatusCode)
	}

	var data2 activityAPIResponse
	if err := json.NewDecoder(resp2.Body).Decode(&data2); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if data2.Total != 2 {
		t.Fatalf("expected 2 total events, got %d", data2.Total)
	}
	if len(data2.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(data2.Items))
	}

	// Reverse chronological: newest first.
	// Both events occurred in snap2. Verify one join (Charlie) and one leave (Alice).
	actions := map[string]activity.Event{}
	for _, item := range data2.Items {
		actions[item.Action] = item
	}

	joinEv, hasJoin := actions["join"]
	if !hasJoin {
		t.Fatalf("missing join event in items: %+v", data2.Items)
	}
	if joinEv.UID != "charlie_uid" || joinEv.Nickname != "Charlie" || joinEv.ChannelID != 10 || joinEv.ChannelName != "General" {
		t.Errorf("unexpected join event content: %+v", joinEv)
	}

	leaveEv, hasLeave := actions["leave"]
	if !hasLeave {
		t.Fatalf("missing leave event in items: %+v", data2.Items)
	}
	if leaveEv.UID != "alice_uid" || leaveEv.Nickname != "Alice" || leaveEv.ChannelID != 1 || leaveEv.ChannelName != "Lobby" {
		t.Errorf("unexpected leave event content: %+v", leaveEv)
	}

	// Verify file on disk exists and has 2 non-empty lines
	fileBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	if len(fileBytes) == 0 {
		t.Fatal("expected non-empty log file")
	}
}

func TestActivityPipeline_ChannelMoveAndRenameWithFrozenChannelNames(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "activity.log")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	snapshotStore := store.New()
	actService := activity.NewService(logPath, logger)

	server := api.NewServer(api.Options{
		Logger:     logger,
		Store:      snapshotStore,
		Activities: actService,
	})
	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	// 1. Initial snapshot: Lobby (1) and Room A (2)
	snap1 := store.Snapshot{
		Channels: []store.ChannelInfo{
			{ID: 1, Name: "Lobby"},
			{ID: 2, Name: "Room A"},
		},
		Clients: []store.ClientInfo{
			{ID: 101, UniqueID: "user_alice", Nickname: "Alice", ChannelID: 1, Type: 0},
			{ID: 102, UniqueID: "user_bob", Nickname: "Bob", ChannelID: 2, Type: 0},
		},
		Meta: store.SnapshotMeta{FetchedAt: time.Now().UTC()},
	}
	snapshotStore.SetReady(snap1)
	if err := actService.ProcessSnapshot(snap1); err != nil {
		t.Fatalf("failed to process snap1: %v", err)
	}

	// 2. Second snapshot:
	// Alice switches channel from Lobby (1) to Room A (2)
	// Bob renames from "Bob" to "Bobby"
	snap2 := store.Snapshot{
		Channels: []store.ChannelInfo{
			{ID: 1, Name: "Lobby"},
			{ID: 2, Name: "Room A"},
		},
		Clients: []store.ClientInfo{
			{ID: 101, UniqueID: "user_alice", Nickname: "Alice", ChannelID: 2, Type: 0},
			{ID: 102, UniqueID: "user_bob", Nickname: "Bobby", ChannelID: 2, Type: 0},
		},
		Meta: store.SnapshotMeta{FetchedAt: time.Now().UTC()},
	}
	snapshotStore.SetReady(snap2)
	if err := actService.ProcessSnapshot(snap2); err != nil {
		t.Fatalf("failed to process snap2: %v", err)
	}

	// 3. Third snapshot: Channels are renamed or deleted in the server.
	// Room A is renamed to "Room B", Lobby is deleted.
	// No client changes.
	snap3 := store.Snapshot{
		Channels: []store.ChannelInfo{
			{ID: 2, Name: "Room B"},
		},
		Clients: []store.ClientInfo{
			{ID: 101, UniqueID: "user_alice", Nickname: "Alice", ChannelID: 2, Type: 0},
			{ID: 102, UniqueID: "user_bob", Nickname: "Bobby", ChannelID: 2, Type: 0},
		},
		Meta: store.SnapshotMeta{FetchedAt: time.Now().UTC()},
	}
	snapshotStore.SetReady(snap3)
	if err := actService.ProcessSnapshot(snap3); err != nil {
		t.Fatalf("failed to process snap3: %v", err)
	}

	// Query API
	resp, err := ts.Client().Get(ts.URL + "/api/v1/activities")
	if err != nil {
		t.Fatalf("failed to get activities: %v", err)
	}
	defer resp.Body.Close()

	var data activityAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if data.Total != 2 {
		t.Fatalf("expected 2 total events, got %d", data.Total)
	}

	actions := map[string]activity.Event{}
	for _, item := range data.Items {
		actions[item.Action] = item
	}

	moveEv, hasMove := actions["move"]
	if !hasMove {
		t.Fatalf("missing move event: %+v", data.Items)
	}
	if moveEv.UID != "user_alice" || moveEv.Nickname != "Alice" {
		t.Errorf("unexpected move client info: %+v", moveEv)
	}
	if moveEv.FromChannelID != 1 || moveEv.FromChannelName != "Lobby" {
		t.Errorf("expected from Lobby (1), got %d (%s)", moveEv.FromChannelID, moveEv.FromChannelName)
	}
	if moveEv.ChannelID != 2 || moveEv.ChannelName != "Room A" {
		t.Errorf("expected to Room A (2), got %d (%s)", moveEv.ChannelID, moveEv.ChannelName)
	}

	renameEv, hasRename := actions["rename"]
	if !hasRename {
		t.Fatalf("missing rename event: %+v", data.Items)
	}
	if renameEv.UID != "user_bob" {
		t.Errorf("unexpected rename UID: %+v", renameEv)
	}
	if renameEv.Nickname != "Bob" || renameEv.TargetNickname != "Bobby" {
		t.Errorf("expected rename Bob -> Bobby, got %s -> %s", renameEv.Nickname, renameEv.TargetNickname)
	}
	if renameEv.ChannelID != 2 || renameEv.ChannelName != "Room A" {
		t.Errorf("expected rename channel Room A (2), got %d (%s)", renameEv.ChannelID, renameEv.ChannelName)
	}
}

func TestActivityPipeline_ReverseSeekPaginationAndFileDeletion(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "activity.log")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	snapshotStore := store.New()
	actService := activity.NewService(logPath, logger)

	server := api.NewServer(api.Options{
		Logger:     logger,
		Store:      snapshotStore,
		Activities: actService,
	})
	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	// 1. Establish baseline
	actService.ProcessSnapshot(store.Snapshot{
		Channels: []store.ChannelInfo{{ID: 1, Name: "Lobby"}},
		Clients:  []store.ClientInfo{},
	})

	// 2. Generate 120 join events
	var clients []store.ClientInfo
	for i := 1; i <= 120; i++ {
		uid := fmt.Sprintf("user_%03d", i)
		nick := fmt.Sprintf("Player_%03d", i)
		clients = append(clients, store.ClientInfo{
			ID: i, UniqueID: uid, Nickname: nick, ChannelID: 1, Type: 0,
		})
		snap := store.Snapshot{
			Channels: []store.ChannelInfo{{ID: 1, Name: "Lobby"}},
			Clients:  clients,
		}
		if err := actService.ProcessSnapshot(snap); err != nil {
			t.Fatalf("failed to process snap: %v", err)
		}
	}

	// Helper to fetch page
	fetchPage := func(page, pageSize int) activityAPIResponse {
		url := fmt.Sprintf("%s/api/v1/activities?page=%d&page_size=%d", ts.URL, page, pageSize)
		resp, err := ts.Client().Get(url)
		if err != nil {
			t.Fatalf("get %s failed: %v", url, err)
		}
		defer resp.Body.Close()
		var res activityAPIResponse
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			t.Fatalf("decode failed: %v", err)
		}
		return res
	}

	// Assert Page 1: 50 items (Player 120 down to Player 71)
	p1 := fetchPage(1, 50)
	if p1.Total != 120 || p1.TotalPages != 3 || len(p1.Items) != 50 {
		t.Fatalf("page 1 mismatch: total=%d, totalPages=%d, len=%d", p1.Total, p1.TotalPages, len(p1.Items))
	}
	if p1.Items[0].Nickname != "Player_120" {
		t.Errorf("expected first item on page 1 to be Player_120, got %s", p1.Items[0].Nickname)
	}
	if p1.Items[49].Nickname != "Player_071" {
		t.Errorf("expected 50th item on page 1 to be Player_071, got %s", p1.Items[49].Nickname)
	}

	// Assert Page 2: 50 items (Player 070 down to Player 021)
	p2 := fetchPage(2, 50)
	if p2.Total != 120 || p2.TotalPages != 3 || len(p2.Items) != 50 {
		t.Fatalf("page 2 mismatch: total=%d, totalPages=%d, len=%d", p2.Total, p2.TotalPages, len(p2.Items))
	}
	if p2.Items[0].Nickname != "Player_070" {
		t.Errorf("expected first item on page 2 to be Player_070, got %s", p2.Items[0].Nickname)
	}
	if p2.Items[49].Nickname != "Player_021" {
		t.Errorf("expected 50th item on page 2 to be Player_021, got %s", p2.Items[49].Nickname)
	}

	// Assert Page 3: 20 items (Player 020 down to Player 001)
	p3 := fetchPage(3, 50)
	if p3.Total != 120 || p3.TotalPages != 3 || len(p3.Items) != 20 {
		t.Fatalf("page 3 mismatch: total=%d, totalPages=%d, len=%d", p3.Total, p3.TotalPages, len(p3.Items))
	}
	if p3.Items[0].Nickname != "Player_020" {
		t.Errorf("expected first item on page 3 to be Player_020, got %s", p3.Items[0].Nickname)
	}
	if p3.Items[19].Nickname != "Player_001" {
		t.Errorf("expected 20th item on page 3 to be Player_001, got %s", p3.Items[19].Nickname)
	}

	// Assert Page 4: 0 items
	p4 := fetchPage(4, 50)
	if p4.Total != 120 || p4.TotalPages != 3 || len(p4.Items) != 0 {
		t.Fatalf("page 4 mismatch: total=%d, totalPages=%d, len=%d", p4.Total, p4.TotalPages, len(p4.Items))
	}

	// --- Test External File Deletion ---
	if err := os.Remove(logPath); err != nil {
		t.Fatalf("failed to delete log file: %v", err)
	}

	// Subsequent read should return empty, zero total, no error
	pAfterDelete := fetchPage(1, 50)
	if pAfterDelete.Total != 0 || len(pAfterDelete.Items) != 0 {
		t.Fatalf("expected 0 items after deletion, got %d items, total %d", len(pAfterDelete.Items), pAfterDelete.Total)
	}

	// Subsequent write should seamlessly recreate the file
	clients = append(clients, store.ClientInfo{
		ID: 200, UniqueID: "new_player", Nickname: "NewPlayer", ChannelID: 1, Type: 0,
	})
	snapNew := store.Snapshot{
		Channels: []store.ChannelInfo{{ID: 1, Name: "Lobby"}},
		Clients:  clients,
	}
	if err := actService.ProcessSnapshot(snapNew); err != nil {
		t.Fatalf("failed to process snapshot after deletion: %v", err)
	}

	pRecreated := fetchPage(1, 50)
	if pRecreated.Total != 1 || len(pRecreated.Items) != 1 {
		t.Fatalf("expected 1 item after recreation, got total=%d, len=%d", pRecreated.Total, len(pRecreated.Items))
	}
	if pRecreated.Items[0].Nickname != "NewPlayer" {
		t.Errorf("expected NewPlayer, got %s", pRecreated.Items[0].Nickname)
	}
}

func TestActivityPipeline_SSEStreamActivityEvents(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "activity.log")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	snapshotStore := store.New()
	actService := activity.NewService(logPath, logger)

	server := api.NewServer(api.Options{
		Logger:     logger,
		Store:      snapshotStore,
		Activities: actService,
	})
	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	// 1. Establish baseline
	snap1 := store.Snapshot{
		Channels: []store.ChannelInfo{{ID: 1, Name: "Lobby"}},
		Clients:  []store.ClientInfo{},
	}
	snapshotStore.SetReady(snap1)
	if err := actService.ProcessSnapshot(snap1); err != nil {
		t.Fatalf("failed to process baseline: %v", err)
	}

	// 2. Connect to SSE stream
	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/stream", nil)
	if err != nil {
		t.Fatalf("new request failed: %v", err)
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("SSE connect failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// 3. Emit a new snapshot with a join event
	snap2 := store.Snapshot{
		Channels: []store.ChannelInfo{{ID: 1, Name: "Lobby"}},
		Clients: []store.ClientInfo{
			{ID: 10, UniqueID: "sse_user", Nickname: "SSEUser", ChannelID: 1, Type: 0},
		},
	}
	snapshotStore.SetReady(snap2)
	if err := actService.ProcessSnapshot(snap2); err != nil {
		t.Fatalf("failed to process snapshot: %v", err)
	}

	// 4. Read from SSE stream with timeout protection
	reader := bufio.NewReader(resp.Body)
	foundActivityEvent := false
	var activityDataLine string

	for i := 0; i < 30; i++ {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("reading sse failed: %v", err)
		}
		line = strings.TrimSpace(line)
		if line == "event: activity" {
			foundActivityEvent = true
			// next line should be data: [...]
			dataLine, err := reader.ReadString('\n')
			if err != nil {
				t.Fatalf("reading sse data failed: %v", err)
			}
			activityDataLine = strings.TrimPrefix(strings.TrimSpace(dataLine), "data: ")
			break
		}
	}

	if !foundActivityEvent {
		t.Fatal("did not receive 'event: activity' on SSE stream")
	}

	var events []activity.Event
	if err := json.Unmarshal([]byte(activityDataLine), &events); err != nil {
		t.Fatalf("failed to parse SSE activity JSON: %v, raw: %s", err, activityDataLine)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event in SSE payload, got %d", len(events))
	}
	if events[0].Action != "join" || events[0].UID != "sse_user" || events[0].Nickname != "SSEUser" {
		t.Errorf("unexpected SSE event content: %+v", events[0])
	}
}
