Status: ready-for-agent

# Spec: Activity Log (客户端活动记录)

## Problem Statement

When monitoring a TeamSpeak server via TSPeek, users and administrators can only view the current instantaneous Snapshot of connected clients and channels. They have no visibility into historical client activities—such as when a player connected to the server, disconnected, moved between channels, or renamed themselves. Furthermore, if a server administrator wants to clear activity records, they prefer a simple zero-configuration approach (such as directly deleting the log file on disk) rather than dealing with complex retention policies, cleanup schedulers, or database migrations.

## Solution

TSPeek will automatically record client lifecycle and movement events to an append-only JSON Lines file on disk with zero configuration required. The file handle is opened on demand so that administrators can manually delete the file at any time to clear history without server locking issues or restarts. If deleted, the file is automatically recreated on the next event.

In the Web UI, an "活动记录" (Activity Log) view is added alongside the existing "实时频道" view via a top navigation tab. Users can browse the chronological timeline of events with traditional numbered pagination. When viewing the first page (latest events), newly captured Activity Events are streamed in real time via Server-Sent Events (SSE) and prepended to the list. Each Activity Event records the Client Identity (UID), nickname at event time, action, target channel name, and any rename transitions to prevent identity spoofing or confusion caused by duplicate nicknames.

## User Stories

1. As a TeamSpeak administrator, I want TSPeek to record client server join events, so that I know who joined the server and at what time.
2. As a TeamSpeak administrator, I want TSPeek to record client server leave events, so that I know when a player disconnected and which channel they were in when they left.
3. As a TeamSpeak administrator, I want TSPeek to record client channel switch events, so that I can see the movement of players between different rooms.
4. As a TeamSpeak administrator, I want TSPeek to record client nickname change events, so that I can track player alias changes and prevent impersonation or confusion.
5. As a TeamSpeak administrator, I want each recorded event to be bound to the client's permanent Unique Identifier (UID), so that duplicate nicknames or renamed players can be definitively tracked.
6. As a TeamSpeak administrator, I want channel names to be frozen at the moment an event occurs, so that historical records remain accurate even if channels are later renamed or deleted.
7. As a TeamSpeak administrator, I want TSPeek to silently baseline existing online clients upon server startup, so that restarting the monitor does not generate a burst of false "join" events for all currently connected users.
8. As a TeamSpeak administrator, I want ServerQuery and bot clients to be ignored by the activity tracker, so that automated health checks or query tools do not pollute the player activity timeline.
9. As a server operator, I want activity records to be stored in an append-only file by default, so that no external database setup or migration is required.
10. As a server operator, I want to be able to configure the activity log file path via `config.yaml`, so that I can mount the file to a host volume in Docker deployments.
11. As a server operator, I want to be able to delete the activity log file from the host filesystem at any time to clear all logs, so that log maintenance requires zero operational complexity.
12. As a server operator, I want TSPeek to automatically recreate the activity log file and resume appending on the next event after manual file deletion, so that I do not need to restart the application.
13. As a web user, I want a top-level tab switcher to toggle between "实时频道" (Realtime Channels) and "活动记录" (Activity Log), so that I can review history in a spacious layout without cluttering the channel tree.
14. As a web user, I want the activity log to be presented as a structured timeline table showing the timestamp, player nickname, UID, action badge, and associated channel, so that I can easily parse the sequence of events.
15. As a web user, I want to navigate historical records using numbered pagination (previous, next, page jump, total records), so that the browser does not become sluggish even when the log contains thousands of entries.
16. As a web user, I want the web interface to display the newest events on page 1 by default, so that I immediately see the most recent activity upon opening the tab.
17. As a web user, I want new activity events to appear in real time on page 1 without refreshing the browser, so that the live monitoring experience remains seamless.
18. As a web user, I want real-time updates not to disrupt my view when I am browsing older historical pages (page > 1), so that my pagination context is not interrupted while reading past events.
19. As a web user, I want clear visual badges for different event types (e.g. green for join, red/gray for leave, blue for channel move, amber for rename), so that I can distinguish actions at a glance.
20. As a web user, I want to view the full UID or copy it easily, so that I can cross-reference suspicious accounts directly in TeamSpeak.

## Implementation Decisions

