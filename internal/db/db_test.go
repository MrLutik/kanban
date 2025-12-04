package db

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupTestDB(t *testing.T) (*DB, func()) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	if err := db.Init(); err != nil {
		t.Fatalf("Failed to initialize test database: %v", err)
	}

	cleanup := func() {
		db.Close()
	}

	return db, cleanup
}

func TestOpen(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer db.Close()

	if db.Path() != dbPath {
		t.Errorf("Path() = %q, want %q", db.Path(), dbPath)
	}
}

func TestOpen_DefaultPath(t *testing.T) {
	// Test that empty path uses default
	defaultPath := DefaultDBPath()
	if defaultPath == "" {
		t.Error("DefaultDBPath() returned empty string")
	}

	// Should contain .kanban
	if !filepath.IsAbs(defaultPath) {
		t.Error("DefaultDBPath() should return absolute path")
	}
}

func TestInit(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Verify tables exist by querying them
	tables := []string{"organizations", "repositories", "issues", "labels"}
	for _, table := range tables {
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count)
		if err != nil {
			t.Errorf("Table %q should exist: %v", table, err)
		}
	}
}

func TestGetOrCreateOrg(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Create org
	org1, err := db.GetOrCreateOrg("testorg")
	if err != nil {
		t.Fatalf("GetOrCreateOrg() error: %v", err)
	}
	if org1.Name != "testorg" {
		t.Errorf("org.Name = %q, want %q", org1.Name, "testorg")
	}
	if org1.ID == 0 {
		t.Error("org.ID should not be 0")
	}

	// Get same org
	org2, err := db.GetOrCreateOrg("testorg")
	if err != nil {
		t.Fatalf("GetOrCreateOrg() error on second call: %v", err)
	}
	if org2.ID != org1.ID {
		t.Errorf("Second call returned different ID: %d vs %d", org2.ID, org1.ID)
	}
}

func TestGetOrCreateRepo(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	org, _ := db.GetOrCreateOrg("testorg")

	// Create repo
	repo1, err := db.GetOrCreateRepo(org.ID, "myrepo", "testorg/myrepo")
	if err != nil {
		t.Fatalf("GetOrCreateRepo() error: %v", err)
	}
	if repo1.Name != "myrepo" {
		t.Errorf("repo.Name = %q, want %q", repo1.Name, "myrepo")
	}
	if repo1.FullName != "testorg/myrepo" {
		t.Errorf("repo.FullName = %q, want %q", repo1.FullName, "testorg/myrepo")
	}

	// Get same repo
	repo2, err := db.GetOrCreateRepo(org.ID, "myrepo", "testorg/myrepo")
	if err != nil {
		t.Fatalf("GetOrCreateRepo() error on second call: %v", err)
	}
	if repo2.ID != repo1.ID {
		t.Errorf("Second call returned different ID: %d vs %d", repo2.ID, repo1.ID)
	}
}

func TestUpsertIssue(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	org, _ := db.GetOrCreateOrg("testorg")
	repo, _ := db.GetOrCreateRepo(org.ID, "myrepo", "testorg/myrepo")

	now := time.Now()
	issue := &Issue{
		RepoID:          repo.ID,
		Number:          1,
		Title:           "Test Issue",
		State:           "open",
		GHCreatedAt:     now,
		GHUpdatedAt:     now,
		CurrentStatus:   "backlog",
		CurrentPriority: "high",
		Assignee:        "testuser",
	}

	// Insert
	err := db.UpsertIssue(issue)
	if err != nil {
		t.Fatalf("UpsertIssue() error: %v", err)
	}
	if issue.ID == 0 {
		t.Error("issue.ID should not be 0 after insert")
	}

	// Update
	issue.Title = "Updated Title"
	issue.CurrentStatus = "in-progress"
	err = db.UpsertIssue(issue)
	if err != nil {
		t.Fatalf("UpsertIssue() error on update: %v", err)
	}

	// Verify update
	var title, status string
	err = db.QueryRow("SELECT title, current_status FROM issues WHERE id = ?", issue.ID).Scan(&title, &status)
	if err != nil {
		t.Fatalf("Failed to query updated issue: %v", err)
	}
	if title != "Updated Title" {
		t.Errorf("title = %q, want %q", title, "Updated Title")
	}
	if status != "in-progress" {
		t.Errorf("status = %q, want %q", status, "in-progress")
	}
}

