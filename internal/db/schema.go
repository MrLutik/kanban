package db

// Schema version for migrations
// Version 2: Added pull_requests and pr_issue_links tables
const SchemaVersion = 2

// Schema contains the database schema
const Schema = `
-- Schema version
CREATE TABLE IF NOT EXISTS schema_version (
    version INTEGER PRIMARY KEY,
    applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- ═══════════════════════════════════════════════════════════════
-- CORE ENTITIES
-- ═══════════════════════════════════════════════════════════════

CREATE TABLE IF NOT EXISTS organizations (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    name            TEXT NOT NULL UNIQUE,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS repositories (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    org_id          INTEGER NOT NULL REFERENCES organizations(id),
    name            TEXT NOT NULL,
    full_name       TEXT NOT NULL UNIQUE,
    is_active       BOOLEAN DEFAULT TRUE,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_sync_at    DATETIME,
    UNIQUE(org_id, name)
);

CREATE TABLE IF NOT EXISTS labels (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    repo_id         INTEGER NOT NULL REFERENCES repositories(id),
    name            TEXT NOT NULL,
    color           TEXT,
    description     TEXT,
    category        TEXT,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(repo_id, name)
);

-- ═══════════════════════════════════════════════════════════════
-- ISSUES
-- ═══════════════════════════════════════════════════════════════

CREATE TABLE IF NOT EXISTS issues (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    repo_id         INTEGER NOT NULL REFERENCES repositories(id),
    number          INTEGER NOT NULL,
    title           TEXT NOT NULL,
    state           TEXT NOT NULL,

    gh_created_at   DATETIME NOT NULL,
    gh_updated_at   DATETIME NOT NULL,
    gh_closed_at    DATETIME,

    current_status  TEXT,
    current_priority TEXT,
    current_type    TEXT,
    current_size    TEXT,
    is_blocked      BOOLEAN DEFAULT FALSE,

    assignee        TEXT,

    entered_ready_at      DATETIME,
    entered_progress_at   DATETIME,
    entered_review_at     DATETIME,
    entered_testing_at    DATETIME,
    entered_done_at       DATETIME,

    lead_time_hours       REAL,
    cycle_time_hours      REAL,
    blocked_time_hours    REAL,

    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(repo_id, number)
);

CREATE TABLE IF NOT EXISTS issue_labels (
    issue_id        INTEGER NOT NULL REFERENCES issues(id),
    label_id        INTEGER NOT NULL REFERENCES labels(id),
    added_at        DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (issue_id, label_id)
);

-- ═══════════════════════════════════════════════════════════════
-- PULL REQUESTS
-- ═══════════════════════════════════════════════════════════════

CREATE TABLE IF NOT EXISTS pull_requests (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    repo_id         INTEGER NOT NULL REFERENCES repositories(id),
    number          INTEGER NOT NULL,
    title           TEXT NOT NULL,
    state           TEXT NOT NULL,
    is_draft        BOOLEAN DEFAULT FALSE,

    gh_created_at   DATETIME NOT NULL,
    gh_updated_at   DATETIME NOT NULL,
    gh_merged_at    DATETIME,
    gh_closed_at    DATETIME,

    author          TEXT,
    additions       INTEGER DEFAULT 0,
    deletions       INTEGER DEFAULT 0,
    changed_files   INTEGER DEFAULT 0,

    review_time_hours    REAL,
    merge_time_hours     REAL,

    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(repo_id, number)
);

CREATE TABLE IF NOT EXISTS pr_issue_links (
    pr_id           INTEGER NOT NULL REFERENCES pull_requests(id),
    issue_id        INTEGER NOT NULL REFERENCES issues(id),
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (pr_id, issue_id)
);

-- ═══════════════════════════════════════════════════════════════
-- STATUS TRANSITIONS
-- ═══════════════════════════════════════════════════════════════

CREATE TABLE IF NOT EXISTS status_transitions (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    issue_id        INTEGER NOT NULL REFERENCES issues(id),
    from_status     TEXT,
    to_status       TEXT NOT NULL,
    transitioned_at DATETIME NOT NULL,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS blocked_periods (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    issue_id        INTEGER NOT NULL REFERENCES issues(id),
    blocked_at      DATETIME NOT NULL,
    unblocked_at    DATETIME,
    duration_hours  REAL,
    reason          TEXT,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- ═══════════════════════════════════════════════════════════════
-- METRICS SNAPSHOTS
-- ═══════════════════════════════════════════════════════════════

CREATE TABLE IF NOT EXISTS metrics_daily (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    repo_id         INTEGER NOT NULL REFERENCES repositories(id),
    snapshot_date   DATE NOT NULL,

    wip_backlog     INTEGER DEFAULT 0,
    wip_ready       INTEGER DEFAULT 0,
    wip_in_progress INTEGER DEFAULT 0,
    wip_review      INTEGER DEFAULT 0,
    wip_testing     INTEGER DEFAULT 0,
    wip_done        INTEGER DEFAULT 0,
    wip_total       INTEGER DEFAULT 0,

    throughput_30d      INTEGER,
    lead_time_avg_30d   REAL,
    lead_time_p85_30d   REAL,
    cycle_time_avg_30d  REAL,
    cycle_time_p85_30d  REAL,

    arrival_rate        REAL,
    departure_rate      REAL,

    littles_law_wip     REAL,
    littles_law_variance REAL,

    flow_efficiency     REAL,

    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(repo_id, snapshot_date)
);

CREATE TABLE IF NOT EXISTS cfd_data (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    repo_id         INTEGER NOT NULL REFERENCES repositories(id),
    snapshot_date   DATE NOT NULL,
    status          TEXT NOT NULL,
    cumulative_count INTEGER NOT NULL,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(repo_id, snapshot_date, status)
);

-- ═══════════════════════════════════════════════════════════════
-- SYNC METADATA
-- ═══════════════════════════════════════════════════════════════

CREATE TABLE IF NOT EXISTS sync_history (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    repo_id         INTEGER REFERENCES repositories(id),
    sync_type       TEXT NOT NULL,
    started_at      DATETIME NOT NULL,
    completed_at    DATETIME,
    status          TEXT NOT NULL,
    items_synced    INTEGER DEFAULT 0,
    error_message   TEXT,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS cache_metadata (
    key             TEXT PRIMARY KEY,
    value           TEXT,
    expires_at      DATETIME,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- ═══════════════════════════════════════════════════════════════
-- INDEXES
-- ═══════════════════════════════════════════════════════════════

CREATE INDEX IF NOT EXISTS idx_issues_repo_status ON issues(repo_id, current_status);
CREATE INDEX IF NOT EXISTS idx_issues_repo_state ON issues(repo_id, state);
CREATE INDEX IF NOT EXISTS idx_issues_gh_updated ON issues(gh_updated_at);
CREATE INDEX IF NOT EXISTS idx_issues_gh_closed ON issues(gh_closed_at);
CREATE INDEX IF NOT EXISTS idx_issues_state_status ON issues(state, current_status);
CREATE INDEX IF NOT EXISTS idx_issues_repo_number ON issues(repo_id, number);
CREATE INDEX IF NOT EXISTS idx_transitions_issue ON status_transitions(issue_id, transitioned_at);
CREATE INDEX IF NOT EXISTS idx_metrics_repo_date ON metrics_daily(repo_id, snapshot_date);
CREATE INDEX IF NOT EXISTS idx_cfd_repo_date ON cfd_data(repo_id, snapshot_date);
CREATE INDEX IF NOT EXISTS idx_repos_fullname ON repositories(full_name);
CREATE INDEX IF NOT EXISTS idx_blocked_issue ON blocked_periods(issue_id);
CREATE INDEX IF NOT EXISTS idx_prs_repo_state ON pull_requests(repo_id, state);
CREATE INDEX IF NOT EXISTS idx_prs_author ON pull_requests(author);
CREATE INDEX IF NOT EXISTS idx_pr_links_pr ON pr_issue_links(pr_id);
CREATE INDEX IF NOT EXISTS idx_pr_links_issue ON pr_issue_links(issue_id);
`

