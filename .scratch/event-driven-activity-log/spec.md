Status: ready-for-agent

# Spec: Event-Driven Activity Logging (基于原生事件监听的精确活动记录)

## Problem Statement

When monitoring TeamSpeak servers via TSPeek, server administrators rely on the Activity Log to audit client lifecycle and movement transitions. However, the current activity tracking implementation relies on snapshot polling diffs executed on a 5-second ticker. If a user connects to the server and disconnects in rapid succession (e.g. within a few hundred milliseconds or seconds between polling intervals), the poller never catches them in consecutive snapshot slices, resulting in zero recorded events.

Lowering the polling interval cannot solve this problem at its root, as sub-second polling quickly triggers TeamSpeak's built-in ServerQuery anti-flood protection ban (`error id=524 msg=client is flooding`). Administrators need a fully accurate, zero-loss mechanism to capture all transient and instantaneous client activities—including short-lived connections, rapid channel switches, and renames—without causing flood bans or burdening system resources.

## Solution

TSPeek will adopt an event-driven dual-connection ServerQuery architecture. A dedicated, persistent Event Listener connection will register for real-time TeamSpeak server and channel events (`servernotifyregister event=server` and `servernotifyregister event=channel id=0`). TeamSpeak's push notifications (`Query Notification`) are streamed asynchronously over TCP and immediately transformed into immutable `Activity Event` records in real time.

Instantaneous connections and disconnections are captured with sub-millisecond precision because TeamSpeak's server engine dispatches enter and leave notifications sequentially regardless of session duration. The existing polling connection remains intact but is relieved of activity tracking duties, dedicating itself purely to periodic full `Snapshot` updates (channel tree, server info, permission groups) for the Web dashboard.

## User Stories

1. As a TeamSpeak administrator, I want TSPeek to capture users who join and leave within a fraction of a second, so that rapid connection attempts or automated network scans are never missed.
2. As a TeamSpeak administrator, I want instantaneous join and leave transitions to produce separate, faithful Activity Events in chronological order, so that historical auditing reflects exact server reality.
3. As a TeamSpeak administrator, I want client channel moves to be recorded the moment they happen via push notifications, so that user navigation across rooms is tracked with zero polling latency.
4. As a TeamSpeak administrator, I want client nickname changes to be recorded in real time as rename Activity Events, so that impersonation or evasive renaming can be monitored immediately.
5. As a TeamSpeak administrator, I want ServerQuery accounts and bots (`client_type != 0`) to be ignored by the event listener, so that automated health checks or monitoring connections do not pollute the player activity timeline.
6. As a TeamSpeak administrator, I want each recorded event to be linked to the client's permanent Unique Identifier (UID), so that users who disconnect immediately or change nicknames can always be uniquely identified.
7. As a TeamSpeak administrator, I want channel names in Activity Events to be resolved and frozen at the exact time of the event, so that subsequent channel name changes or deletions do not corrupt historical logs.
8. As a TeamSpeak administrator, I want fallback channel naming (e.g. `Channel #<id>`) to apply cleanly if an event occurs in a newly created channel before the channel snapshot cache updates, so that no event fails or gets dropped.
9. As a TeamSpeak administrator, I want TSPeek startup to silently register currently connected users without emitting false join events, so that restarting the service does not spam the Activity Log with all online players.
10. As a TeamSpeak administrator, I want the event listener connection to send periodic keep-alive pings, so that idle connections are not dropped by TeamSpeak server inactivity timeouts.
11. As a TeamSpeak administrator, I want automatic reconnection with exponential backoff if the event listener TCP stream disconnects, so that the service self-heals after network blips without manual intervention.
12. As a TeamSpeak administrator, I want reconnection to silently baseline current online clients without fabricating events for missed transient sessions during the downtime, so that log integrity is preserved.
13. As a server operator, I want the event-driven tracker to append records to the existing append-only JSON Lines file format, so that existing log file maintenance (such as clearing logs by deleting the file) remains 100% compatible.
14. As a web user, I want new Activity Events generated from push notifications to immediately appear on page 1 of the Activity Log tab via Server-Sent Events (SSE), so that live activity updates feel instantaneous.
15. As a web user, I want browsing historical pages (> 1) to remain undisturbed by incoming live events, so that reading past audit history is not disrupted by new client traffic.
16. As a web user, I want to see consistent action badges (`join`, `leave`, `move`, `rename`) regardless of whether the event was captured during long sessions or instantaneous visits.

## Implementation Decisions

### Architectural Separation: Dual ServerQuery Connections
- The system divides ServerQuery communication into two decoupled responsibilities:
  1. **Poller Connection**: Periodically fetches full `Snapshot` payloads (`serverinfo`, `channellist`, `clientlist`, permission groups) on a 5-second interval solely to populate the in-memory `SnapshotStore` for the Web UI. It no longer feeds or triggers `Activity Event` processing.
  2. **Event Listener Connection**: Maintains a persistent TCP stream to TeamSpeak ServerQuery, authenticates, selects the virtual server, and registers for push notifications via:
     - `servernotifyregister event=server` (listens for client connections, disconnections, and server edits)
     - `servernotifyregister event=channel id=0` (listens for channel-level movements and updates across all channels)

