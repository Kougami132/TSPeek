# 01: Tracer Bullet Join and Leave Activity Tracking

**What to build:** Establish the end-to-end tracer bullet for client activity tracking. When a regular voice client connects to or disconnects from the TeamSpeak server, the system detects the change across consecutive snapshots, records the event to an append-only JSON Lines file on disk with on-demand file handles, exposes a basic activities API, and allows users to switch to an "活动记录" tab on the web dashboard to see the chronological join/leave event list.

**Blocked by:** None (can start immediately)

**Status:** resolved

- [x] Configure `activity_log` path in configuration with a sensible default (`activity.log`).
- [x] Silently baseline online clients upon the first successful snapshot after startup without generating false join events.
- [x] Detect regular voice clients joining or leaving the server across polling snapshots, filtering out ServerQuery clients (`client_type != 0`).
- [x] Append generated join and leave events to the JSON Lines log file on disk using on-demand open-append-close operations.
- [x] Expose an HTTP endpoint to retrieve recent activity entries.
- [x] Provide a tab switcher on the web dashboard to toggle between "实时频道" and "活动记录", displaying the basic activity feed.
- [x] Include an end-to-end automated test validating cold startup baseline, join/leave capture, and API response.

## Answer

1. Added `activity_log` configuration parameter to `Config` in `internal/config/config.go`, defaulting to `"activity.log"`, with unit tests in `internal/config/config_test.go`.
2. Implemented `Tracker` in `internal/activity/tracker.go` which silently baselines connected voice clients (`client_type == 0`) upon initial snapshot, and detects subsequent `join` and `leave` transitions.
3. Implemented append-only JSON Lines `Storage` in `internal/activity/storage.go` with on-demand file handles and zero-lock operations.
4. Added HTTP endpoint `GET /api/v1/activities` in `internal/api/handlers.go`.
5. Added Fluent UI tab switcher in `web/src/App.tsx` toggling between "实时频道" and "活动记录".
6. Verified with automated pipeline test `TestActivityPipeline_ColdBaselineAndJoinLeave`.
