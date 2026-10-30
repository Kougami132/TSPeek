package tsquery_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"tspeek/internal/config"
	"tspeek/internal/store"
	"tspeek/internal/tsquery"
)

func TestSingleConnectionMultiplexing_FetchAndEvents(t *testing.T) {
	baseline := "clid=1 cid=1 client_database_id=1 client_nickname=ServerAdmin client_type=1 client_unique_identifier=query_uid|clid=2 cid=1 client_database_id=2 client_nickname=Alice client_type=0 client_unique_identifier=alice_uid"

	server := newMockServer(t, baseline)
	defer server.Close()

	snapshotStore := store.New()
	snapshotStore.SetReady(store.Snapshot{
		Channels: []store.ChannelInfo{
			{ID: 1, Name: "Lobby"},
		},
	})

	tracker := newRecordTracker()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	cfg := config.ServerQueryConfig{
		Host:       "127.0.0.1",
		QueryPort:  server.port,
		ServerPort: 9987,
		Username:   "serveradmin",
		Password:   "secret",
		Nickname:   "TSPeekBot",
	}

	client := tsquery.NewClientWithOptions(tsquery.ClientOptions{
		Config:            cfg,
		Logger:            logger,
		KeepAliveInterval: 10 * time.Second,
	})
	defer client.Close()

	listener := tsquery.NewEventListener(tsquery.EventListenerOptions{
		Client:   client,
		Config:   cfg,
		Logger:   logger,
		Resolver: snapshotStore,
		Recorder: tracker,
	})
	defer listener.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go client.Run(ctx)
	go listener.Run(ctx)

	// Wait for baseline silence
	tracker.AssertNoEvents(t, 200*time.Millisecond)

	// 1. Verify that ONLY ONE TCP connection was established
	if count := server.ConnectionCount(); count != 1 {
		t.Fatalf("expected exactly 1 TCP connection, got %d", count)
	}

	// 2. Fetch snapshot over the shared connection
	fetchCtx, fetchCancel := context.WithTimeout(ctx, 3*time.Second)
	defer fetchCancel()

	snap, err := client.Fetch(fetchCtx)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if snap.Server.Name != "Test Server" {
		t.Errorf("expected server name 'Test Server', got %q", snap.Server.Name)
	}

	// 3. Send async notification over the same connection
	server.SendNotification("notifycliententerview cfid=0 ctid=1 reasonid=0 clid=3 client_unique_identifier=bob_uid client_nickname=Bob client_type=0")
	joinBob := tracker.WaitEvent(t, 1*time.Second)
	if joinBob.Action != "join" || joinBob.Nickname != "Bob" {
		t.Fatalf("expected join event for Bob, got: %+v", joinBob)
	}

	// 4. Execute an arbitrary query (simulating IconService ftinitdownload)
	lines, err := client.Exec(fetchCtx, "ftinitdownload clientftfid=1 name=/icon_1 cid=0 cpw= seekpos=0 proto=0")
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}
	_ = lines

	// 5. Confirm that throughout all operations, exactly ONE connection was used
	if count := server.ConnectionCount(); count != 1 {
		t.Fatalf("expected connection count to remain 1, got %d", count)
	}
}

func TestSingleConnection_CommandAndNotificationInterleaving(t *testing.T) {
	baseline := "clid=1 cid=1 client_database_id=1 client_nickname=ServerAdmin client_type=1 client_unique_identifier=query_uid"

	server := newMockServer(t, baseline)
	defer server.Close()

	snapshotStore := store.New()
	snapshotStore.SetReady(store.Snapshot{
		Channels: []store.ChannelInfo{
			{ID: 1, Name: "Lobby"},
		},
	})

	tracker := newRecordTracker()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	cfg := config.ServerQueryConfig{
		Host:       "127.0.0.1",
		QueryPort:  server.port,
		ServerPort: 9987,
		Username:   "serveradmin",
		Password:   "secret",
	}

	client := tsquery.NewClientWithOptions(tsquery.ClientOptions{
		Config:            cfg,
		Logger:            logger,
		KeepAliveInterval: 10 * time.Second,
	})
	defer client.Close()

	listener := tsquery.NewEventListener(tsquery.EventListenerOptions{
		Client:   client,
		Config:   cfg,
		Logger:   logger,
		Resolver: snapshotStore,
		Recorder: tracker,
	})
	defer listener.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go client.Run(ctx)
	go listener.Run(ctx)

	tracker.AssertNoEvents(t, 200*time.Millisecond)

	// Execute command that receives a notify interleaved before its data and status
	cmdCtx, cmdCancel := context.WithTimeout(ctx, 2*time.Second)
	defer cmdCancel()

	lines, err := client.Exec(cmdCtx, "interleaved_cmd")
	if err != nil {
		t.Fatalf("Exec interleaved_cmd failed: %v", err)
	}

	// Verify command returned data line without the notify line
	if len(lines) != 1 || lines[0] != "data_key=data_value" {
		t.Fatalf("expected ['data_key=data_value'], got: %v", lines)
	}

	// Verify the interleaved notification was routed to the event listener
	ev := tracker.WaitEvent(t, 1*time.Second)
	if ev.Action != "join" || ev.UID != "interleaved_uid" || ev.Nickname != "InterleavedUser" {
		t.Fatalf("unexpected interleaved event: %+v", ev)
	}
}

