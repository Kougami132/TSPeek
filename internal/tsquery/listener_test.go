package tsquery_test

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"tspeek/internal/activity"
	"tspeek/internal/config"
	"tspeek/internal/store"
	"tspeek/internal/tsquery"
)

type recordTracker struct {
	events []activity.Event
	ch     chan activity.Event
}

func newRecordTracker() *recordTracker {
	return &recordTracker{
		ch: make(chan activity.Event, 50),
	}
}

func (rt *recordTracker) RecordEvents(events []activity.Event) error {
	for _, e := range events {
		rt.events = append(rt.events, e)
		rt.ch <- e
	}
	return nil
}

func (rt *recordTracker) WaitEvent(t *testing.T, timeout time.Duration) activity.Event {
	t.Helper()
	select {
	case ev := <-rt.ch:
		return ev
	case <-time.After(timeout):
		t.Fatalf("timed out waiting for event")
		return activity.Event{}
	}
}

func (rt *recordTracker) AssertNoEvents(t *testing.T, duration time.Duration) {
	t.Helper()
	select {
	case ev := <-rt.ch:
		t.Fatalf("expected no events, got: %+v", ev)
	case <-time.After(duration):
	}
}

func TestEventListener_BaselineSilenceAndJoinLeave(t *testing.T) {
	// Baseline clientlist: Alice (regular) and QueryBot (query client)
	baseline := "clid=1 cid=1 client_database_id=1 client_nickname=ServerAdmin client_type=1 client_unique_identifier=query_uid|clid=2 cid=1 client_database_id=2 client_nickname=Alice client_type=0 client_unique_identifier=alice_uid"

	server := newMockServer(t, baseline)
	defer server.Close()

	snapshotStore := store.New()
	snapshotStore.SetReady(store.Snapshot{
		Channels: []store.ChannelInfo{
			{ID: 1, Name: "Lobby"},
			{ID: 2, Name: "Gaming"},
		},
	})

	tracker := newRecordTracker()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	listener := tsquery.NewEventListener(tsquery.EventListenerOptions{
		Config: config.ServerQueryConfig{
			Host:       "127.0.0.1",
			QueryPort:  server.port,
			ServerPort: 9987,
			Username:   "serveradmin",
			Password:   "secret",
		},
		Logger:            logger,
		Resolver:          snapshotStore,
		Recorder:          tracker,
		KeepAliveInterval: 10 * time.Second,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go listener.Run(ctx)
	defer listener.Close()

	// Wait briefly for handshake and baseline to finish
	tracker.AssertNoEvents(t, 200*time.Millisecond)

	// Verify handshake commands were sent
	cmds := server.Commands()
	hasLogin := false
	hasUse := false
	hasNotifyServer := false
	hasClientList := false
	for _, cmd := range cmds {
		if cmd == "login client_login_name=serveradmin client_login_password=secret" {
			hasLogin = true
		}
		if cmd == "use port=9987" {
			hasUse = true
		}
		if cmd == "servernotifyregister event=server" {
			hasNotifyServer = true
		}
		if cmd == "clientlist -uid" {
			hasClientList = true
		}
	}
	if !hasLogin || !hasUse || !hasNotifyServer || !hasClientList {
		t.Fatalf("handshake commands incomplete, got: %v", cmds)
	}

	// 1. Regular user Bob joins channel 2 ("Gaming")
	server.SendNotification("notifycliententerview cfid=0 ctid=2 reasonid=0 clid=3 client_unique_identifier=bob_uid client_nickname=Bob client_type=0")
	joinBob := tracker.WaitEvent(t, 1*time.Second)
	if joinBob.Action != "join" || joinBob.UID != "bob_uid" || joinBob.Nickname != "Bob" || joinBob.ChannelID != 2 || joinBob.ChannelName != "Gaming" {
		t.Fatalf("unexpected join event for Bob: %+v", joinBob)
	}

	// 2. Query client connects: should be filtered out
	server.SendNotification("notifycliententerview cfid=0 ctid=1 reasonid=0 clid=4 client_unique_identifier=query2_uid client_nickname=Bot2 client_type=1")
	tracker.AssertNoEvents(t, 100*time.Millisecond)

	// 3. Bob leaves
	server.SendNotification("notifyclientleftview cfid=2 ctid=0 reasonid=8 reasonmsg=leaving clid=3")
	leaveBob := tracker.WaitEvent(t, 1*time.Second)
	if leaveBob.Action != "leave" || leaveBob.UID != "bob_uid" || leaveBob.Nickname != "Bob" || leaveBob.ChannelID != 2 || leaveBob.ChannelName != "Gaming" {
		t.Fatalf("unexpected leave event for Bob: %+v", leaveBob)
	}

	// 4. Instantaneous join and leave (sub-second)
	server.SendNotification("notifycliententerview cfid=0 ctid=1 reasonid=0 clid=10 client_unique_identifier=flash_uid client_nickname=Flash client_type=0")
	server.SendNotification("notifyclientleftview cfid=1 ctid=0 reasonid=8 clid=10")

	joinFlash := tracker.WaitEvent(t, 1*time.Second)
	if joinFlash.Action != "join" || joinFlash.UID != "flash_uid" || joinFlash.Nickname != "Flash" || joinFlash.ChannelID != 1 || joinFlash.ChannelName != "Lobby" {
		t.Fatalf("unexpected join event for Flash: %+v", joinFlash)
	}

	leaveFlash := tracker.WaitEvent(t, 1*time.Second)
	if leaveFlash.Action != "leave" || leaveFlash.UID != "flash_uid" || leaveFlash.Nickname != "Flash" || leaveFlash.ChannelID != 1 || leaveFlash.ChannelName != "Lobby" {
		t.Fatalf("unexpected leave event for Flash: %+v", leaveFlash)
	}

	// 5. Channel fallback for unknown channel (e.g. newly created channel 99)
	server.SendNotification("notifycliententerview cfid=0 ctid=99 reasonid=0 clid=20 client_unique_identifier=new_uid client_nickname=NewUser client_type=0")
	joinNew := tracker.WaitEvent(t, 1*time.Second)
	if joinNew.Action != "join" || joinNew.ChannelID != 99 || joinNew.ChannelName != "Channel #99" {
		t.Fatalf("unexpected channel fallback: %+v", joinNew)
	}
}

func TestEventListener_ChannelMoveAndRename(t *testing.T) {
	// Baseline clientlist: QueryAdmin (query) and Alice (regular in Channel 1 "Lobby")
	baseline := "clid=1 cid=1 client_database_id=1 client_nickname=ServerAdmin client_type=1 client_unique_identifier=query_uid|clid=2 cid=1 client_database_id=2 client_nickname=Alice client_type=0 client_unique_identifier=alice_uid"

	server := newMockServer(t, baseline)
	defer server.Close()

	snapshotStore := store.New()
	snapshotStore.SetReady(store.Snapshot{
		Channels: []store.ChannelInfo{
			{ID: 1, Name: "Lobby"},
			{ID: 2, Name: "Gaming"},
		},
	})

	tracker := newRecordTracker()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	listener := tsquery.NewEventListener(tsquery.EventListenerOptions{
		Config: config.ServerQueryConfig{
			Host:       "127.0.0.1",
			QueryPort:  server.port,
			ServerPort: 9987,
			Username:   "serveradmin",
			Password:   "secret",
		},
		Logger:            logger,
		Resolver:          snapshotStore,
		Recorder:          tracker,
		KeepAliveInterval: 10 * time.Second,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go listener.Run(ctx)
	defer listener.Close()

	// Wait for baseline to finish
	tracker.AssertNoEvents(t, 200*time.Millisecond)

	// Verify channel event notification registration
	cmds := server.Commands()
	hasNotifyChannel := false
	for _, cmd := range cmds {
		if cmd == "servernotifyregister event=channel id=0" {
			hasNotifyChannel = true
		}
	}
	if !hasNotifyChannel {
		t.Fatalf("expected registration for event=channel id=0, commands were: %v", cmds)
	}

	// 1. Channel move: Alice moves from Lobby (1) to Gaming (2)
	server.SendNotification("notifyclientmoved ctid=2 reasonid=0 clid=2")
	moveEv := tracker.WaitEvent(t, 1*time.Second)
	if moveEv.Action != "move" || moveEv.UID != "alice_uid" || moveEv.Nickname != "Alice" {
		t.Fatalf("unexpected move client info: %+v", moveEv)
	}
	if moveEv.FromChannelID != 1 || moveEv.FromChannelName != "Lobby" {
		t.Errorf("expected from Lobby (1), got %d (%s)", moveEv.FromChannelID, moveEv.FromChannelName)
	}
	if moveEv.ChannelID != 2 || moveEv.ChannelName != "Gaming" {
		t.Errorf("expected to Gaming (2), got %d (%s)", moveEv.ChannelID, moveEv.ChannelName)
	}

	// 2. Nickname rename: Alice updates nickname to "AliceInWonderland"
	server.SendNotification("notifyclientupdated clid=2 client_nickname=AliceInWonderland")
	renameEv := tracker.WaitEvent(t, 1*time.Second)
	if renameEv.Action != "rename" || renameEv.UID != "alice_uid" {
		t.Fatalf("unexpected rename client info: %+v", renameEv)
	}
	if renameEv.Nickname != "Alice" || renameEv.TargetNickname != "AliceInWonderland" {
		t.Errorf("expected Alice -> AliceInWonderland, got %s -> %s", renameEv.Nickname, renameEv.TargetNickname)
	}
	if renameEv.ChannelID != 2 || renameEv.ChannelName != "Gaming" {
		t.Errorf("expected rename on channel Gaming (2), got %d (%s)", renameEv.ChannelID, renameEv.ChannelName)
	}

	// 3. Non-nickname property updates should NOT emit any event
	server.SendNotification("notifyclientupdated clid=2 client_is_talker=1 client_talk_power=10 client_is_channel_commander=1")
	tracker.AssertNoEvents(t, 100*time.Millisecond)

	// 4. Nickname update with unchanged name should NOT emit event
	server.SendNotification("notifyclientupdated clid=2 client_nickname=AliceInWonderland")
	tracker.AssertNoEvents(t, 100*time.Millisecond)

	// 5. Query client move and rename should NOT emit events
	server.SendNotification("notifyclientmoved ctid=2 reasonid=0 clid=1")
	server.SendNotification("notifyclientupdated clid=1 client_nickname=NewAdmin")
	tracker.AssertNoEvents(t, 100*time.Millisecond)

	// 6. Alice moves again, verifying updated cached nickname and previous channel
	server.SendNotification("notifyclientmoved ctid=1 reasonid=0 clid=2")
	move2 := tracker.WaitEvent(t, 1*time.Second)
	if move2.Action != "move" || move2.Nickname != "AliceInWonderland" {
		t.Errorf("expected AliceInWonderland, got: %+v", move2)
	}
	if move2.FromChannelID != 2 || move2.FromChannelName != "Gaming" || move2.ChannelID != 1 || move2.ChannelName != "Lobby" {
		t.Errorf("unexpected move2 channels: %+v", move2)
	}
}

func TestEventListener_KeepAliveHeartbeat(t *testing.T) {
	server := newMockServer(t, "")
	defer server.Close()

	tracker := newRecordTracker()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	listener := tsquery.NewEventListener(tsquery.EventListenerOptions{
		Config: config.ServerQueryConfig{
			Host:       "127.0.0.1",
			QueryPort:  server.port,
			ServerPort: 9987,
			Username:   "serveradmin",
			Password:   "secret",
		},
		Logger:            logger,
		Recorder:          tracker,
		KeepAliveInterval: 40 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go listener.Run(ctx)
	defer listener.Close()

	// Wait 150ms to allow at least 2 keep-alive ticks
	time.Sleep(150 * time.Millisecond)

	cmds := server.Commands()
	whoamiCount := 0
	for _, cmd := range cmds {
		if cmd == "whoami" {
			whoamiCount++
		}
	}
	if whoamiCount < 2 {
		t.Fatalf("expected at least 2 whoami commands, got %d, all commands: %v", whoamiCount, cmds)
	}
}

func TestEventListener_ReconnectAndSilentBaseline(t *testing.T) {
	server := newMockServer(t, "clid=1 cid=1 client_database_id=1 client_nickname=ServerAdmin client_type=1 client_unique_identifier=query_uid|clid=2 cid=1 client_database_id=2 client_nickname=Alice client_type=0 client_unique_identifier=alice_uid")
	defer server.Close()

	snapshotStore := store.New()
	snapshotStore.SetReady(store.Snapshot{
		Channels: []store.ChannelInfo{
			{ID: 1, Name: "Lobby"},
			{ID: 2, Name: "Gaming"},
		},
	})

	tracker := newRecordTracker()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	listener := tsquery.NewEventListener(tsquery.EventListenerOptions{
		Config: config.ServerQueryConfig{
			Host:       "127.0.0.1",
			QueryPort:  server.port,
			ServerPort: 9987,
			Username:   "serveradmin",
			Password:   "secret",
		},
		Logger:            logger,
		Resolver:          snapshotStore,
		Recorder:          tracker,
		KeepAliveInterval: 10 * time.Second,
		InitialBackoff:    20 * time.Millisecond,
		MaxBackoff:        100 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go listener.Run(ctx)
	defer listener.Close()

	// Wait for initial connection & baseline
	tracker.AssertNoEvents(t, 150*time.Millisecond)

	// Simulate server changes during disconnect: Alice left, Charlie joined Gaming
	server.SetBaselineRows("clid=1 cid=1 client_database_id=1 client_nickname=ServerAdmin client_type=1 client_unique_identifier=query_uid|clid=3 cid=2 client_database_id=3 client_nickname=Charlie client_type=0 client_unique_identifier=charlie_uid")

	// Drop the connection to trigger reconnect
	server.DisconnectClient()

	// Wait for reconnect and re-baseline
	time.Sleep(200 * time.Millisecond)

	// Reconnection MUST be completely silent: no false events for Alice leaving or Charlie joining
	tracker.AssertNoEvents(t, 100*time.Millisecond)

	// Now Charlie moves to Lobby (1) -> should generate move event with correct origin and target
	server.SendNotification("notifyclientmoved ctid=1 reasonid=0 clid=3")
	moveCharlie := tracker.WaitEvent(t, 1*time.Second)
	if moveCharlie.Action != "move" || moveCharlie.UID != "charlie_uid" || moveCharlie.Nickname != "Charlie" {
		t.Fatalf("unexpected move event: %+v", moveCharlie)
	}
	if moveCharlie.FromChannelID != 2 || moveCharlie.FromChannelName != "Gaming" || moveCharlie.ChannelID != 1 || moveCharlie.ChannelName != "Lobby" {
		t.Errorf("unexpected move channels: %+v", moveCharlie)
	}
}

func TestEventListener_EndToEndActivityLogStorageAndPipeline(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "activity.log")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	server := newMockServer(t, "")
	defer server.Close()

	snapshotStore := store.New()
	snapshotStore.SetReady(store.Snapshot{
		Channels: []store.ChannelInfo{{ID: 1, Name: "Lobby"}},
	})

	activityService := activity.NewService(logPath, logger)
	eventsCh, cancelSub := activityService.Subscribe()
	defer cancelSub()

	listener := tsquery.NewEventListener(tsquery.EventListenerOptions{
		Config: config.ServerQueryConfig{
			Host:       "127.0.0.1",
			QueryPort:  server.port,
			ServerPort: 9987,
			Username:   "serveradmin",
			Password:   "secret",
		},
		Logger:   logger,
		Resolver: snapshotStore,
		Recorder: activityService,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go listener.Run(ctx)
	defer listener.Close()

	// Wait for connection
	time.Sleep(100 * time.Millisecond)

	// Send enter and leave in rapid succession
	server.SendNotification("notifycliententerview cfid=0 ctid=1 reasonid=0 clid=10 client_unique_identifier=rapid_uid client_nickname=RapidUser client_type=0")
	server.SendNotification("notifyclientleftview cfid=1 ctid=0 reasonid=8 clid=10")

	// Read from SSE subscriber channel
	var received []activity.Event
	timeout := time.After(2 * time.Second)
	for len(received) < 2 {
		select {
		case evs := <-eventsCh:
			received = append(received, evs...)
		case <-timeout:
			t.Fatalf("timed out waiting for 2 events on SSE channel, got %d", len(received))
		}
	}

	if received[0].Action != "join" || received[0].UID != "rapid_uid" {
		t.Errorf("expected join event first, got: %+v", received[0])
	}
	if received[1].Action != "leave" || received[1].UID != "rapid_uid" {
		t.Errorf("expected leave event second, got: %+v", received[1])
	}

	// Verify reverse-seek pagination via activityService
	page, err := activityService.GetActivities(1, 10)
	if err != nil {
		t.Fatalf("failed to get activities: %v", err)
	}
	if page.Total != 2 {
		t.Fatalf("expected 2 total activities, got %d", page.Total)
	}
	// Reverse chronological: leave first, then join
	if page.Items[0].Action != "leave" || page.Items[1].Action != "join" {
		t.Errorf("expected items in reverse chronological order, got: %s then %s", page.Items[0].Action, page.Items[1].Action)
	}
}
