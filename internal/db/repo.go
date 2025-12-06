package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// GetOrCreateOrg gets or creates an organization
func (db *DB) GetOrCreateOrg(name string) (*Organization, error) {
	var org Organization

	err := db.QueryRow("SELECT id, name, created_at FROM organizations WHERE name = ?", name).
		Scan(&org.ID, &org.Name, &org.CreatedAt)

	if err == sql.ErrNoRows {
		result, err := db.Exec("INSERT INTO organizations (name) VALUES (?)", name)
		if err != nil {
			return nil, err
		}
		org.ID, _ = result.LastInsertId()
		org.Name = name
		org.CreatedAt = time.Now()
	} else if err != nil {
		return nil, err
	}

	return &org, nil
}

// GetOrCreateRepo gets or creates a repository
func (db *DB) GetOrCreateRepo(orgID int64, name, fullName string) (*Repository, error) {
	var repo Repository

	err := db.QueryRow(`SELECT id, org_id, name, full_name, is_active, last_sync_at, created_at
		FROM repositories WHERE full_name = ?`, fullName).
		Scan(&repo.ID, &repo.OrgID, &repo.Name, &repo.FullName, &repo.IsActive, &repo.LastSyncAt, &repo.CreatedAt)

	if err == sql.ErrNoRows {
		result, err := db.Exec("INSERT INTO repositories (org_id, name, full_name) VALUES (?, ?, ?)",
			orgID, name, fullName)
		if err != nil {
			return nil, err
		}
		repo.ID, _ = result.LastInsertId()
		repo.OrgID = orgID
		repo.Name = name
		repo.FullName = fullName
		repo.IsActive = true
		repo.CreatedAt = time.Now()
	} else if err != nil {
		return nil, err
	}

	return &repo, nil
}

// UpdateRepoSyncTime updates the last sync time for a repo
func (db *DB) UpdateRepoSyncTime(repoID int64) error {
	_, err := db.Exec("UPDATE repositories SET last_sync_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE id = ?", repoID)
	return err
}

// GetRepoLastSync returns the last sync time for a repo
func (db *DB) GetRepoLastSync(repoID int64) (*time.Time, error) {
	var lastSync sql.NullTime
	err := db.QueryRow("SELECT last_sync_at FROM repositories WHERE id = ?", repoID).Scan(&lastSync)
	if err != nil {
		return nil, err
	}
	if lastSync.Valid {
		return &lastSync.Time, nil
	}
	return nil, nil
}

