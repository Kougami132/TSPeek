# 01: Event Listener Tracer Bullet for Join and Leave

**What to build:** A dedicated ServerQuery event listener connection that registers for server-level push notifications (`servernotifyregister event=server`). Upon connection, it establishes an in-memory client registry from an initial client list without emitting any false join events (cold baseline silence). When receiving client enter or leave notifications, it filters out ServerQuery bots, resolves the channel name from the shared snapshot store (with fallback to `Channel #<id>`), and persists precise `join` and `leave` Activity Events to the append-only log file and streams them over SSE. Even when a user connects and disconnects in milliseconds, both events are accurately captured and recorded in chronological order.

**Blocked by:** None (can start immediately)

**Status:** closed

- [x] ServerQuery event listener connects, logs in, selects virtual server, and registers `event=server`
- [x] Startup baseline silently loads connected clients into the internal session registry without emitting Activity Events
- [x] Incoming `notifycliententerview` notifications for regular users (`client_type == 0`) create an accurate `join` Activity Event with UID, nickname, and channel name
- [x] Incoming `notifyclientleftview` notifications looked up by connection ID (`clid`) emit a corresponding `leave` Activity Event
- [x] Query clients (`client_type != 0`) are filtered out and do not generate Activity Events
- [x] Instantaneous connect-and-disconnect sequences (sub-second) generate both `join` and `leave` events faithfully
- [x] End-to-end test validates the mock ServerQuery TCP notification stream to the Activity Log file and HTTP API/SSE
