package activity_test

import (
	"testing"
	"time"

	"tspeek/internal/activity"
	"tspeek/internal/store"
)

func TestTracker_BaselineSilenceAndEventGeneration(t *testing.T) {
	tracker := activity.NewTracker()

	// 1. First snapshot: baseline silence
	snap1 := store.Snapshot{
		Channels: []store.ChannelInfo{{ID: 1, Name: "General"}},
		Clients: []store.ClientInfo{
			{ID: 1, UniqueID: "user1", Nickname: "Alice", ChannelID: 1, Type: 0},
			{ID: 2, UniqueID: "query1", Nickname: "ServerAdmin", ChannelID: 1, Type: 1}, // Query client
		},
		Meta: store.SnapshotMeta{FetchedAt: time.Now().UTC()},
	}

	events1 := tracker.ProcessSnapshot(snap1)
	if len(events1) != 0 {
		t.Fatalf("expected 0 events on baseline, got %d", len(events1))
	}

	// 2. Second snapshot: Alice moves to Channel 2, Bob joins Channel 2, Query client 2 joins
	snap2 := store.Snapshot{
		Channels: []store.ChannelInfo{
			{ID: 1, Name: "General"},
			{ID: 2, Name: "Gaming"},
		},
		Clients: []store.ClientInfo{
			{ID: 1, UniqueID: "user1", Nickname: "Alice", ChannelID: 2, Type: 0},
			{ID: 3, UniqueID: "user2", Nickname: "Bob", ChannelID: 2, Type: 0},
			{ID: 4, UniqueID: "query2", Nickname: "Bot", ChannelID: 1, Type: 1},
		},
		Meta: store.SnapshotMeta{FetchedAt: time.Now().UTC()},
	}

	events2 := tracker.ProcessSnapshot(snap2)
	if len(events2) != 2 {
		t.Fatalf("expected 2 events in snap2, got %d", len(events2))
	}

	actions := map[string]activity.Event{}
	for _, ev := range events2 {
		actions[ev.Action] = ev
	}

	moveEv, hasMove := actions["move"]
	if !hasMove || moveEv.UID != "user1" || moveEv.FromChannelID != 1 || moveEv.ChannelID != 2 {
		t.Errorf("unexpected move event: %+v", moveEv)
	}

	joinEv, hasJoin := actions["join"]
	if !hasJoin || joinEv.UID != "user2" || joinEv.ChannelID != 2 {
		t.Errorf("unexpected join event: %+v", joinEv)
	}

	// 3. Third snapshot: Alice leaves, Bob renames to "Bobby"
	snap3 := store.Snapshot{
		Channels: []store.ChannelInfo{
			{ID: 1, Name: "General"},
			{ID: 2, Name: "Gaming"},
		},
		Clients: []store.ClientInfo{
			{ID: 3, UniqueID: "user2", Nickname: "Bobby", ChannelID: 2, Type: 0},
		},
		Meta: store.SnapshotMeta{FetchedAt: time.Now().UTC()},
	}

	events3 := tracker.ProcessSnapshot(snap3)
	if len(events3) != 2 {
		t.Fatalf("expected 2 events in snap3, got %d", len(events3))
	}

	actions3 := map[string]activity.Event{}
	for _, ev := range events3 {
		actions3[ev.Action] = ev
	}

	leaveEv, hasLeave := actions3["leave"]
	if !hasLeave || leaveEv.UID != "user1" {
		t.Errorf("unexpected leave event: %+v", leaveEv)
	}

	renameEv, hasRename := actions3["rename"]
	if !hasRename || renameEv.UID != "user2" || renameEv.Nickname != "Bob" || renameEv.TargetNickname != "Bobby" {
		t.Errorf("unexpected rename event: %+v", renameEv)
	}
}