func TestGetBoardIssues(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	org, _ := db.GetOrCreateOrg("testorg")
	repo, _ := db.GetOrCreateRepo(org.ID, "myrepo", "testorg/myrepo")

	now := time.Now()

	// Create test issues
	issues := []*Issue{
		{RepoID: repo.ID, Number: 1, Title: "Issue 1", State: "open", CurrentStatus: "backlog", GHCreatedAt: now, GHUpdatedAt: now},
		{RepoID: repo.ID, Number: 2, Title: "Issue 2", State: "open", CurrentStatus: "in-progress", GHCreatedAt: now, GHUpdatedAt: now},
		{RepoID: repo.ID, Number: 3, Title: "Issue 3", State: "closed", CurrentStatus: "done", GHCreatedAt: now, GHUpdatedAt: now},
	}

	for _, issue := range issues {
		if err := db.UpsertIssue(issue); err != nil {
			t.Fatalf("Failed to insert test issue: %v", err)
		}
	}

	// Get all board issues (open + done)
	boardIssues, err := db.GetBoardIssues("", "")
	if err != nil {
		t.Fatalf("GetBoardIssues() error: %v", err)
	}

	// Should include open issues and closed done issues
	if len(boardIssues) < 2 {
		t.Errorf("GetBoardIssues() returned %d issues, want at least 2", len(boardIssues))
	}

	// Filter by status
	backlogIssues, err := db.GetBoardIssues("", "backlog")
	if err != nil {
		t.Fatalf("GetBoardIssues(backlog) error: %v", err)
	}
	if len(backlogIssues) != 1 {
		t.Errorf("GetBoardIssues(backlog) returned %d issues, want 1", len(backlogIssues))
	}
}

func TestGetWIPSummary(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	org, _ := db.GetOrCreateOrg("testorg")
	repo, _ := db.GetOrCreateRepo(org.ID, "myrepo", "testorg/myrepo")

	now := time.Now()

	// Create test issues
	issues := []*Issue{
		{RepoID: repo.ID, Number: 1, Title: "Issue 1", State: "open", CurrentStatus: "backlog", GHCreatedAt: now, GHUpdatedAt: now},
		{RepoID: repo.ID, Number: 2, Title: "Issue 2", State: "open", CurrentStatus: "in-progress", GHCreatedAt: now, GHUpdatedAt: now},
		{RepoID: repo.ID, Number: 3, Title: "Issue 3", State: "open", CurrentStatus: "in-progress", GHCreatedAt: now, GHUpdatedAt: now},
	}

	for _, issue := range issues {
		db.UpsertIssue(issue)
	}

	summary, err := db.GetWIPSummary("")
	if err != nil {
		t.Fatalf("GetWIPSummary() error: %v", err)
	}

	// Should have 2 status entries (backlog and in-progress)
	if len(summary) < 2 {
		t.Errorf("GetWIPSummary() returned %d entries, want at least 2", len(summary))
	}

	// Check counts
	statusCounts := make(map[string]int)
	for _, s := range summary {
		statusCounts[s.Status] = s.Count
	}

	if statusCounts["backlog"] != 1 {
		t.Errorf("backlog count = %d, want 1", statusCounts["backlog"])
	}
	if statusCounts["in-progress"] != 2 {
		t.Errorf("in-progress count = %d, want 2", statusCounts["in-progress"])
	}
}

func TestBackupAndRestore(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Add some data
	org, _ := db.GetOrCreateOrg("testorg")
	db.GetOrCreateRepo(org.ID, "myrepo", "testorg/myrepo")

	// Backup
	tmpDir := t.TempDir()
	backupPath := filepath.Join(tmpDir, "backup.db")
	err := db.Backup(backupPath)
	if err != nil {
		t.Fatalf("Backup() error: %v", err)
	}

	// Verify backup file exists
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Error("Backup file was not created")
	}

	// Restore overwrites the current DB, so we need to reopen after restore
	restorePath := filepath.Join(tmpDir, "restored.db")
	db2, err := Open(restorePath)
	if err != nil {
		t.Fatalf("Failed to open restore DB: %v", err)
	}

	err = db2.Restore(backupPath)
	if err != nil {
		db2.Close()
		t.Fatalf("Restore() error: %v", err)
	}
	db2.Close()

	// Reopen after restore
	db3, err := Open(restorePath)
	if err != nil {
		t.Fatalf("Failed to reopen after restore: %v", err)
	}
	defer db3.Close()

	// Verify data exists in restored DB
	var count int
	err = db3.QueryRow("SELECT COUNT(*) FROM organizations").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query restored DB: %v", err)
	}
	if count != 1 {
		t.Errorf("Restored DB has %d organizations, want 1", count)
	}
}

func TestExportAndImport(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Add some data
	org, _ := db.GetOrCreateOrg("testorg")
	repo, _ := db.GetOrCreateRepo(org.ID, "myrepo", "testorg/myrepo")

	now := time.Now()
	issue := &Issue{
		RepoID:        repo.ID,
		Number:        1,
		Title:         "Test Issue",
		State:         "open",
		CurrentStatus: "backlog",
		GHCreatedAt:   now,
		GHUpdatedAt:   now,
	}
	db.UpsertIssue(issue)

	// Export
	tmpDir := t.TempDir()
	exportPath := filepath.Join(tmpDir, "export.json")
	exportFile, err := os.Create(exportPath)
	if err != nil {
		t.Fatalf("Failed to create export file: %v", err)
	}

	err = db.Export(exportFile)
	exportFile.Close()
	if err != nil {
		t.Fatalf("Export() error: %v", err)
	}

	// Verify export file is valid JSON
	exportData, _ := os.ReadFile(exportPath)
	var exported map[string]interface{}
	if err := json.Unmarshal(exportData, &exported); err != nil {
		t.Fatalf("Export produced invalid JSON: %v", err)
	}

	// Create new DB and import
	importDBPath := filepath.Join(tmpDir, "import.db")
	db2, _ := Open(importDBPath)
	defer db2.Close()
	db2.Init()

	importFile, _ := os.Open(exportPath)
	defer importFile.Close()

	err = db2.Import(importFile)
	if err != nil {
		t.Fatalf("Import() error: %v", err)
	}

	// Verify data exists in imported DB
	var count int
	db2.QueryRow("SELECT COUNT(*) FROM issues").Scan(&count)
	if count != 1 {
		t.Errorf("Imported DB has %d issues, want 1", count)
	}
}