// Views contains the database views
const Views = `
-- ═══════════════════════════════════════════════════════════════
-- VIEWS
-- ═══════════════════════════════════════════════════════════════

CREATE VIEW IF NOT EXISTS board_view AS
SELECT
    r.full_name as repo,
    i.number,
    i.title,
    i.current_status as status,
    i.current_priority as priority,
    i.current_type as type,
    i.assignee,
    i.is_blocked,
    COALESCE(i.blocked_time_hours, 0) as blocked_time_hours,
    ROUND((julianday('now') - julianday(i.gh_updated_at)) * 24, 1) as age_hours,
    i.gh_created_at,
    i.gh_updated_at
FROM issues i
JOIN repositories r ON i.repo_id = r.id
WHERE i.state = 'open' OR (i.state = 'closed' AND i.current_status = 'done')
ORDER BY r.full_name, i.current_status, i.current_priority;

CREATE VIEW IF NOT EXISTS wip_summary AS
SELECT
    r.full_name as repo,
    i.current_status as status,
    COUNT(*) as count,
    AVG((julianday('now') - julianday(i.gh_updated_at)) * 24) as avg_age_hours
FROM issues i
JOIN repositories r ON i.repo_id = r.id
WHERE i.state = 'open' OR (i.state = 'closed' AND i.current_status = 'done')
GROUP BY r.full_name, i.current_status;

CREATE VIEW IF NOT EXISTS throughput_30d AS
SELECT
    r.full_name as repo,
    COUNT(*) as completed,
    AVG(i.lead_time_hours) as avg_lead_time_hours,
    AVG(i.cycle_time_hours) as avg_cycle_time_hours
FROM issues i
JOIN repositories r ON i.repo_id = r.id
WHERE i.state = 'closed'
  AND i.gh_closed_at > datetime('now', '-30 days')
GROUP BY r.full_name;
`
