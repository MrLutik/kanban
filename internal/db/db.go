package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/kiracore/kanban/internal/paths"
	_ "modernc.org/sqlite"
)

// DB represents the kanban database
type DB struct {
	*sql.DB
	path string
}

// DefaultDBPath returns the default database path.
// Uses XDG_DATA_HOME/kanban/kanban.db or ~/.local/share/kanban/kanban.db
func DefaultDBPath() string {
	return paths.DatabasePath()
}

// Open opens or creates the database
func Open(path string) (*DB, error) {
	if path == "" {
		path = DefaultDBPath()
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	// Performance-optimized connection string
	connStr := path + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=cache_size(-64000)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Connection pool settings for better concurrency
	db.SetMaxOpenConns(1) // SQLite works best with single connection
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0) // Keep connection open indefinitely

	return &DB{DB: db, path: path}, nil
}

// Path returns the database file path
func (db *DB) Path() string {
	return db.path
}

// Init initializes the database schema
func (db *DB) Init() error {
	// Check if already initialized
	var version int
	err := db.QueryRow("SELECT version FROM schema_version ORDER BY version DESC LIMIT 1").Scan(&version)
	if err == nil && version >= SchemaVersion {
		return nil // Already up to date
	}

	// Create schema
	if _, err := db.Exec(Schema); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	// Create views
	if _, err := db.Exec(Views); err != nil {
		return fmt.Errorf("failed to create views: %w", err)
	}

	// Record schema version
	_, err = db.Exec("INSERT OR REPLACE INTO schema_version (version) VALUES (?)", SchemaVersion)
	if err != nil {
		return fmt.Errorf("failed to record schema version: %w", err)
	}

	return nil
}

// Backup copies the database to the specified path
func (db *DB) Backup(destPath string) error {
	// Close WAL checkpoint first
	if _, err := db.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		return fmt.Errorf("failed to checkpoint: %w", err)
	}

	src, err := os.Open(db.path)
	if err != nil {
		return fmt.Errorf("failed to open source: %w", err)
	}
	defer src.Close()

	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	dst, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("failed to copy: %w", err)
	}

	return nil
}