// UpsertIssue inserts or updates an issue
func (db *DB) UpsertIssue(issue *Issue) error {
	// Get existing issue to check for status changes
	var existingID int64
	var existingStatus sql.NullString
	err := db.QueryRow("SELECT id, current_status FROM issues WHERE repo_id = ? AND number = ?",
		issue.RepoID, issue.Number).Scan(&existingID, &existingStatus)

	if err == sql.ErrNoRows {
		// Insert new issue
		result, err := db.Exec(`INSERT INTO issues
			(repo_id, number, title, state, gh_created_at, gh_updated_at, gh_closed_at,
			current_status, current_priority, current_type, current_size, is_blocked, assignee,
			entered_ready_at, entered_progress_at, entered_review_at, entered_testing_at, entered_done_at,
			lead_time_hours, cycle_time_hours, blocked_time_hours)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			issue.RepoID, issue.Number, issue.Title, issue.State,
			issue.GHCreatedAt, issue.GHUpdatedAt, issue.GHClosedAt,
			nullString(issue.CurrentStatus), nullString(issue.CurrentPriority),
			nullString(issue.CurrentType), nullString(issue.CurrentSize),
			issue.IsBlocked, nullString(issue.Assignee),
			issue.EnteredReadyAt, issue.EnteredProgressAt, issue.EnteredReviewAt,
			issue.EnteredTestingAt, issue.EnteredDoneAt,
			issue.LeadTimeHours, issue.CycleTimeHours, issue.BlockedTimeHours)
		if err != nil {
			return err
		}
		issue.ID, _ = result.LastInsertId()

		// Record initial status transition
		if issue.CurrentStatus != "" {
			db.RecordStatusTransition(issue.ID, "", issue.CurrentStatus, issue.GHUpdatedAt)
		}
	} else if err != nil {
		return err
	} else {
		// Update existing issue
		issue.ID = existingID

		// Check for status change
		oldStatus := ""
		if existingStatus.Valid {
			oldStatus = existingStatus.String
		}
		if oldStatus != issue.CurrentStatus {
			db.RecordStatusTransition(issue.ID, oldStatus, issue.CurrentStatus, time.Now())
			// Update entered_*_at timestamps
			db.updateStatusTimestamp(issue.ID, issue.CurrentStatus)
		}

		_, err := db.Exec(`UPDATE issues SET
			title = ?, state = ?, gh_updated_at = ?, gh_closed_at = ?,
			current_status = ?, current_priority = ?, current_type = ?, current_size = ?,
			is_blocked = ?, assignee = ?,
			lead_time_hours = ?, cycle_time_hours = ?, blocked_time_hours = ?,
			updated_at = CURRENT_TIMESTAMP
			WHERE id = ?`,
			issue.Title, issue.State, issue.GHUpdatedAt, issue.GHClosedAt,
			nullString(issue.CurrentStatus), nullString(issue.CurrentPriority),
			nullString(issue.CurrentType), nullString(issue.CurrentSize),
			issue.IsBlocked, nullString(issue.Assignee),
			issue.LeadTimeHours, issue.CycleTimeHours, issue.BlockedTimeHours,
			issue.ID)
		if err != nil {
			return err
		}
	}

	return nil
}

func (db *DB) updateStatusTimestamp(issueID int64, status string) {
	column := ""
	switch status {
	case "ready":
		column = "entered_ready_at"
	case "in-progress":
		column = "entered_progress_at"
	case "review":
		column = "entered_review_at"
	case "testing":
		column = "entered_testing_at"
	case "done":
		column = "entered_done_at"
	}
	if column != "" {
		db.Exec("UPDATE issues SET "+column+" = CURRENT_TIMESTAMP WHERE id = ? AND "+column+" IS NULL",
			issueID)
	}
}

// RecordStatusTransition records a status change
func (db *DB) RecordStatusTransition(issueID int64, fromStatus, toStatus string, transitionedAt time.Time) error {
	_, err := db.Exec(`INSERT INTO status_transitions (issue_id, from_status, to_status, transitioned_at)
		VALUES (?, ?, ?, ?)`, issueID, nullString(fromStatus), toStatus, transitionedAt)
	return err
}

// GetLabelsByRepo returns all labels for a repository
func (db *DB) GetLabelsByRepo(repoID int64) ([]Label, error) {
	rows, err := db.Query(`SELECT id, repo_id, name, color, description, category
		FROM labels WHERE repo_id = ?`, repoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var labels []Label
	for rows.Next() {
		var l Label
		var desc, cat sql.NullString
		if err := rows.Scan(&l.ID, &l.RepoID, &l.Name, &l.Color, &desc, &cat); err != nil {
			return nil, err
		}
		if desc.Valid {
			l.Description = desc.String
		}
		if cat.Valid {
			l.Category = cat.String
		}
		labels = append(labels, l)
	}
	return labels, nil
}

// LabelsNeedSync compares config labels with cached DB labels
// Returns true if any label is missing, has different color, or different description
func (db *DB) LabelsNeedSync(repoID int64, names, colors, descriptions []string) (bool, error) {
	cached, err := db.GetLabelsByRepo(repoID)
	if err != nil {
		return true, err // On error, assume sync is needed
	}

	// Build map of cached labels
	cachedMap := make(map[string]Label)
	for _, l := range cached {
		cachedMap[l.Name] = l
	}

	// Check each config label
	for i := range names {
		existing, exists := cachedMap[names[i]]
		if !exists {
			return true, nil // Missing label
		}
		if existing.Color != colors[i] || existing.Description != descriptions[i] {
			return true, nil // Different color or description
		}
	}

	return false, nil // All labels match
}

// UpsertLabel inserts or updates a label
func (db *DB) UpsertLabel(label *Label) error {
	// Determine category from label name
	label.Category = categorizeLabel(label.Name)

	result, err := db.Exec(`INSERT INTO labels (repo_id, name, color, description, category)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(repo_id, name) DO UPDATE SET
		color = excluded.color, description = excluded.description, category = excluded.category,
		updated_at = CURRENT_TIMESTAMP`,
		label.RepoID, label.Name, label.Color, label.Description, label.Category)
	if err != nil {
		return err
	}
	if label.ID == 0 {
		label.ID, _ = result.LastInsertId()
	}
	return nil
}

// categorizeLabel determines the category of a label
func categorizeLabel(name string) string {
	lower := strings.ToLower(name)
	if strings.HasPrefix(lower, "status:") || strings.HasPrefix(lower, "status ") {
		return "status"
	}
	if strings.HasPrefix(lower, "priority:") || strings.HasPrefix(lower, "priority ") {
		return "priority"
	}
	if strings.HasPrefix(lower, "type:") || strings.HasPrefix(lower, "type ") {
		return "type"
	}
	if strings.HasPrefix(lower, "size:") || strings.HasPrefix(lower, "size ") {
		return "size"
	}
	return "special"
}

// GetBoardIssues returns issues for board display
func (db *DB) GetBoardIssues(repoFullName string, status string) ([]BoardIssue, error) {
	query := `SELECT repo, number, title, status, priority, type, assignee, is_blocked, blocked_time_hours, age_hours, gh_created_at, gh_updated_at
		FROM board_view WHERE 1=1`
	args := []interface{}{}

	if repoFullName != "" {
		query += " AND repo = ?"
		args = append(args, repoFullName)
	}
	if status != "" {
		query += " AND status = ?"
		args = append(args, status)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var issues []BoardIssue
	for rows.Next() {
		var i BoardIssue
		var priority, itype, assignee, status sql.NullString
		var blockedTimeHours, ageHours sql.NullFloat64
		err := rows.Scan(&i.Repo, &i.Number, &i.Title, &status, &priority, &itype, &assignee,
			&i.IsBlocked, &blockedTimeHours, &ageHours, &i.CreatedAt, &i.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan error: %w", err)
		}
		if status.Valid {
			i.Status = status.String
		}
		if priority.Valid {
			i.Priority = priority.String
		}
		if itype.Valid {
			i.Type = itype.String
		}
		if assignee.Valid {
			i.Assignee = assignee.String
		}
		if blockedTimeHours.Valid {
			i.BlockedTimeHours = blockedTimeHours.Float64
		}
		if ageHours.Valid {
			i.AgeHours = ageHours.Float64
		}
		issues = append(issues, i)
	}

	return issues, nil
}

// GetWIPSummary returns WIP summary
func (db *DB) GetWIPSummary(repoFullName string) ([]WIPSummary, error) {
	query := "SELECT repo, status, count, avg_age_hours FROM wip_summary"
	args := []interface{}{}

	if repoFullName != "" {
		query += " WHERE repo = ?"
		args = append(args, repoFullName)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []WIPSummary
	for rows.Next() {
		var s WIPSummary
		rows.Scan(&s.Repo, &s.Status, &s.Count, &s.AvgAgeHours)
		summaries = append(summaries, s)
	}

	return summaries, nil
}

// RecordSyncStart records the start of a sync operation
func (db *DB) RecordSyncStart(repoID *int64, syncType string) (int64, error) {
	result, err := db.Exec(`INSERT INTO sync_history (repo_id, sync_type, started_at, status)
		VALUES (?, ?, CURRENT_TIMESTAMP, 'running')`, repoID, syncType)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// RecordSyncComplete records completion of a sync operation
func (db *DB) RecordSyncComplete(syncID int64, itemsSynced int, errMsg string) error {
	status := "completed"
	if errMsg != "" {
		status = "failed"
	}
	_, err := db.Exec(`UPDATE sync_history SET
		completed_at = CURRENT_TIMESTAMP, status = ?, items_synced = ?, error_message = ?
		WHERE id = ?`, status, itemsSynced, nullString(errMsg), syncID)
	return err
}

// SaveMetricsSnapshot saves daily metrics
func (db *DB) SaveMetricsSnapshot(m *MetricsDaily) error {
	_, err := db.Exec(`INSERT OR REPLACE INTO metrics_daily
		(repo_id, snapshot_date, wip_backlog, wip_ready, wip_in_progress, wip_review, wip_testing, wip_done, wip_total,
		throughput_30d, lead_time_avg_30d, lead_time_p85_30d, cycle_time_avg_30d, cycle_time_p85_30d,
		arrival_rate, departure_rate, littles_law_wip, littles_law_variance, flow_efficiency)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.RepoID, m.SnapshotDate.Format("2006-01-02"),
		m.WIPBacklog, m.WIPReady, m.WIPInProgress, m.WIPReview, m.WIPTesting, m.WIPDone, m.WIPTotal,
		m.Throughput30d, m.LeadTimeAvg30d, m.LeadTimeP8530d, m.CycleTimeAvg30d, m.CycleTimeP8530d,
		m.ArrivalRate, m.DepartureRate, m.LittlesLawWIP, m.LittlesLawVariance, m.FlowEfficiency)
	return err
}

// GetMetricsHistory returns metrics history for a repo
func (db *DB) GetMetricsHistory(repoID int64, days int) ([]MetricsDaily, error) {
	rows, err := db.Query(`SELECT id, repo_id, snapshot_date,
		wip_backlog, wip_ready, wip_in_progress, wip_review, wip_testing, wip_done, wip_total,
		throughput_30d, lead_time_avg_30d, lead_time_p85_30d, cycle_time_avg_30d, cycle_time_p85_30d,
		arrival_rate, departure_rate, littles_law_wip, littles_law_variance, flow_efficiency
		FROM metrics_daily WHERE repo_id = ? AND snapshot_date > date('now', '-' || ? || ' days')
		ORDER BY snapshot_date`, repoID, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metrics []MetricsDaily
	for rows.Next() {
		var m MetricsDaily
		rows.Scan(&m.ID, &m.RepoID, &m.SnapshotDate,
			&m.WIPBacklog, &m.WIPReady, &m.WIPInProgress, &m.WIPReview, &m.WIPTesting, &m.WIPDone, &m.WIPTotal,
			&m.Throughput30d, &m.LeadTimeAvg30d, &m.LeadTimeP8530d, &m.CycleTimeAvg30d, &m.CycleTimeP8530d,
			&m.ArrivalRate, &m.DepartureRate, &m.LittlesLawWIP, &m.LittlesLawVariance, &m.FlowEfficiency)
		metrics = append(metrics, m)
	}

	return metrics, nil
}

// RecordBlockedPeriod inserts a blocked period
func (db *DB) RecordBlockedPeriod(issueID int64, blockedAt, unblockedAt *time.Time, reason string) error {
	var duration float64
	if unblockedAt != nil && blockedAt != nil {
		duration = unblockedAt.Sub(*blockedAt).Hours()
	}
	_, err := db.Exec(`INSERT INTO blocked_periods (issue_id, blocked_at, unblocked_at, duration_hours, reason)
		VALUES (?, ?, ?, ?, ?)`, issueID, blockedAt, unblockedAt, duration, nullString(reason))
	return err
}

// UpdateIssueBlockedTime updates total blocked time for an issue
func (db *DB) UpdateIssueBlockedTime(issueID int64, totalHours float64) error {
	_, err := db.Exec("UPDATE issues SET blocked_time_hours = ?, is_blocked = ? WHERE id = ?",
		totalHours, totalHours > 0, issueID)
	return err
}

// UpdateIssueTimestamps updates the entered_*_at timestamps from timeline data
func (db *DB) UpdateIssueTimestamps(issueID int64, ready, progress, review, testing, done *time.Time) error {
	_, err := db.Exec(`UPDATE issues SET
		entered_ready_at = COALESCE(?, entered_ready_at),
		entered_progress_at = COALESCE(?, entered_progress_at),
		entered_review_at = COALESCE(?, entered_review_at),
		entered_testing_at = COALESCE(?, entered_testing_at),
		entered_done_at = COALESCE(?, entered_done_at),
		updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`, ready, progress, review, testing, done, issueID)
	return err
}

// RecalcCycleTime recalculates cycle time from timestamps
// Cycle time: only calculated when issue went through in-progress (real workflow)
// Lead time: calculated for all closed issues (creation → done)
func (db *DB) RecalcCycleTime(issueID int64) error {
	// Note: REPLACE strips Go time format suffix " +0000 UTC" for SQLite julianday compatibility
	_, err := db.Exec(`UPDATE issues SET
		cycle_time_hours = CASE
			WHEN entered_progress_at IS NOT NULL AND (entered_done_at IS NOT NULL OR gh_closed_at IS NOT NULL)
			THEN (julianday(REPLACE(REPLACE(COALESCE(entered_done_at, gh_closed_at), ' +0000 UTC', ''), ' UTC', ''))
			    - julianday(REPLACE(REPLACE(entered_progress_at, ' +0000 UTC', ''), ' UTC', ''))) * 24
			    - COALESCE(blocked_time_hours, 0)
			ELSE NULL
		END,
		lead_time_hours = CASE
			WHEN entered_done_at IS NOT NULL OR gh_closed_at IS NOT NULL
			THEN (julianday(REPLACE(REPLACE(COALESCE(entered_done_at, gh_closed_at), ' +0000 UTC', ''), ' UTC', ''))
			    - julianday(gh_created_at)) * 24
			ELSE NULL
		END
		WHERE id = ?`, issueID)
	return err
}

// GetIssueByRepoAndNumber gets an issue by repo and number
func (db *DB) GetIssueByRepoAndNumber(repoID int64, number int) (*Issue, error) {
	var i Issue
	var closedAt, readyAt, progressAt, reviewAt, testingAt, doneAt sql.NullTime
	var status, priority, itype, size, assignee sql.NullString

	err := db.QueryRow(`SELECT id, repo_id, number, title, state,
		gh_created_at, gh_updated_at, gh_closed_at,
		current_status, current_priority, current_type, current_size, is_blocked, assignee,
		entered_ready_at, entered_progress_at, entered_review_at, entered_testing_at, entered_done_at,
		lead_time_hours, cycle_time_hours, blocked_time_hours
		FROM issues WHERE repo_id = ? AND number = ?`, repoID, number).Scan(
		&i.ID, &i.RepoID, &i.Number, &i.Title, &i.State,
		&i.GHCreatedAt, &i.GHUpdatedAt, &closedAt,
		&status, &priority, &itype, &size, &i.IsBlocked, &assignee,
		&readyAt, &progressAt, &reviewAt, &testingAt, &doneAt,
		&i.LeadTimeHours, &i.CycleTimeHours, &i.BlockedTimeHours)

	if err != nil {
		return nil, err
	}

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
	if readyAt.Valid {
		i.EnteredReadyAt = &readyAt.Time
	}
	if progressAt.Valid {
		i.EnteredProgressAt = &progressAt.Time
	}
	if reviewAt.Valid {
		i.EnteredReviewAt = &reviewAt.Time
	}
	if testingAt.Valid {
		i.EnteredTestingAt = &testingAt.Time
	}
	if doneAt.Valid {
		i.EnteredDoneAt = &doneAt.Time
	}

	return &i, nil
}

// SaveCFDSnapshot saves CFD data for a date
func (db *DB) SaveCFDSnapshot(repoID int64, date time.Time, statusCounts map[string]int) error {
	for status, count := range statusCounts {
		_, err := db.Exec(`INSERT OR REPLACE INTO cfd_data (repo_id, snapshot_date, status, cumulative_count)
			VALUES (?, ?, ?, ?)`, repoID, date.Format("2006-01-02"), status, count)
		if err != nil {
			return err
		}
	}
	return nil
}

// GetCFDData returns CFD data for a repo
func (db *DB) GetCFDData(repoID int64, days int) ([]struct {
	Date   string
	Status string
	Count  int
}, error) {
	rows, err := db.Query(`SELECT snapshot_date, status, cumulative_count
		FROM cfd_data WHERE repo_id = ? AND snapshot_date > date('now', '-' || ? || ' days')
		ORDER BY snapshot_date, status`, repoID, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var data []struct {
		Date   string
		Status string
		Count  int
	}
	for rows.Next() {
		var d struct {
			Date   string
			Status string
			Count  int
		}
		rows.Scan(&d.Date, &d.Status, &d.Count)
		data = append(data, d)
	}
	return data, nil
}

// GetLastCFDSnapshot returns the date of the last CFD snapshot
func (db *DB) GetLastCFDSnapshot(repoID int64) (*time.Time, error) {
	var dateStr sql.NullString
	err := db.QueryRow("SELECT MAX(snapshot_date) FROM cfd_data WHERE repo_id = ?", repoID).Scan(&dateStr)
	if err != nil || !dateStr.Valid {
		return nil, err
	}
	t, err := time.Parse("2006-01-02", dateStr.String)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// GetStatusCounts returns current issue counts per status for a repo
func (db *DB) GetStatusCounts(repoID int64) (map[string]int, error) {
	rows, err := db.Query(`SELECT COALESCE(current_status, 'none'), COUNT(*)
		FROM issues WHERE repo_id = ? AND state = 'open' GROUP BY current_status`, repoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		rows.Scan(&status, &count)
		counts[status] = count
	}
	return counts, nil
}

// helper to convert empty string to NULL
func nullString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// ClosedIssueStats represents a closed issue with timing data
type ClosedIssueStats struct {
	Number         int
	Title          string
	CreatedAt      time.Time
	ClosedAt       time.Time
	LeadTimeHours  float64
	CycleTimeHours float64
}

// GetClosedIssuesInPeriod returns closed issues within the specified days for flow metrics
func (db *DB) GetClosedIssuesInPeriod(repoFilter string, days int) ([]ClosedIssueStats, error) {
	query := `SELECT i.number, i.title, i.gh_created_at, i.gh_closed_at,
		COALESCE(i.lead_time_hours, 0), COALESCE(i.cycle_time_hours, 0)
		FROM issues i
		JOIN repositories r ON i.repo_id = r.id
		WHERE i.state = 'closed'
		AND i.gh_closed_at > datetime('now', '-' || ? || ' days')`
	args := []interface{}{days}

	if repoFilter != "" {
		query += " AND r.full_name = ?"
		args = append(args, repoFilter)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var issues []ClosedIssueStats
	for rows.Next() {
		var issue ClosedIssueStats
		var createdAt, closedAt string
		err := rows.Scan(&issue.Number, &issue.Title, &createdAt, &closedAt,
			&issue.LeadTimeHours, &issue.CycleTimeHours)
		if err != nil {
			continue
		}
		issue.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		issue.ClosedAt, _ = time.Parse(time.RFC3339, closedAt)

		// Calculate lead time if not stored
		if issue.LeadTimeHours == 0 && !issue.ClosedAt.IsZero() && !issue.CreatedAt.IsZero() {
			issue.LeadTimeHours = issue.ClosedAt.Sub(issue.CreatedAt).Hours()
		}

		issues = append(issues, issue)
	}
	return issues, nil
}

// GetThroughputByRepo returns throughput data grouped by repo
func (db *DB) GetThroughputByRepo(days int) (map[string]int, error) {
	query := `SELECT r.full_name, COUNT(*) as completed
		FROM issues i
		JOIN repositories r ON i.repo_id = r.id
		WHERE i.state = 'closed'
		AND i.gh_closed_at > datetime('now', '-' || ? || ' days')
		GROUP BY r.full_name`

	rows, err := db.Query(query, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]int)
	for rows.Next() {
		var repo string
		var count int
		rows.Scan(&repo, &count)
		result[repo] = count
	}
	return result, nil
}

// GetArrivalByRepo returns count of issues created in the period, grouped by repo
func (db *DB) GetArrivalByRepo(days int) (map[string]int, error) {
	query := `SELECT r.full_name, COUNT(*) as created
		FROM issues i
		JOIN repositories r ON i.repo_id = r.id
		WHERE i.gh_created_at > datetime('now', '-' || ? || ' days')
		GROUP BY r.full_name`

	rows, err := db.Query(query, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]int)
	for rows.Next() {
		var repo string
		var count int
		rows.Scan(&repo, &count)
		result[repo] = count
	}
	return result, nil
}

// Transaction wraps a function in a database transaction
func (db *DB) Transaction(fn func(tx *Tx) error) error {
	sqlTx, err := db.Begin()
	if err != nil {
		return err
	}

	tx := &Tx{Tx: sqlTx}
	if err := fn(tx); err != nil {
		sqlTx.Rollback()
		return err
	}

	return sqlTx.Commit()
}

// Tx wraps sql.Tx with helper methods
type Tx struct {
	*sql.Tx
}

// UpsertIssueBatch inserts or updates multiple issues in a single transaction
func (db *DB) UpsertIssueBatch(issues []*Issue) error {
	if len(issues) == 0 {
		return nil
	}

	return db.Transaction(func(tx *Tx) error {
		// Prepare statement for checking existing issues
		checkStmt, err := tx.Prepare("SELECT id, current_status FROM issues WHERE repo_id = ? AND number = ?")
		if err != nil {
			return err
		}
		defer checkStmt.Close()

		// Prepare insert statement
		insertStmt, err := tx.Prepare(`INSERT INTO issues
			(repo_id, number, title, state, gh_created_at, gh_updated_at, gh_closed_at,
			current_status, current_priority, current_type, current_size, is_blocked, assignee,
			entered_ready_at, entered_progress_at, entered_review_at, entered_testing_at, entered_done_at,
			lead_time_hours, cycle_time_hours, blocked_time_hours)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
		if err != nil {
			return err
		}
		defer insertStmt.Close()

		// Prepare update statement
		updateStmt, err := tx.Prepare(`UPDATE issues SET
			title = ?, state = ?, gh_updated_at = ?, gh_closed_at = ?,
			current_status = ?, current_priority = ?, current_type = ?, current_size = ?,
			is_blocked = ?, assignee = ?,
			lead_time_hours = ?, cycle_time_hours = ?, blocked_time_hours = ?,
			updated_at = CURRENT_TIMESTAMP
			WHERE id = ?`)
		if err != nil {
			return err
		}
		defer updateStmt.Close()

		for _, issue := range issues {
			var existingID int64
			var existingStatus sql.NullString

			err := checkStmt.QueryRow(issue.RepoID, issue.Number).Scan(&existingID, &existingStatus)
			if err == sql.ErrNoRows {
				// Insert new issue
				result, err := insertStmt.Exec(
					issue.RepoID, issue.Number, issue.Title, issue.State,
					issue.GHCreatedAt, issue.GHUpdatedAt, issue.GHClosedAt,
					nullString(issue.CurrentStatus), nullString(issue.CurrentPriority),
					nullString(issue.CurrentType), nullString(issue.CurrentSize),
					issue.IsBlocked, nullString(issue.Assignee),
					issue.EnteredReadyAt, issue.EnteredProgressAt, issue.EnteredReviewAt,
					issue.EnteredTestingAt, issue.EnteredDoneAt,
					issue.LeadTimeHours, issue.CycleTimeHours, issue.BlockedTimeHours)
				if err != nil {
					return err
				}
				issue.ID, _ = result.LastInsertId()
			} else if err != nil {
				return err
			} else {
				// Update existing issue
				issue.ID = existingID
				_, err := updateStmt.Exec(
					issue.Title, issue.State, issue.GHUpdatedAt, issue.GHClosedAt,
					nullString(issue.CurrentStatus), nullString(issue.CurrentPriority),
					nullString(issue.CurrentType), nullString(issue.CurrentSize),
					issue.IsBlocked, nullString(issue.Assignee),
					issue.LeadTimeHours, issue.CycleTimeHours, issue.BlockedTimeHours,
					issue.ID)
				if err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// UpsertLabelBatch inserts or updates multiple labels in a single transaction
func (db *DB) UpsertLabelBatch(labels []*Label) error {
	if len(labels) == 0 {
		return nil
	}

	return db.Transaction(func(tx *Tx) error {
		stmt, err := tx.Prepare(`INSERT INTO labels (repo_id, name, color, description, category)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(repo_id, name) DO UPDATE SET
			color = excluded.color, description = excluded.description, category = excluded.category,
			updated_at = CURRENT_TIMESTAMP`)
		if err != nil {
			return err
		}
		defer stmt.Close()

		for _, label := range labels {
			label.Category = categorizeLabel(label.Name)
			_, err := stmt.Exec(label.RepoID, label.Name, label.Color, label.Description, label.Category)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// Vacuum optimizes the database file
func (db *DB) Vacuum() error {
	_, err := db.Exec("VACUUM")
	return err
}

// Analyze updates query planner statistics
func (db *DB) Analyze() error {
	_, err := db.Exec("ANALYZE")
	return err
}

// Optimize runs both VACUUM and ANALYZE
func (db *DB) Optimize() error {
	if err := db.Vacuum(); err != nil {
		return err
	}
	return db.Analyze()
}

// UpsertPR inserts or updates a pull request
func (db *DB) UpsertPR(pr *PullRequest) error {
	// Calculate review and merge times
	if pr.GHMergedAt != nil {
		pr.MergeTimeHours = pr.GHMergedAt.Sub(pr.GHCreatedAt).Hours()
	}

	var existingID int64
	err := db.QueryRow("SELECT id FROM pull_requests WHERE repo_id = ? AND number = ?",
		pr.RepoID, pr.Number).Scan(&existingID)

	if err == sql.ErrNoRows {
		// Insert new PR
		result, err := db.Exec(`INSERT INTO pull_requests
			(repo_id, number, title, state, is_draft,
			gh_created_at, gh_updated_at, gh_merged_at, gh_closed_at,
			author, additions, deletions, changed_files,
			review_time_hours, merge_time_hours)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			pr.RepoID, pr.Number, pr.Title, pr.State, pr.IsDraft,
			pr.GHCreatedAt, pr.GHUpdatedAt, pr.GHMergedAt, pr.GHClosedAt,
			nullString(pr.Author), pr.Additions, pr.Deletions, pr.ChangedFiles,
			pr.ReviewTimeHours, pr.MergeTimeHours)
		if err != nil {
			return err
		}
		pr.ID, _ = result.LastInsertId()
	} else if err != nil {
		return err
	} else {
		// Update existing PR
		pr.ID = existingID
		_, err := db.Exec(`UPDATE pull_requests SET
			title = ?, state = ?, is_draft = ?,
			gh_updated_at = ?, gh_merged_at = ?, gh_closed_at = ?,
			author = ?, additions = ?, deletions = ?, changed_files = ?,
			review_time_hours = ?, merge_time_hours = ?,
			updated_at = CURRENT_TIMESTAMP
			WHERE id = ?`,
			pr.Title, pr.State, pr.IsDraft,
			pr.GHUpdatedAt, pr.GHMergedAt, pr.GHClosedAt,
			nullString(pr.Author), pr.Additions, pr.Deletions, pr.ChangedFiles,
			pr.ReviewTimeHours, pr.MergeTimeHours,
			pr.ID)
		if err != nil {
			return err
		}
	}

	return nil
}

// LinkPRToIssue creates a link between a PR and an issue
func (db *DB) LinkPRToIssue(prID, issueID int64) error {
	_, err := db.Exec(`INSERT OR IGNORE INTO pr_issue_links (pr_id, issue_id)
		VALUES (?, ?)`, prID, issueID)
	return err
}

// GetPRsByRepo returns PRs for a repository
func (db *DB) GetPRsByRepo(repoID int64, state string) ([]PullRequest, error) {
	query := `SELECT id, repo_id, number, title, state, is_draft,
		gh_created_at, gh_updated_at, gh_merged_at, gh_closed_at,
		author, additions, deletions, changed_files,
		review_time_hours, merge_time_hours
		FROM pull_requests WHERE repo_id = ?`
	args := []interface{}{repoID}

	if state != "" && state != "all" {
		query += " AND state = ?"
		args = append(args, state)
	}
	query += " ORDER BY number DESC"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var prs []PullRequest
	for rows.Next() {
		var pr PullRequest
		var mergedAt, closedAt sql.NullTime
		var author sql.NullString
		var reviewTime, mergeTime sql.NullFloat64

		err := rows.Scan(&pr.ID, &pr.RepoID, &pr.Number, &pr.Title, &pr.State, &pr.IsDraft,
			&pr.GHCreatedAt, &pr.GHUpdatedAt, &mergedAt, &closedAt,
			&author, &pr.Additions, &pr.Deletions, &pr.ChangedFiles,
			&reviewTime, &mergeTime)
		if err != nil {
			continue
		}

		if mergedAt.Valid {
			pr.GHMergedAt = &mergedAt.Time
		}
		if closedAt.Valid {
			pr.GHClosedAt = &closedAt.Time
		}
		if author.Valid {
			pr.Author = author.String
		}
		if reviewTime.Valid {
			pr.ReviewTimeHours = reviewTime.Float64
		}
		if mergeTime.Valid {
			pr.MergeTimeHours = mergeTime.Float64
		}

		prs = append(prs, pr)
	}
	return prs, nil
}

// GetPRSummary returns PR metrics summary for a repo
func (db *DB) GetPRSummary(repoFullName string) (*PRSummary, error) {
	summary := &PRSummary{Repo: repoFullName}

	// Get repo ID
	var repoID int64
	err := db.QueryRow("SELECT id FROM repositories WHERE full_name = ?", repoFullName).Scan(&repoID)
	if err != nil {
		return nil, err
	}

	// Open PRs
	db.QueryRow("SELECT COUNT(*) FROM pull_requests WHERE repo_id = ? AND state = 'OPEN'", repoID).Scan(&summary.OpenPRs)

	// Draft PRs
	db.QueryRow("SELECT COUNT(*) FROM pull_requests WHERE repo_id = ? AND is_draft = TRUE AND state = 'OPEN'", repoID).Scan(&summary.DraftPRs)

	// Merged last 30 days
	db.QueryRow(`SELECT COUNT(*) FROM pull_requests WHERE repo_id = ?
		AND gh_merged_at > datetime('now', '-30 days')`, repoID).Scan(&summary.MergedLast30d)

	// Average merge time (for merged PRs in last 30 days)
	db.QueryRow(`SELECT AVG(merge_time_hours) FROM pull_requests WHERE repo_id = ?
		AND gh_merged_at > datetime('now', '-30 days') AND merge_time_hours > 0`, repoID).Scan(&summary.AvgMergeTimeHrs)

	// Average additions/deletions (for merged PRs in last 30 days)
	db.QueryRow(`SELECT AVG(additions), AVG(deletions) FROM pull_requests WHERE repo_id = ?
		AND gh_merged_at > datetime('now', '-30 days')`, repoID).Scan(&summary.AvgAdditions, &summary.AvgDeletions)

	return summary, nil
}

// GetLinkedIssues returns issue numbers linked to a PR
func (db *DB) GetLinkedIssues(prID int64) ([]int, error) {
	rows, err := db.Query(`SELECT i.number FROM issues i
		JOIN pr_issue_links l ON i.id = l.issue_id
		WHERE l.pr_id = ?`, prID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var issues []int
	for rows.Next() {
		var num int
		rows.Scan(&num)
		issues = append(issues, num)
	}
	return issues, nil
}

// GetIssueIDByNumber returns the issue ID for a repo and issue number
func (db *DB) GetIssueIDByNumber(repoID int64, number int) (int64, error) {
	var id int64
	err := db.QueryRow("SELECT id FROM issues WHERE repo_id = ? AND number = ?", repoID, number).Scan(&id)
	return id, err
}