func TestGetClosedIssuesInPeriod(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	org, _ := db.GetOrCreateOrg("testorg")
	repo, _ := db.GetOrCreateRepo(org.ID, "myrepo", "testorg/myrepo")

	now := time.Now()
	closedAt := now.Add(-24 * time.Hour) // Closed yesterday
	oldClosedAt := now.Add(-60 * 24 * time.Hour) // Closed 60 days ago

	issues := []*Issue{
		{RepoID: repo.ID, Number: 1, Title: "Recent closed", State: "closed", CurrentStatus: "done", GHCreatedAt: now.Add(-48 * time.Hour), GHUpdatedAt: now, GHClosedAt: &closedAt, LeadTimeHours: 24},
		{RepoID: repo.ID, Number: 2, Title: "Old closed", State: "closed", CurrentStatus: "done", GHCreatedAt: now.Add(-90 * 24 * time.Hour), GHUpdatedAt: now, GHClosedAt: &oldClosedAt, LeadTimeHours: 720},
		{RepoID: repo.ID, Number: 3, Title: "Still open", State: "open", CurrentStatus: "in-progress", GHCreatedAt: now, GHUpdatedAt: now},
	}

	for _, issue := range issues {
		db.UpsertIssue(issue)
	}

	// Get closed issues in last 30 days
	closed, err := db.GetClosedIssuesInPeriod("testorg/myrepo", 30)
	if err != nil {
		t.Fatalf("GetClosedIssuesInPeriod() error: %v", err)
	}

	if len(closed) != 1 {
		t.Errorf("GetClosedIssuesInPeriod() returned %d issues, want 1", len(closed))
	}
	if len(closed) > 0 && closed[0].Number != 1 {
		t.Errorf("Expected issue #1, got #%d", closed[0].Number)
	}
}

func TestRecordStatusTransition(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	org, _ := db.GetOrCreateOrg("testorg")
	repo, _ := db.GetOrCreateRepo(org.ID, "myrepo", "testorg/myrepo")

	now := time.Now()
	issue := &Issue{
		RepoID:      repo.ID,
		Number:      1,
		Title:       "Test Issue",
		State:       "open",
		GHCreatedAt: now,
		GHUpdatedAt: now,
	}
	db.UpsertIssue(issue)

	// Record transition
	transitionTime := now.Add(-1 * time.Hour)
	err := db.RecordStatusTransition(issue.ID, "backlog", "in-progress", transitionTime)
	if err != nil {
		t.Fatalf("RecordStatusTransition() error: %v", err)
	}

	// Verify transition was recorded
	var count int
	db.QueryRow("SELECT COUNT(*) FROM status_transitions WHERE issue_id = ?", issue.ID).Scan(&count)
	if count != 1 {
		t.Errorf("Expected 1 transition, got %d", count)
	}
}

func TestSaveCFDSnapshot(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	org, _ := db.GetOrCreateOrg("testorg")
	repo, _ := db.GetOrCreateRepo(org.ID, "myrepo", "testorg/myrepo")

	statusCounts := map[string]int{
		"backlog":     5,
		"in-progress": 3,
		"done":        10,
	}

	err := db.SaveCFDSnapshot(repo.ID, time.Now(), statusCounts)
	if err != nil {
		t.Fatalf("SaveCFDSnapshot() error: %v", err)
	}

	// Verify data was saved
	var count int
	db.QueryRow("SELECT COUNT(*) FROM cfd_data WHERE repo_id = ?", repo.ID).Scan(&count)
	if count != 3 {
		t.Errorf("Expected 3 CFD entries, got %d", count)
	}
}

func TestGetStats(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	org, _ := db.GetOrCreateOrg("testorg")
	repo, _ := db.GetOrCreateRepo(org.ID, "myrepo", "testorg/myrepo")

	now := time.Now()
	issue := &Issue{
		RepoID:      repo.ID,
		Number:      1,
		Title:       "Test",
		State:       "open",
		GHCreatedAt: now,
		GHUpdatedAt: now,
	}
	db.UpsertIssue(issue)

	stats, err := db.GetStats()
	if err != nil {
		t.Fatalf("GetStats() error: %v", err)
	}

	if stats.Organizations != 1 {
		t.Errorf("Organizations = %d, want 1", stats.Organizations)
	}
	if stats.Repositories != 1 {
		t.Errorf("Repositories = %d, want 1", stats.Repositories)
	}
	if stats.Issues != 1 {
		t.Errorf("Issues = %d, want 1", stats.Issues)
	}
}
