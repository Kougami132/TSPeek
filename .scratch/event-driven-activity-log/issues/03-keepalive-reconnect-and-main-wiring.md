# 03: Keep-Alive, Reconnection, and Main Service Wiring

**What to build:** Add periodic keep-alive pings (`whoami` command every 60 seconds) to prevent TeamSpeak server idle disconnects, and build automatic exponential-backoff reconnection logic for the event listener. Upon reconnecting after a network disruption, the listener re-registers all notifications and re-baselines current online clients silently without fabricating events for missed downtime sessions. Wire the new event listener into `cmd/server/main.go`, and decouple the snapshot poller from activity log processing so that snapshot polling is dedicated strictly to Web UI dashboard state.

**Blocked by:** 02: Channel Move and Nickname Rename Event Tracking

**Status:** closed

- [x] Background heartbeat sends `whoami` every 60 seconds on idle event listener connections
- [x] Network disconnects trigger automatic reconnection with exponential backoff (up to 60s max)
- [x] Reconnection re-registers `event=server` and `event=channel id=0` and silently rebuilds client session registry
- [x] Reconnection does not emit false join/leave events for transient changes that occurred during downtime
- [x] Main entrypoint wires the dual-connection architecture: dedicated event listener for activities, and poller for Web UI snapshots
- [x] Poller snapshot processing no longer generates Activity Events
- [x] Clean shutdown closes both ServerQuery connections gracefully
- [x] All automated tests pass with explicit timeout protections
