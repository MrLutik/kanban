# Issue #002: Last Sync timestamp never updated after sync [FIXED]

## Summary
After running `kanban sync`, the database status shows "Last Sync: Never" even though sync completed successfully.

## Steps to Reproduce
```bash
./kanban sync --issues-only
./kanban db status
```

## Expected Behavior
The "Last Sync" field should show the timestamp of the last successful sync operation.

## Actual Behavior
```
╠════════════════════════════════════════════════════════════╣
║  Last Sync:      Never                                    ║
╚════════════════════════════════════════════════════════════╝
```

## Root Cause
Looking at the code:
1. `sync.go` calls `database.UpdateRepoSyncTime(dbRepo.ID)` which should update `last_sync_at`
2. `db.go` GetStats queries `SELECT MAX(last_sync_at) FROM repositories`

Possible causes:
- The `UpdateRepoSyncTime` function might not be committing
- The `last_sync_at` column might have a different format issue
- The goroutine updating sync time may have a race condition

## Affected Files
- `cmd/kanban/cmd/sync.go`
- `internal/db/repo.go` (UpdateRepoSyncTime function)
- `internal/db/db.go` (GetStats function)

## Severity
Medium - Informational display issue, sync still works

## Resolution
Root cause: `sql.NullTime` was failing to parse SQLite's TEXT datetime format silently.

Fixed by scanning as `sql.NullString` and manually parsing the datetime using `time.Parse()` with the expected SQLite format `"2006-01-02 15:04:05"`.

Files changed:
- `internal/db/db.go` - GetStats function now properly parses datetime strings

## Labels
- type: bug
- priority: medium
- status: resolved