### In-Memory Client Registry & Notification Mapping
- The Event Listener maintains an in-memory client session map keyed by temporary connection ID (`clid`):
  `clid -> { UID: string, Nickname: string, ChannelID: int, ClientType: int }`
- **Initial Baseline & Reconnection**: Upon connecting or reconnecting, the listener executes a single `clientlist -uid` to populate the `clid` map silently. No Activity Events are emitted during this cold initialization.
- **Handling `notifycliententerview`**:
  - Parses `clid`, `client_unique_identifier`, `client_nickname`, `ctid`, and `client_type`.
  - If `client_type != 0`, marks as query client in the internal map and ignores it.
  - If `client_type == 0`, registers the client in the `clid` map and emits a `join` Activity Event with `ChannelID = ctid` and resolved channel name.
- **Handling `notifyclientleftview`**:
  - Parses `clid`.
  - Looks up client in the `clid` map and removes the entry.
  - If the client was `client_type == 0`, emits a `leave` Activity Event with the last known `ChannelID` and channel name.
- **Handling `notifyclientmoved`**:
  - Parses `clid` and target channel `ctid`.
  - Looks up client in the `clid` map. If found and `client_type == 0`, records previous channel ID, updates to `ctid`, and emits a `move` Activity Event with origin and target channel details.
- **Handling `notifyclientupdated`**:
  - Parses `clid` and updated properties.
  - If `client_nickname` is updated and differs from cached nickname, updates the cache and emits a `rename` Activity Event (`nickname` = old, `target_nickname` = new).

### Channel Name Resolution & Resilience
- Channel names are looked up from the current `SnapshotStore`.
- If a channel ID is not found in the snapshot store (e.g. temporary channel created within the last 5 seconds), the system formats the name as `Channel #<id>`, ensuring zero blocked notifications and zero crashes.

### Keep-Alive & Connection Lifecycle
- A background ticker sends a lightweight `whoami` command every 60 seconds over the event listener connection.
- If the TCP connection terminates or errors, the listener triggers an exponential backoff reconnect loop (1s to 60s max backoff), establishes a clean session, re-registers event notifications, and rebuilds the `clid` registry silently.

### Activity Event Ingestion & Streaming
- Dispatched Activity Events are forwarded directly to the existing `activity.Service`, which persists them to the append-only JSON Lines file and broadcasts them over the SSE broker to connected Web clients.
- Existing file storage, reverse-seek pagination, and REST API endpoints (`/api/v1/activities`, `/api/v1/stream`) remain completely unchanged and compatible.

## Testing Decisions

### Seam Architecture: ServerQuery TCP Notification to Activity Pipeline Seam
The primary test seam is the **ServerQuery TCP Protocol to Activity Pipeline Seam**:
- Using an in-memory mock TCP server (`net.Pipe` or `net.Listen("tcp", "127.0.0.1:0")`), tests simulate the TeamSpeak ServerQuery protocol server.
- The test harness simulates raw protocol lines: authentication handshake (`login`, `use`), registration commands (`servernotifyregister`), and incoming asynchronous `notify...` lines interleaved with command responses.
- External behaviors validated at this seam:
  1. **Instantaneous Join & Leave**: A client sends `notifycliententerview` followed 10ms later by `notifyclientleftview`. The test asserts that both events are written to the JSON Lines file and streamed over SSE in proper order with accurate UID and channel attribution.
  2. **Channel Movement**: `notifyclientmoved` generates a valid `move` event with correct origin and target channel names.
  3. **Client Renaming**: `notifyclientupdated` with a new nickname generates a valid `rename` event.
  4. **Query Client Filtering**: `notifycliententerview` with `client_type=1` produces zero stored activity events.
  5. **Cold Startup Baseline Silence**: Initial `clientlist` responses populate the internal registry without emitting any join events.
  6. **Connection Loss & Silent Recovery**: Simulating network termination and reconnection verifies that re-baselining does not emit spurious join/leave events.
  7. **Keep-Alive Dispatch**: Verifies that the listener transmits periodic `whoami` heartbeats on idle streams.

## Out of Scope

- Tracking client mute, deafen, or microphone talk states (out of scope for activity logs).
- Subscribing to text chat messages (`event=textserver`, `event=textchannel`).
- Modifying the REST API schema or frontend UI layout (existing UI and API contracts remain preserved).
- Removing the periodic poller (the poller remains necessary for snapshot tree and icon status).

## Further Notes

- Documented architectural decision recorded in `docs/adr/0002-event-driven-activity-logging.md`.
- Domain terminology aligned with `GLOSSARY.md` (specifically `Query Notification` as wire protocol detail vs `Activity Event` as domain entity).
