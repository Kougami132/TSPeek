# Append-Only JSON Lines File for Activity Log

We chose to store activity logs in an append-only JSON Lines file using on-demand file handles rather than an embedded database (like SQLite). This satisfies the requirement that server administrators can clear all logs at any time simply by deleting the file from the filesystem on Linux and Windows without file-lock issues or service restarts, while reverse-seeking allows constant-memory pagination for recent events.
