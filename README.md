# Kanban CLI

A CLI tool for applying Kanban methodology to GitHub organizations. Manages labels, workflows, and issue tracking across multiple repositories with a single configuration.

## Features

- **Label Management** - Standardize labels across all repositories in an organization
- **Sync Engine** - Apply label configurations with dry-run safety
- **Local Database** - SQLite cache for offline board/metrics (instant queries)
- **Kanban Board** - Terminal-friendly board view with status columns
- **Comprehensive Metrics** - 15 kanban formulas with bottleneck detection
- **Audit** - Check label consistency and compliance across repos
- **Migration** - Migrate existing issues from old labels to new kanban labels
- **Backup/Restore** - Portable database with export/import support
- **Config-Driven** - YAML configuration with presets (minimal, standard, full)

## Installation

### From Source (Docker)

```bash
git clone https://github.com/kiracore/kanban.git
cd kanban
make build
```

### Binary

Download from [Releases](https://github.com/kiracore/kanban/releases) (coming soon)

## Quick Start

```bash
# Initialize config for your organization
kanban init --org myorg --preset standard

# Or for personal repositories (use your GitHub username)
kanban init --org johnwick --preset standard

# Initialize the database
kanban db init

# Audit current label state
kanban audit --org myorg --all

# Preview changes (dry-run)
kanban sync --org myorg --all --dry-run

# Apply labels and cache issues
kanban sync --org myorg --all

# Migrate existing issues to new labels
kanban migrate --org myorg --all --dry-run

# View kanban board (instant from cache)
kanban board --org myorg --all

# View metrics
kanban metrics --org myorg --all

# Backup before changes
kanban db backup --output ./backup-$(date +%Y%m%d).db
```

## Commands

### `kanban init`

Initialize a `.kanban.yaml` configuration file.

```bash
kanban init --org <organization> [--preset minimal|standard|full]
```

**Presets:**
- `minimal` - Basic status (todo/doing/done) and priority (high/low)
- `standard` - Full status workflow, priorities, types, and special labels
- `full` - Everything including size estimation labels

### `kanban labels`

Manage labels across repositories.

```bash
# List labels in a repo
kanban labels list --org myorg --repo myrepo

# List labels across all repos
kanban labels list --org myorg --all

# Export labels to file
kanban labels export --org myorg --repo myrepo --format yaml > labels.yaml

# Import labels from file
kanban labels import labels.yaml --org myorg --repo myrepo
kanban labels import labels.yaml --org myorg --all
```

### `kanban sync`

Synchronize labels to repositories and cache issues in local database.

```bash
# Sync labels and issues to specific repo
kanban sync --org myorg --repo myrepo

# Sync to all repos (with dry-run)
kanban sync --org myorg --all --dry-run

# Sync to all repos
kanban sync --org myorg --all

# Only sync issues (skip labels)
kanban sync --org myorg --repo myrepo --issues-only

# Only sync labels (skip issues)
kanban sync --org myorg --repo myrepo --labels-only
```

### `kanban audit`

Check label consistency across repositories.

```bash
# Audit specific repo
kanban audit --org myorg --repo myrepo

# Audit all repos
kanban audit --org myorg --all

# JSON output
kanban audit --org myorg --all --format json
```

### `kanban db`

Manage the local SQLite database for caching and offline access.

```bash
# Initialize database
kanban db init

# Show database status
kanban db status

# Show database file path
kanban db path

# Backup database
kanban db backup --output ./backup.db

# Restore from backup
kanban db restore --input ./backup.db

# Export to JSON (for portability)
kanban db export > data.json

# Import from JSON
kanban db import < data.json

# Reset database (destroy all data)
kanban db reset
```

### `kanban board`

Display kanban board in terminal.

```bash
# View board from cache (instant)
kanban board --org myorg --repo myrepo

# View board directly from GitHub (live)
kanban board --org myorg --repo myrepo --live

# Sort by age (oldest first, like gh)
kanban board --org myorg --repo myrepo --sort age

# Sort by updated time (newest first)
kanban board --org myorg --repo myrepo --sort updated

# Sort by assignee
kanban board --org myorg --repo myrepo --sort assignee

# Filter by assignee
kanban board --org myorg --repo myrepo --assignee username

# View board across all repos
kanban board --org myorg --all

# Limit issues per column
kanban board --org myorg --repo myrepo --limit 5
```

**Sort options:** `priority` (default), `updated`, `age`, `assignee`, `created`

### `kanban metrics`

Display comprehensive kanban metrics and analytics.

```bash
# View metrics from cache (instant)
kanban metrics --org myorg --repo myrepo

# View metrics from live GitHub data
kanban metrics --org myorg --repo myrepo --live

# View metrics for 90 days
kanban metrics --org myorg --repo myrepo --days 90

# Show only aging issues (skip other metrics)
kanban metrics --org myorg --repo myrepo --aging

# Sort aging issues by assignee
kanban metrics --org myorg --repo myrepo --aging --sort assignee

# Filter by assignee
kanban metrics --org myorg --repo myrepo --assignee username

# JSON output
kanban metrics --org myorg --repo myrepo --format json
```

**Sort options for aging issues:** `age` (default), `assignee`, `status`

**Metrics included:**
- **Flow Metrics**: Lead Time, Cycle Time, Throughput, Flow Efficiency
- **WIP Metrics**: Work In Progress, WIP Age, Little's Law validation
- **Rate Metrics**: Arrival Rate, Departure Rate, system balance
- **Aging Issues**: Oldest items by status
- **Bottleneck Detection**: Automatic warnings for flow problems

### `kanban migrate`

Migrate issues from old labels to new labels.

```bash
# Migrate single label
kanban migrate --org myorg --repo myrepo --from "bug" --to "type: bug"

# Migrate using config mappings
kanban migrate --org myorg --all --config .kanban.yaml --dry-run

# Migrate across all repos
kanban migrate --org myorg --all
```

## Configuration

### Organizations vs Personal Repos

The `--org` flag works with both GitHub organizations and personal usernames:

```bash
# Organization repos
kanban sync --org kiracore --all

# Personal repos (use your GitHub username)
kanban sync --org johnwick --all

# Specific personal repo
kanban sync --org johnwick --repo my-project
```

### Multi-Org/User Setup

You can track repos from multiple organizations and users in a single config by listing them explicitly:

```yaml
version: "1"

organization: "primary-org"  # Default org for commands

repositories:
  list:
    - "primary-org/repo1"
    - "primary-org/repo2"
    - "johnwick/personal-project"   # Personal repo
    - "other-org/shared-repo"       # Another org
```

### `.kanban.yaml`

```yaml
version: "1"

organization: "myorg"

repositories:
  include: ["*"]
  exclude:
    - "*.github.io"
    - ".github"

labels:
  status:
    - name: "status: backlog"
      color: "d4d4d4"
      description: "Prioritized but not started"
    - name: "status: ready"
      color: "0075ca"
      description: "Ready to be worked on"
    - name: "status: in-progress"
      color: "fbca04"
      description: "Actively being worked on"
    - name: "status: review"
      color: "d93f0b"
      description: "Waiting for code review"
    - name: "status: done"
      color: "0e8a16"
      description: "Completed and merged"

  priority:
    - name: "priority: critical"
      color: "b60205"
      description: "Drop everything"
    - name: "priority: high"
      color: "d93f0b"
      description: "Next up"
    - name: "priority: medium"
      color: "fbca04"
      description: "Normal priority"
    - name: "priority: low"
      color: "0e8a16"
      description: "When time permits"

  type:
    - name: "type: bug"
      color: "d73a4a"
      description: "Something is broken"
    - name: "type: feature"
      color: "a2eeef"
      description: "New functionality"

migrations:
  - from: "bug"
    to: "type: bug"
  - from: "enhancement"
    to: "type: feature"
  - from: "in-progress"
    to: "status: in-progress"

settings:
  preserve_unknown: true
  concurrency: 5
```

## Label Schema (24 labels)

```
Status (6):    backlog → ready → in-progress → review → testing → done
Priority (4):  critical (=blocked) | high | medium | low
Type (8):      bug | feature | improvement | docs | refactor | chore | test | security
Size (5):      XS | S | M | L | XL
Special (1):   needs-triage
```

### Status Labels (Kanban Columns)

| Label | Color | Description |
|-------|-------|-------------|
| `status: backlog` | #d4d4d4 | Prioritized but not started |
| `status: ready` | #0075ca | Ready to be worked on |
| `status: in-progress` | #fbca04 | Actively being worked on |
| `status: review` | #d93f0b | Waiting for code review |
| `status: testing` | #a371f7 | Being tested/validated |
| `status: done` | #0e8a16 | Completed and merged |

### Priority Labels

| Label | Color | Description |
|-------|-------|-------------|
| `priority: critical` | #b60205 | Drop everything - blocks release |
| `priority: high` | #d93f0b | Next up - important for milestone |
| `priority: medium` | #fbca04 | Normal priority |
| `priority: low` | #0e8a16 | When time permits |

### Type Labels

| Label | Color | Description |
|-------|-------|-------------|
| `type: bug` | #d73a4a | Something is broken |
| `type: feature` | #a2eeef | New functionality |
| `type: improvement` | #84b6eb | Enhancement to existing feature |
| `type: docs` | #0075ca | Documentation work |
| `type: refactor` | #5319e7 | Code quality improvement |
| `type: chore` | #fef2c0 | Maintenance tasks |
| `type: test` | #bfd4f2 | Test coverage improvement |
| `type: security` | #ee0701 | Security related issue |

### Size Labels

| Label | Color | Description |
|-------|-------|-------------|
| `size: XS` | #ededed | < 1 hour |
| `size: S` | #d4d4d4 | 1-4 hours |
| `size: M` | #bdbdbd | 1-2 days |
| `size: L` | #9e9e9e | 3-5 days |
| `size: XL` | #757575 | > 1 week |

### Special Labels

| Label | Color | Description |
|-------|-------|-------------|
| `needs-triage` | #fbca04 | Needs initial review |

## Requirements

- GitHub CLI (`gh`) installed and authenticated
- Access to target organization repositories

## Development

```bash
# Build
make build

# Build for all platforms
make build-all

# Run tests
make test

# Development shell
make dev-shell
```

## License

MIT

## Documentation

- [Kanban Framework Guide](docs/KANBAN_FRAMEWORK.md) - Complete methodology documentation
- [Example Configs](configs/examples/) - Configuration examples for different use cases
