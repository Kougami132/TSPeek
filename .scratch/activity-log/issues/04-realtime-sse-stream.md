# 04: Realtime SSE Activity Streaming and Live Updates

**What to build:** Real-time push of newly captured activity events to the web dashboard via Server-Sent Events. When users are browsing page 1 of the activity log, newly generated events appear immediately at the top of the list without requiring a manual page refresh, matching the live feel of the realtime channel tree.

**Blocked by:** 01: Tracer Bullet Join and Leave Activity Tracking, 03: Reverse Seek Pagination and Zero-Lock File Deletion

**Status:** resolved

- [x] Extend the Server-Sent Events stream `/api/v1/stream` to publish newly captured activity events under `event: activity`.
- [x] Connect the frontend activity state to the SSE stream to receive live events.
- [x] Automatically prepend incoming events to the top of the list when the user is viewing page 1.
- [x] Display an unobtrusive indicator if new events arrive while the user is inspecting an older historical page (page > 1).
- [x] Provide tests verifying SSE activity event broadcast and payload formatting.

## Answer

1. Extended `/api/v1/stream` in `internal/api/sse.go` to multiplex activity updates and broadcast `event: activity` with JSON arrays of events.
2. Connected `useSnapshot` and `useActivities` in `web/src/hooks/useActivities.ts` to consume live SSE `activity` events.
3. Automatically prepended incoming events to the top of page 1 without page refresh, deduplicating by ID.
4. Displayed an unobtrusive banner notifying users when new events arrive while browsing older pages (page > 1) with a one-click return to page 1.
5. Added automated tests `TestActivityPipeline_SSEStreamActivityEvents` verifying SSE broadcast and payload formatting.
