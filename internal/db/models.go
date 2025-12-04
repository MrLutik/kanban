package db

import (
	"time"
)

// Organization represents a GitHub organization
type Organization struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Repository represents a GitHub repository
type Repository struct {
	ID         int64      `json:"id"`
	OrgID      int64      `json:"org_id"`
	Name       string     `json:"name"`
	FullName   string     `json:"full_name"`
	IsActive   bool       `json:"is_active"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	LastSyncAt *time.Time `json:"last_sync_at,omitempty"`
}

// Label represents a GitHub label
type Label struct {
	ID          int64     `json:"id"`
	RepoID      int64     `json:"repo_id"`
	Name        string    `json:"name"`
	Color       string    `json:"color"`
	Description string    `json:"description"`
	Category    string    `json:"category"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Issue represents a GitHub issue
type Issue struct {
	ID        int64     `json:"id"`
	RepoID    int64     `json:"repo_id"`
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	State     string    `json:"state"`

	GHCreatedAt time.Time  `json:"gh_created_at"`
	GHUpdatedAt time.Time  `json:"gh_updated_at"`
	GHClosedAt  *time.Time `json:"gh_closed_at,omitempty"`

	CurrentStatus   string `json:"current_status,omitempty"`
	CurrentPriority string `json:"current_priority,omitempty"`
	CurrentType     string `json:"current_type,omitempty"`
	CurrentSize     string `json:"current_size,omitempty"`
	IsBlocked       bool   `json:"is_blocked"`
	Assignee        string `json:"assignee,omitempty"`

	EnteredReadyAt    *time.Time `json:"entered_ready_at,omitempty"`
	EnteredProgressAt *time.Time `json:"entered_progress_at,omitempty"`
	EnteredReviewAt   *time.Time `json:"entered_review_at,omitempty"`
	EnteredTestingAt  *time.Time `json:"entered_testing_at,omitempty"`
	EnteredDoneAt     *time.Time `json:"entered_done_at,omitempty"`

	LeadTimeHours    float64 `json:"lead_time_hours,omitempty"`
	CycleTimeHours   float64 `json:"cycle_time_hours,omitempty"`
	BlockedTimeHours float64 `json:"blocked_time_hours,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// StatusTransition represents a status change
type StatusTransition struct {
	ID            int64     `json:"id"`
	IssueID       int64     `json:"issue_id"`
	FromStatus    string    `json:"from_status"`
	ToStatus      string    `json:"to_status"`
	TransitionedAt time.Time `json:"transitioned_at"`
	CreatedAt     time.Time `json:"created_at"`
}

// BlockedPeriod represents a period when an issue was blocked
type BlockedPeriod struct {
	ID            int64      `json:"id"`
	IssueID       int64      `json:"issue_id"`
	BlockedAt     time.Time  `json:"blocked_at"`
	UnblockedAt   *time.Time `json:"unblocked_at,omitempty"`
	DurationHours float64    `json:"duration_hours"`
	Reason        string     `json:"reason,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

// MetricsDaily represents daily metrics snapshot
type MetricsDaily struct {
	ID           int64     `json:"id"`
	RepoID       int64     `json:"repo_id"`
	SnapshotDate time.Time `json:"snapshot_date"`

	WIPBacklog    int `json:"wip_backlog"`
	WIPReady      int `json:"wip_ready"`
	WIPInProgress int `json:"wip_in_progress"`
	WIPReview     int `json:"wip_review"`
	WIPTesting    int `json:"wip_testing"`
	WIPDone       int `json:"wip_done"`
	WIPTotal      int `json:"wip_total"`

	Throughput30d    int     `json:"throughput_30d"`
	LeadTimeAvg30d   float64 `json:"lead_time_avg_30d"`
	LeadTimeP8530d   float64 `json:"lead_time_p85_30d"`
	CycleTimeAvg30d  float64 `json:"cycle_time_avg_30d"`
	CycleTimeP8530d  float64 `json:"cycle_time_p85_30d"`

	ArrivalRate   float64 `json:"arrival_rate"`
	DepartureRate float64 `json:"departure_rate"`

	LittlesLawWIP      float64 `json:"littles_law_wip"`
	LittlesLawVariance float64 `json:"littles_law_variance"`

	FlowEfficiency float64 `json:"flow_efficiency"`

	CreatedAt time.Time `json:"created_at"`
}

// SyncHistory represents a sync operation record
type SyncHistory struct {
	ID           int64      `json:"id"`
	RepoID       *int64     `json:"repo_id,omitempty"`
	SyncType     string     `json:"sync_type"`
	StartedAt    time.Time  `json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	Status       string     `json:"status"`
	ItemsSynced  int        `json:"items_synced"`
	ErrorMessage string     `json:"error_message,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// BoardIssue represents an issue for board display
type BoardIssue struct {
	Repo             string    `json:"repo"`
	Number           int       `json:"number"`
	Title            string    `json:"title"`
	Status           string    `json:"status"`
	Priority         string    `json:"priority"`
	Type             string    `json:"type"`
	Assignee         string    `json:"assignee"`
	IsBlocked        bool      `json:"is_blocked"`
	BlockedTimeHours float64   `json:"blocked_time_hours"`
	AgeHours         float64   `json:"age_hours"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// WIPSummary represents WIP summary per status
type WIPSummary struct {
	Repo        string  `json:"repo"`
	Status      string  `json:"status"`
	Count       int     `json:"count"`
	AvgAgeHours float64 `json:"avg_age_hours"`
}

// PullRequest represents a GitHub pull request
type PullRequest struct {
	ID        int64     `json:"id"`
	RepoID    int64     `json:"repo_id"`
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	State     string    `json:"state"`
	IsDraft   bool      `json:"is_draft"`

	GHCreatedAt time.Time  `json:"gh_created_at"`
	GHUpdatedAt time.Time  `json:"gh_updated_at"`
	GHMergedAt  *time.Time `json:"gh_merged_at,omitempty"`
	GHClosedAt  *time.Time `json:"gh_closed_at,omitempty"`

	Author       string `json:"author,omitempty"`
	Additions    int    `json:"additions"`
	Deletions    int    `json:"deletions"`
	ChangedFiles int    `json:"changed_files"`

	ReviewTimeHours float64 `json:"review_time_hours,omitempty"`
	MergeTimeHours  float64 `json:"merge_time_hours,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// PRIssueLink represents a link between a PR and an issue
type PRIssueLink struct {
	PRID      int64     `json:"pr_id"`
	IssueID   int64     `json:"issue_id"`
	CreatedAt time.Time `json:"created_at"`
}

// PRSummary represents PR metrics summary
type PRSummary struct {
	Repo              string  `json:"repo"`
	OpenPRs           int     `json:"open_prs"`
	DraftPRs          int     `json:"draft_prs"`
	MergedLast30d     int     `json:"merged_last_30d"`
	AvgReviewTimeHrs  float64 `json:"avg_review_time_hrs"`
	AvgMergeTimeHrs   float64 `json:"avg_merge_time_hrs"`
	AvgAdditions      float64 `json:"avg_additions"`
	AvgDeletions      float64 `json:"avg_deletions"`
}
