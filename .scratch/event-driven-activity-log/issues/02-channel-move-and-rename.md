# 02: Channel Move and Nickname Rename Event Tracking

**What to build:** Extend the ServerQuery event listener to subscribe to channel-level notifications (`servernotifyregister event=channel id=0`), process channel movement notifications (`notifyclientmoved`), and capture client profile updates (`notifyclientupdated`). When a user switches channels, an Activity Event with action `move` is generated with the origin and destination channel IDs and frozen channel names. When a user updates their nickname, an Activity Event with action `rename` is recorded containing the prior and new nicknames.

**Blocked by:** 01: Event Listener Tracer Bullet for Join and Leave

**Status:** closed

- [x] Event listener registers `servernotifyregister event=channel id=0`
- [x] Incoming `notifyclientmoved` notifications emit a `move` Activity Event containing the user UID, nickname, origin channel, and target channel
- [x] Internal client session registry updates the client's current channel upon channel move
- [x] Incoming `notifyclientupdated` notifications with nickname changes emit a `rename` Activity Event containing the old nickname and target nickname
- [x] Internal client session registry updates the cached nickname upon rename
- [x] Non-nickname property updates (e.g. talk power, avatar flags) do not generate spurious Activity Events
- [x] End-to-end test validates move and rename event generation through the mock ServerQuery TCP protocol seam