// Restore restores the database from a backup
func (db *DB) Restore(srcPath string) error {
	// Close the current database
	if err := db.Close(); err != nil {
		return fmt.Errorf("failed to close database: %w", err)
	}

	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open backup: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(db.path)
	if err != nil {
		return fmt.Errorf("failed to create destination: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("failed to copy: %w", err)
	}

	return nil
}

// Stats returns database statistics
type Stats struct {
	Path          string    `json:"path"`
	Size          int64     `json:"size_bytes"`
	Organizations int       `json:"organizations"`
	Repositories  int       `json:"repositories"`
	Issues        int       `json:"issues"`
	PullRequests  int       `json:"pull_requests"`
	Labels        int       `json:"labels"`
	Transitions   int       `json:"transitions"`
	MetricsDays   int       `json:"metrics_days"`
	LastSync      time.Time `json:"last_sync"`
	SchemaVersion int       `json:"schema_version"`
}

// GetStats returns database statistics
func (db *DB) GetStats() (*Stats, error) {
	stats := &Stats{Path: db.path}

	// File size
	info, err := os.Stat(db.path)
	if err == nil {
		stats.Size = info.Size()
	}

	// Counts
	db.QueryRow("SELECT COUNT(*) FROM organizations").Scan(&stats.Organizations)
	db.QueryRow("SELECT COUNT(*) FROM repositories").Scan(&stats.Repositories)
	db.QueryRow("SELECT COUNT(*) FROM issues").Scan(&stats.Issues)
	db.QueryRow("SELECT COUNT(*) FROM pull_requests").Scan(&stats.PullRequests)
	db.QueryRow("SELECT COUNT(*) FROM labels").Scan(&stats.Labels)
	db.QueryRow("SELECT COUNT(*) FROM status_transitions").Scan(&stats.Transitions)
	db.QueryRow("SELECT COUNT(DISTINCT snapshot_date) FROM metrics_daily").Scan(&stats.MetricsDays)
	db.QueryRow("SELECT version FROM schema_version ORDER BY version DESC LIMIT 1").Scan(&stats.SchemaVersion)

	// Last sync - scan as string since SQLite stores as TEXT
	var lastSyncStr sql.NullString
	db.QueryRow("SELECT MAX(last_sync_at) FROM repositories").Scan(&lastSyncStr)
	if lastSyncStr.Valid && lastSyncStr.String != "" {
		// Parse SQLite datetime format
		if t, err := time.Parse("2006-01-02 15:04:05", lastSyncStr.String); err == nil {
			stats.LastSync = t
		}
	}

	return stats, nil
}

// ExportData exports all data to JSON
type ExportData struct {
	ExportedAt    time.Time      `json:"exported_at"`
	SchemaVersion int            `json:"schema_version"`
	Organizations []Organization `json:"organizations"`
	Repositories  []Repository   `json:"repositories"`
	Labels        []Label        `json:"labels"`
	Issues        []Issue        `json:"issues"`
}

// Export exports the database to JSON
func (db *DB) Export(w io.Writer) error {
	data := ExportData{
		ExportedAt:    time.Now().UTC(),
		SchemaVersion: SchemaVersion,
	}

	// Export organizations
	rows, err := db.Query("SELECT id, name, created_at FROM organizations")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var o Organization
		rows.Scan(&o.ID, &o.Name, &o.CreatedAt)
		data.Organizations = append(data.Organizations, o)
	}

	// Export repositories
	rows, err = db.Query("SELECT id, org_id, name, full_name, is_active, last_sync_at, created_at FROM repositories")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var r Repository
		var lastSync sql.NullTime
		rows.Scan(&r.ID, &r.OrgID, &r.Name, &r.FullName, &r.IsActive, &lastSync, &r.CreatedAt)
		if lastSync.Valid {
			r.LastSyncAt = &lastSync.Time
		}
		data.Repositories = append(data.Repositories, r)
	}

	// Export labels
	rows, err = db.Query("SELECT id, repo_id, name, color, description, category FROM labels")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var l Label
		rows.Scan(&l.ID, &l.RepoID, &l.Name, &l.Color, &l.Description, &l.Category)
		data.Labels = append(data.Labels, l)
	}

	// Export issues
	rows, err = db.Query(`SELECT id, repo_id, number, title, state,
		gh_created_at, gh_updated_at, gh_closed_at,
		current_status, current_priority, current_type, current_size, is_blocked, assignee,
		lead_time_hours, cycle_time_hours, blocked_time_hours FROM issues`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var i Issue
		var closedAt sql.NullTime
		var status, priority, itype, size, assignee sql.NullString
		var leadTime, cycleTime, blockedTime sql.NullFloat64
		rows.Scan(&i.ID, &i.RepoID, &i.Number, &i.Title, &i.State,
			&i.GHCreatedAt, &i.GHUpdatedAt, &closedAt,
			&status, &priority, &itype, &size, &i.IsBlocked, &assignee,
			&leadTime, &cycleTime, &blockedTime)
		if closedAt.Valid {
			i.GHClosedAt = &closedAt.Time
		}
		if status.Valid {
			i.CurrentStatus = status.String
		}
		if priority.Valid {
			i.CurrentPriority = priority.String
		}
		if itype.Valid {
			i.CurrentType = itype.String
		}
		if size.Valid {
			i.CurrentSize = size.String
		}
		if assignee.Valid {
			i.Assignee = assignee.String
		}
		if leadTime.Valid {
			i.LeadTimeHours = leadTime.Float64
		}
		if cycleTime.Valid {
			i.CycleTimeHours = cycleTime.Float64
		}
		if blockedTime.Valid {
			i.BlockedTimeHours = blockedTime.Float64
		}
		data.Issues = append(data.Issues, i)
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// Import imports data from JSON
func (db *DB) Import(r io.Reader) error {
	var data ExportData
	if err := json.NewDecoder(r).Decode(&data); err != nil {
		return fmt.Errorf("failed to decode JSON: %w", err)
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Import organizations
	for _, o := range data.Organizations {
		_, err := tx.Exec(`INSERT OR REPLACE INTO organizations (id, name, created_at) VALUES (?, ?, ?)`,
			o.ID, o.Name, o.CreatedAt)
		if err != nil {
			return fmt.Errorf("failed to import organization: %w", err)
		}
	}

	// Import repositories
	for _, r := range data.Repositories {
		_, err := tx.Exec(`INSERT OR REPLACE INTO repositories
			(id, org_id, name, full_name, is_active, last_sync_at, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			r.ID, r.OrgID, r.Name, r.FullName, r.IsActive, r.LastSyncAt, r.CreatedAt)
		if err != nil {
			return fmt.Errorf("failed to import repository: %w", err)
		}
	}

	// Import labels
	for _, l := range data.Labels {
		_, err := tx.Exec(`INSERT OR REPLACE INTO labels
			(id, repo_id, name, color, description, category) VALUES (?, ?, ?, ?, ?, ?)`,
			l.ID, l.RepoID, l.Name, l.Color, l.Description, l.Category)
		if err != nil {
			return fmt.Errorf("failed to import label: %w", err)
		}
	}

	// Import issues
	for _, i := range data.Issues {
		_, err := tx.Exec(`INSERT OR REPLACE INTO issues
			(id, repo_id, number, title, state, gh_created_at, gh_updated_at, gh_closed_at,
			current_status, current_priority, current_type, current_size, is_blocked, assignee,
			lead_time_hours, cycle_time_hours, blocked_time_hours)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			i.ID, i.RepoID, i.Number, i.Title, i.State,
			i.GHCreatedAt, i.GHUpdatedAt, i.GHClosedAt,
			i.CurrentStatus, i.CurrentPriority, i.CurrentType, i.CurrentSize, i.IsBlocked, i.Assignee,
			i.LeadTimeHours, i.CycleTimeHours, i.BlockedTimeHours)
		if err != nil {
			return fmt.Errorf("failed to import issue: %w", err)
		}
	}

	return tx.Commit()
}
