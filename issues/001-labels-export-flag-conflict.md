# Issue #001: labels export command panics due to flag shorthand conflict [FIXED]

## Summary
The `kanban labels export` command panics with "unable to redefine 'o' shorthand" error.

## Steps to Reproduce
```bash
./kanban labels export --repo interx
```

## Expected Behavior
Should export labels from the repository to stdout or specified output file.

## Actual Behavior
Panic with stack trace:
```
panic: unable to redefine 'o' shorthand in "export" flagset: it's already used for "output" flag
```

## Root Cause
The `-o` shorthand is used by:
1. Global flag `--org` with shorthand `-o`
2. Local flag `--output` with shorthand `-o` in labels export command

This causes a conflict when cobra merges persistent flags.

## Fix
Remove the `-o` shorthand from the `--output` flag in `cmd/kanban/cmd/labels.go`, or use a different shorthand like `-O` or no shorthand at all.

## Affected File
`cmd/kanban/cmd/labels.go`

## Severity
High - Command completely unusable

## Resolution
Fixed by:
1. Removing `-o` shorthand from `--output` flag in labels.go
2. Renaming shared `format` variable to `labelsFormat` in labels.go to avoid conflicts with other commands
3. Adding shared `format` variable to root.go for audit, cfd, and metrics commands

Files changed:
- `cmd/kanban/cmd/labels.go` - renamed variables to `labelsFormat` and `labelsOutputFile`
- `cmd/kanban/cmd/root.go` - added shared `format` variable

## Labels
- type: bug
- priority: high
- status: resolved