### Domain Model & State Tracking
- The system introduces the `Activity Event` domain concept with four concrete action variants: `join`, `leave`, `move`, and `rename`.
- An Activity Tracker module continuously compares consecutive valid `Snapshot` instances generated by the poller.
- On cold startup, the first valid snapshot is treated as a silent baseline: the tracker registers currently connected clients into its internal state cache without emitting any Activity Events.
- Subsequent snapshots are diffed against the known client cache using `unique_id` as the primary key.
- Clients with `client_type != 0` (ServerQuery connections) are filtered out prior to diff calculation.
- Resolved channel names are preserved from the snapshot at event generation time so that subsequent channel renames do not mutate historical entries.

### Event Schema
The serialized JSON Lines entry and API representation adheres to the following shape:
```json
{
  "id": 1711785600000001,
  "time": "2026-03-30T15:04:05Z",
  "action": "join",
  "uid": "e123...=",
  "nickname": "Alice",
  "target_nickname": "",
  "channel_id": 10,
  "channel_name": "General",
  "from_channel_id": 0,
  "from_channel_name": ""
}
```
- `action` is one of `"join"`, `"leave"`, `"move"`, `"rename"`.
- For `rename`, `nickname` holds the previous nickname and `target_nickname` holds the new nickname.
- For `move`, `from_channel_*` and `channel_*` represent the origin and destination channels.
- For `join` and `leave`, `channel_*` represents the channel joined or left.

### Storage & File Lifecycle
- Conforming to ADR-0001, events are written as newline-delimited JSON (JSON Lines).
- To allow external file deletion without lock contention on Windows or stale unlinked inode writing on Linux, file writes utilize an on-demand open (`O_APPEND | O_CREATE | O_WRONLY`), write batch, and immediate close pattern.
- If the log file does not exist, it is created with standard permissions (`0644`).
- File pagination reads lines backwards from the end of the file (reverse seek). Reading stops once the requested page of items is collected, maintaining $O(1)$ memory consumption and sub-millisecond response times regardless of total file size.

### HTTP & SSE API
- A new REST endpoint is introduced:
  `GET /api/v1/activities?page=1&page_size=50`
  Response format:
  ```json
  {
    "items": [...],
    "page": 1,
    "page_size": 50,
    "total": 128,
    "total_pages": 3
  }
  ```
- The SSE endpoint `/api/v1/stream` will emit an `activity` event whenever new events are captured:
  `event: activity`
  `data: [{"id":..., "action":..., ...}]`

### Web User Interface
- A Fluent UI tab navigation is added to the main dashboard container, toggling between the Channel Tree view and the Activity Log view.
- The Activity Log view renders a responsive table/list displaying Timestamp, Player (Nickname + UID tooltip/badge), Action Badge, and Details (Channel or Rename info).
- Pagination controls are situated at the bottom of the table, offering page navigation and total count display.
- The frontend hook handles SSE `activity` events: if the user is on page 1, new events are smoothly prepended; if on a later page, a subtle notification indicator informs the user that newer entries have arrived.

## Testing Decisions

### Seam Architecture
The primary automated test seam is the **HTTP API & Activity Pipeline Seam**:
- Using an in-memory HTTP test client (`httptest.Server`), tests feed mock TeamSpeak snapshots into the system and assert against the resulting HTTP API endpoints (`/api/v1/activities`, `/api/v1/stream`) and the generated JSON Lines file on disk.
- External behavior tested at this seam:
  1. Baseline silence: Providing snapshot 1 produces zero events and an empty log.
  2. Join & leave detection: Providing snapshot 2 (with client additions and removals) produces corresponding `join` and `leave` events in the log and API.
  3. Channel move & rename detection: Providing snapshot 3 (with channel transitions and nickname edits) produces `move` and `rename` events.
  4. Query client exclusion: Query clients connecting or disconnecting do not trigger any activity events.
  5. Reverse pagination: Ingestion of N records correctly returns reverse-chronological pages, validating offset and boundary conditions.
  6. External file deletion resilience: Deleting the log file from disk mid-test results in an empty response for subsequent queries, and the next emitted event recreates the file without error.

## Out of Scope

- Automated log rotation or max-file-size truncation (the user explicitly specified that manual deletion of the log file is the intended maintenance mechanism).
- Disabling/enabling toggle for activity tracking (tracking is always enabled by default).
- Filtering, keyword search, or date-range query filters in the UI (standard pagination is sufficient).
- Tracking of fine-grained client mute, deafen, or talk status changes in the activity log.

## Further Notes

- The default log file path is `activity.log` in the working directory, configurable via `activity_log` in `config.yaml`.
- The frontend build process will bundle the updated UI into the embedded static assets served by Go.
