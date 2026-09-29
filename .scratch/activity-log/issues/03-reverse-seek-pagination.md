# 03: Reverse Seek Pagination and Zero-Lock File Deletion

**What to build:** High-performance reverse-seeking pagination and resilient file handling for long-term accumulating logs. Web users can navigate through historical records using numbered pagination (page 1, 2, ..., next, previous, total counts) with instantaneous millisecond response and constant $O(1)$ memory usage. System administrators can delete the log file directly on the host at any moment to reset history; subsequent reads return empty and future writes seamlessly recreate the log without service disruption or file lock errors.

**Blocked by:** 01: Tracer Bullet Join and Leave Activity Tracking

**Status:** resolved

- [x] Implement reverse line-seeking reader on the JSON Lines file to read pages from the tail backwards without loading the whole file into memory.
- [x] Provide paginated API endpoint `GET /api/v1/activities?page=1&page_size=50` returning structured pagination metadata (`items`, `page`, `page_size`, `total`, `total_pages`).
- [x] Implement numbered pagination controls in the web interface (previous, next, current page, total count).
- [x] Handle missing or deleted file scenarios gracefully in the reader by returning empty results.
- [x] Support seamless file recreation on subsequent event writes following external file deletion on Linux and Windows.
- [x] Provide tests asserting reverse page boundary order and resilience against mid-run external file deletion.

## Answer

1. Implemented chunk-based reverse line-seeking in `internal/activity/storage.go` (`readLinesReverse`), reading lines backwards from the end of the file without full-file in-memory loading.
2. Formatted API responses with full pagination metadata (`items`, `page`, `page_size`, `total`, `total_pages`) in `GET /api/v1/activities`.
3. Added numbered page navigation controls, page jump buttons, and total count display in `web/src/components/ActivityLog.tsx`.
4. Gracefully handled missing or deleted file states in `Storage.ReadPage` by returning empty results without failing.
5. Supported on-demand open-append-close operations so external file deletions on Linux/Windows result in seamless recreation on the subsequent write.
6. Added automated tests `TestActivityPipeline_ReverseSeekPaginationAndFileDeletion` and unit tests in `internal/activity/storage_test.go`.
