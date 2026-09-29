# 02: Channel Move and Nickname Change Tracking

**What to build:** Extend activity detection to capture channel movements and nickname alterations with permanent UID tracking and frozen channel names. Users viewing the activity log can see clear badges and details for when a player switches channels or renames themselves, ensuring identity continuity even across frequent alias changes.

**Blocked by:** 01: Tracer Bullet Join and Leave Activity Tracking

**Status:** resolved

- [x] Detect when a connected client switches from one channel to another across consecutive snapshots.
- [x] Detect when a connected client changes their nickname across consecutive snapshots while retaining their permanent UID.
- [x] Freeze the channel name strings in the event record at generation time to preserve historical accuracy against later channel renaming or deletion.
- [x] In the Web UI, display distinct visual badges for all four action types (`join`, `leave`, `move`, `rename`).
- [x] In the Web UI, display origin and destination channels for moves, and former vs current nicknames for renames.
- [x] Provide tests verifying channel move and rename detection with frozen channel names.

## Answer

1. Extended `Tracker` in `internal/activity/tracker.go` to diff connected client channels and nicknames across snapshots, producing `move` and `rename` events keyed by permanent UID.
2. Channel names are frozen into `channel_name` and `from_channel_name` at event creation time from the snapshot channel dictionary, ensuring historical integrity even if channels are later edited or deleted.
3. Implemented distinct badges in `web/src/components/ActivityLog.tsx` for `join` (green), `leave` (red/danger), `move` (blue/informative), and `rename` (warning/amber).
4. Rendered move origin/destination channels and rename former/target nicknames in the details column, along with copyable client UID tooltips.
5. Added automated tests `TestActivityPipeline_ChannelMoveAndRenameWithFrozenChannelNames` and unit tests in `internal/activity/tracker_test.go`.
