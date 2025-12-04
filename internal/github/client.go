package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/kiracore/kanban/internal/config"
)

// Client wraps GitHub operations (using gh CLI)
type Client struct{}

// NewClient creates a new GitHub client
func NewClient() *Client {
	return &Client{}
}

// ghLabel represents a label from gh CLI
type ghLabel struct {
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description"`
}

// ListRepos lists repositories in an organization
func (c *Client) ListRepos(org string) ([]string, error) {
	cmd := exec.Command("gh", "repo", "list", org, "--limit", "500", "--json", "name")

	// Unset GH_TOKEN to use default auth
	cmd.Env = filterEnv("GH_TOKEN")

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list repos: %w", err)
	}

	var repos []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(output, &repos); err != nil {
		return nil, err
	}

	var names []string
	for _, r := range repos {
		names = append(names, r.Name)
	}
	return names, nil
}

// ListLabels lists labels for a repository
func (c *Client) ListLabels(org, repo string) ([]config.Label, error) {
	cmd := exec.Command("gh", "label", "list", "--repo", fmt.Sprintf("%s/%s", org, repo), "--json", "name,color,description")
	cmd.Env = filterEnv("GH_TOKEN")

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list labels: %w", err)
	}

	var ghLabels []ghLabel
	if err := json.Unmarshal(output, &ghLabels); err != nil {
		return nil, err
	}

	var labels []config.Label
	for _, l := range ghLabels {
		labels = append(labels, config.Label{
			Name:        l.Name,
			Color:       strings.TrimPrefix(l.Color, "#"),
			Description: l.Description,
		})
	}
	return labels, nil
}

// SyncLabels syncs labels to a repository
func (c *Client) SyncLabels(org, repo string, labels []config.Label, dryRun bool) error {
	repoPath := fmt.Sprintf("%s/%s", org, repo)

	// Get current labels
	current, err := c.ListLabels(org, repo)
	if err != nil {
		return err
	}

	currentMap := make(map[string]config.Label)
	for _, l := range current {
		currentMap[l.Name] = l
	}

	// Process each label
	for _, label := range labels {
		existing, exists := currentMap[label.Name]

		if !exists {
			// Create new label
			fmt.Printf("  Creating: %s\n", label.Name)
			if !dryRun {
				if err := c.createLabel(repoPath, label); err != nil {
					fmt.Printf("    Warning: %v\n", err)
				}
			}
		} else if existing.Color != label.Color || existing.Description != label.Description {
			// Update existing label
			fmt.Printf("  Updating: %s\n", label.Name)
			if !dryRun {
				if err := c.editLabel(repoPath, label); err != nil {
					fmt.Printf("    Warning: %v\n", err)
				}
			}
		}
	}

	return nil
}

func (c *Client) createLabel(repo string, label config.Label) error {
	args := []string{"label", "create", label.Name, "--repo", repo, "--color", label.Color}
	if label.Description != "" {
		args = append(args, "--description", label.Description)
	}

	cmd := exec.Command("gh", args...)
	cmd.Env = filterEnv("GH_TOKEN")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %s", err, stderr.String())
	}
	return nil
}

func (c *Client) editLabel(repo string, label config.Label) error {
	args := []string{"label", "edit", label.Name, "--repo", repo, "--color", label.Color}
	if label.Description != "" {
		args = append(args, "--description", label.Description)
	}

	cmd := exec.Command("gh", args...)
	cmd.Env = filterEnv("GH_TOKEN")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %s", err, stderr.String())
	}
	return nil
}

// DeleteLabel deletes a label from a repository
func (c *Client) DeleteLabel(org, repo, name string, dryRun bool) error {
	if dryRun {
		fmt.Printf("  Would delete: %s\n", name)
		return nil
	}

	cmd := exec.Command("gh", "label", "delete", name, "--repo", fmt.Sprintf("%s/%s", org, repo), "--yes")
	cmd.Env = filterEnv("GH_TOKEN")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %s", err, stderr.String())
	}
	return nil
}

// MigrateIssueLabels migrates issues from one label to another
func (c *Client) MigrateIssueLabels(org, repo, fromLabel, toLabel string, dryRun bool) (int, error) {
	repoPath := fmt.Sprintf("%s/%s", org, repo)

	// Find issues with the old label
	issues, err := c.listIssuesWithLabel(repoPath, fromLabel)
	if err != nil {
		return 0, err
	}

	if len(issues) == 0 {
		return 0, nil
	}

	if dryRun {
		return len(issues), nil
	}

	// Migrate each issue
	migrated := 0
	for _, issue := range issues {
		// Add new label
		if err := c.addLabelToIssue(repoPath, issue.Number, toLabel); err != nil {
			continue
		}
		// Remove old label
		if err := c.removeLabelFromIssue(repoPath, issue.Number, fromLabel); err != nil {
			continue
		}
		migrated++
	}

	return migrated, nil
}

// ghIssue represents an issue from gh CLI
type ghIssue struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
}

// BoardIssue represents an issue for board display
type BoardIssue struct {
	Number   int      `json:"number"`
	Title    string   `json:"title"`
	Labels   []string `json:"labels"`
	Assignee string   `json:"assignee"`
}

// ListIssuesForBoard lists issues with a specific label for board display
func (c *Client) ListIssuesForBoard(org, repo, label string, includeClosed bool, limit int) ([]BoardIssue, error) {
	repoPath := fmt.Sprintf("%s/%s", org, repo)

	state := "open"
	if includeClosed {
		state = "all"
	}

	cmd := exec.Command("gh", "issue", "list",
		"--repo", repoPath,
		"--label", label,
		"--json", "number,title,labels,assignees",
		"--limit", fmt.Sprintf("%d", limit),
		"--state", state)
	cmd.Env = filterEnv("GH_TOKEN")

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list issues: %w", err)
	}

	var rawIssues []struct {
		Number    int    `json:"number"`
		Title     string `json:"title"`
		Labels    []struct {
			Name string `json:"name"`
		} `json:"labels"`
		Assignees []struct {
			Login string `json:"login"`
		} `json:"assignees"`
	}

	if err := json.Unmarshal(output, &rawIssues); err != nil {
		return nil, err
	}

	var issues []BoardIssue
	for _, ri := range rawIssues {
		var labels []string
		for _, l := range ri.Labels {
			labels = append(labels, l.Name)
		}

		assignee := ""
		if len(ri.Assignees) > 0 {
			assignee = ri.Assignees[0].Login
		}

		issues = append(issues, BoardIssue{
			Number:   ri.Number,
			Title:    ri.Title,
			Labels:   labels,
			Assignee: assignee,
		})
	}

	return issues, nil
}

func (c *Client) listIssuesWithLabel(repo, label string) ([]ghIssue, error) {
	cmd := exec.Command("gh", "issue", "list", "--repo", repo, "--label", label, "--json", "number,title", "--limit", "500", "--state", "all")
	cmd.Env = filterEnv("GH_TOKEN")

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list issues: %w", err)
	}

	var issues []ghIssue
	if err := json.Unmarshal(output, &issues); err != nil {
		return nil, err
	}

	return issues, nil
}

func (c *Client) addLabelToIssue(repo string, issueNum int, label string) error {
	cmd := exec.Command("gh", "issue", "edit", fmt.Sprintf("%d", issueNum), "--repo", repo, "--add-label", label)
	cmd.Env = filterEnv("GH_TOKEN")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %s", err, stderr.String())
	}
	return nil
}

func (c *Client) removeLabelFromIssue(repo string, issueNum int, label string) error {
	cmd := exec.Command("gh", "issue", "edit", fmt.Sprintf("%d", issueNum), "--repo", repo, "--remove-label", label)
	cmd.Env = filterEnv("GH_TOKEN")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %s", err, stderr.String())
	}
	return nil
}

// IssueDetails contains detailed issue information
type IssueDetails struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	State     string    `json:"state"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	ClosedAt  time.Time `json:"closedAt"`
	Labels    []string  `json:"labels"`
	Assignee  string    `json:"assignee"`
}

// IssueWithTimes contains issue with timeline data
type IssueWithTimes struct {
	Number          int       `json:"number"`
	Title           string    `json:"title"`
	State           string    `json:"state"`
	CreatedAt       time.Time `json:"createdAt"`
	ClosedAt        time.Time `json:"closedAt"`
	InProgressAt    time.Time `json:"inProgressAt"`    // When moved to in-progress
	ReadyAt         time.Time `json:"readyAt"`         // When moved to ready
	Labels          []string  `json:"labels"`
	BlockedDuration float64   `json:"blockedDuration"` // Hours blocked
}

// GetIssueDetails gets detailed info for a single issue
func (c *Client) GetIssueDetails(org, repo string, number int) (*IssueDetails, error) {
	repoPath := fmt.Sprintf("%s/%s", org, repo)

	cmd := exec.Command("gh", "issue", "view", fmt.Sprintf("%d", number),
		"--repo", repoPath,
		"--json", "number,title,state,createdAt,updatedAt,closedAt,labels,assignees")
	cmd.Env = filterEnv("GH_TOKEN")

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get issue details: %w", err)
	}

	var raw struct {
		Number    int       `json:"number"`
		Title     string    `json:"title"`
		State     string    `json:"state"`
		CreatedAt time.Time `json:"createdAt"`
		UpdatedAt time.Time `json:"updatedAt"`
		ClosedAt  time.Time `json:"closedAt"`
		Labels    []struct {
			Name string `json:"name"`
		} `json:"labels"`
		Assignees []struct {
			Login string `json:"login"`
		} `json:"assignees"`
	}

	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, err
	}

	details := &IssueDetails{
		Number:    raw.Number,
		Title:     raw.Title,
		State:     raw.State,
		CreatedAt: raw.CreatedAt,
		UpdatedAt: raw.UpdatedAt,
		ClosedAt:  raw.ClosedAt,
	}

	for _, l := range raw.Labels {
		details.Labels = append(details.Labels, l.Name)
	}

	if len(raw.Assignees) > 0 {
		details.Assignee = raw.Assignees[0].Login
	}

	return details, nil
}

// ListClosedIssuesWithTimes lists closed issues with timing info
func (c *Client) ListClosedIssuesWithTimes(org, repo string, days int) ([]IssueWithTimes, error) {
	repoPath := fmt.Sprintf("%s/%s", org, repo)
	since := time.Now().AddDate(0, 0, -days).Format("2006-01-02")

	cmd := exec.Command("gh", "issue", "list",
		"--repo", repoPath,
		"--state", "closed",
		"--json", "number,title,state,createdAt,closedAt,labels",
		"--limit", "500",
		"--search", fmt.Sprintf("closed:>=%s", since))
	cmd.Env = filterEnv("GH_TOKEN")

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list closed issues: %w", err)
	}

	var rawIssues []struct {
		Number    int       `json:"number"`
		Title     string    `json:"title"`
		State     string    `json:"state"`
		CreatedAt time.Time `json:"createdAt"`
		ClosedAt  time.Time `json:"closedAt"`
		Labels    []struct {
			Name string `json:"name"`
		} `json:"labels"`
	}

	if err := json.Unmarshal(output, &rawIssues); err != nil {
		return nil, err
	}

	var issues []IssueWithTimes
	for _, ri := range rawIssues {
		issue := IssueWithTimes{
			Number:    ri.Number,
			Title:     ri.Title,
			State:     ri.State,
			CreatedAt: ri.CreatedAt,
			ClosedAt:  ri.ClosedAt,
		}
		for _, l := range ri.Labels {
			issue.Labels = append(issue.Labels, l.Name)
		}
		issues = append(issues, issue)
	}

	return issues, nil
}

// TimelineEvent represents a label change event
type TimelineEvent struct {
	Event     string    `json:"event"`      // "labeled" or "unlabeled"
	Label     string    `json:"label"`      // label name
	CreatedAt time.Time `json:"created_at"` // when it happened
}

// TimelineResult contains parsed timeline data
type TimelineResult struct {
	Events         []TimelineEvent
	StatusChanges  map[string]time.Time // status -> first time entered
	BlockedPeriods []BlockedPeriod
	TotalBlocked   float64 // hours
}

// BlockedPeriod represents a period when issue was blocked
type BlockedPeriod struct {
	Start    time.Time
	End      time.Time // zero if still blocked
	Duration float64   // hours
}

// GetIssueTimeline gets timeline events for an issue
func (c *Client) GetIssueTimeline(org, repo string, number int) (*TimelineResult, error) {
	repoPath := fmt.Sprintf("%s/%s", org, repo)

	cmd := exec.Command("gh", "api",
		fmt.Sprintf("repos/%s/issues/%d/timeline", repoPath, number),
		"--paginate")
	cmd.Env = filterEnv("GH_TOKEN")

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("timeline API failed: %w", err)
	}

	var rawEvents []struct {
		Event     string    `json:"event"`
		CreatedAt time.Time `json:"created_at"`
		Label     *struct {
			Name string `json:"name"`
		} `json:"label"`
	}

	if err := json.Unmarshal(output, &rawEvents); err != nil {
		return nil, err
	}

	result := &TimelineResult{
		StatusChanges: make(map[string]time.Time),
	}

	var blockedStart time.Time

	for _, e := range rawEvents {
		if e.Label == nil {
			continue
		}

		evt := TimelineEvent{
			Event:     e.Event,
			Label:     e.Label.Name,
			CreatedAt: e.CreatedAt,
		}
		result.Events = append(result.Events, evt)

		// Track status label changes (first entry only)
		if e.Event == "labeled" && strings.HasPrefix(strings.ToLower(e.Label.Name), "status:") {
			status := extractStatus(e.Label.Name)
			if _, exists := result.StatusChanges[status]; !exists {
				result.StatusChanges[status] = e.CreatedAt
			}
		}

		// Track blocked periods
		if e.Event == "labeled" && strings.ToLower(e.Label.Name) == "blocked" {
			blockedStart = e.CreatedAt
		}
		if e.Event == "unlabeled" && strings.ToLower(e.Label.Name) == "blocked" && !blockedStart.IsZero() {
			period := BlockedPeriod{
				Start:    blockedStart,
				End:      e.CreatedAt,
				Duration: e.CreatedAt.Sub(blockedStart).Hours(),
			}
			result.BlockedPeriods = append(result.BlockedPeriods, period)
			result.TotalBlocked += period.Duration
			blockedStart = time.Time{}
		}
	}

	// If still blocked, add open period
	if !blockedStart.IsZero() {
		period := BlockedPeriod{
			Start:    blockedStart,
			Duration: time.Since(blockedStart).Hours(),
		}
		result.BlockedPeriods = append(result.BlockedPeriods, period)
		result.TotalBlocked += period.Duration
	}

	return result, nil
}

// extractStatus extracts status name from label like "status: in-progress"
func extractStatus(label string) string {
	lower := strings.ToLower(label)
	if strings.HasPrefix(lower, "status:") {
		return strings.TrimSpace(strings.TrimPrefix(lower, "status:"))
	}
	if strings.HasPrefix(lower, "status ") {
		return strings.TrimSpace(strings.TrimPrefix(lower, "status "))
	}
	return lower
}

// ListAllIssues lists all issues (open and closed) for metrics
func (c *Client) ListAllIssues(org, repo string, limit int) ([]IssueDetails, error) {
	repoPath := fmt.Sprintf("%s/%s", org, repo)

	cmd := exec.Command("gh", "issue", "list",
		"--repo", repoPath,
		"--state", "all",
		"--json", "number,title,state,createdAt,updatedAt,closedAt,labels,assignees",
		"--limit", fmt.Sprintf("%d", limit))
	cmd.Env = filterEnv("GH_TOKEN")

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list issues: %w", err)
	}

	var rawIssues []struct {
		Number    int       `json:"number"`
		Title     string    `json:"title"`
		State     string    `json:"state"`
		CreatedAt time.Time `json:"createdAt"`
		UpdatedAt time.Time `json:"updatedAt"`
		ClosedAt  time.Time `json:"closedAt"`
		Labels    []struct {
			Name string `json:"name"`
		} `json:"labels"`
		Assignees []struct {
			Login string `json:"login"`
		} `json:"assignees"`
	}

	if err := json.Unmarshal(output, &rawIssues); err != nil {
		return nil, err
	}

	var issues []IssueDetails
	for _, ri := range rawIssues {
		issue := IssueDetails{
			Number:    ri.Number,
			Title:     ri.Title,
			State:     ri.State,
			CreatedAt: ri.CreatedAt,
			UpdatedAt: ri.UpdatedAt,
			ClosedAt:  ri.ClosedAt,
		}
		for _, l := range ri.Labels {
			issue.Labels = append(issue.Labels, l.Name)
		}
		if len(ri.Assignees) > 0 {
			issue.Assignee = ri.Assignees[0].Login
		}
		issues = append(issues, issue)
	}

	return issues, nil
}

// filterEnv returns environment without specified variable
func filterEnv(exclude string) []string {
	var env []string
	for _, e := range exec.Command("").Environ() {
		if !strings.HasPrefix(e, exclude+"=") {
			env = append(env, e)
		}
	}
	return env
}

// PRDetails contains pull request information
type PRDetails struct {
	Number       int       `json:"number"`
	Title        string    `json:"title"`
	State        string    `json:"state"`
	IsDraft      bool      `json:"isDraft"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	MergedAt     time.Time `json:"mergedAt"`
	ClosedAt     time.Time `json:"closedAt"`
	Labels       []string  `json:"labels"`
	Author       string    `json:"author"`
	Assignees    []string  `json:"assignees"`
	ReviewStatus string    `json:"reviewStatus"`
	Additions    int       `json:"additions"`
	Deletions    int       `json:"deletions"`
	ChangedFiles int       `json:"changedFiles"`
	LinkedIssues []int     `json:"linkedIssues"`
}

// ListPRs lists all pull requests for a repository
func (c *Client) ListPRs(org, repo string, limit int) ([]PRDetails, error) {
	repoPath := fmt.Sprintf("%s/%s", org, repo)

	cmd := exec.Command("gh", "pr", "list",
		"--repo", repoPath,
		"--state", "all",
		"--json", "number,title,state,isDraft,createdAt,updatedAt,mergedAt,closedAt,labels,author,assignees,additions,deletions,changedFiles",
		"--limit", fmt.Sprintf("%d", limit))
	cmd.Env = filterEnv("GH_TOKEN")

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list PRs: %w", err)
	}

	var rawPRs []struct {
		Number       int       `json:"number"`
		Title        string    `json:"title"`
		State        string    `json:"state"`
		IsDraft      bool      `json:"isDraft"`
		CreatedAt    time.Time `json:"createdAt"`
		UpdatedAt    time.Time `json:"updatedAt"`
		MergedAt     time.Time `json:"mergedAt"`
		ClosedAt     time.Time `json:"closedAt"`
		Additions    int       `json:"additions"`
		Deletions    int       `json:"deletions"`
		ChangedFiles int       `json:"changedFiles"`
		Labels       []struct {
			Name string `json:"name"`
		} `json:"labels"`
		Author struct {
			Login string `json:"login"`
		} `json:"author"`
		Assignees []struct {
			Login string `json:"login"`
		} `json:"assignees"`
	}

	if err := json.Unmarshal(output, &rawPRs); err != nil {
		return nil, err
	}

	var prs []PRDetails
	for _, rp := range rawPRs {
		pr := PRDetails{
			Number:       rp.Number,
			Title:        rp.Title,
			State:        rp.State,
			IsDraft:      rp.IsDraft,
			CreatedAt:    rp.CreatedAt,
			UpdatedAt:    rp.UpdatedAt,
			MergedAt:     rp.MergedAt,
			ClosedAt:     rp.ClosedAt,
			Author:       rp.Author.Login,
			Additions:    rp.Additions,
			Deletions:    rp.Deletions,
			ChangedFiles: rp.ChangedFiles,
		}
		for _, l := range rp.Labels {
			pr.Labels = append(pr.Labels, l.Name)
		}
		for _, a := range rp.Assignees {
			pr.Assignees = append(pr.Assignees, a.Login)
		}
		prs = append(prs, pr)
	}

	return prs, nil
}

// GetPRLinkedIssues gets issues linked to a PR
func (c *Client) GetPRLinkedIssues(org, repo string, prNumber int) ([]int, error) {
	repoPath := fmt.Sprintf("%s/%s", org, repo)

	// Use GraphQL to get linked issues
	query := fmt.Sprintf(`{
		repository(owner: "%s", name: "%s") {
			pullRequest(number: %d) {
				closingIssuesReferences(first: 10) {
					nodes {
						number
					}
				}
			}
		}
	}`, org, repo, prNumber)

	cmd := exec.Command("gh", "api", "graphql", "-f", fmt.Sprintf("query=%s", query))
	cmd.Env = filterEnv("GH_TOKEN")

	output, err := cmd.Output()
	if err != nil {
		// Fallback: try to parse PR body for issue references
		return c.parseLinkedIssuesFromPR(repoPath, prNumber)
	}

	var result struct {
		Data struct {
			Repository struct {
				PullRequest struct {
					ClosingIssuesReferences struct {
						Nodes []struct {
							Number int `json:"number"`
						} `json:"nodes"`
					} `json:"closingIssuesReferences"`
				} `json:"pullRequest"`
			} `json:"repository"`
		} `json:"data"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return nil, err
	}

	var issues []int
	for _, node := range result.Data.Repository.PullRequest.ClosingIssuesReferences.Nodes {
		issues = append(issues, node.Number)
	}
	return issues, nil
}

// parseLinkedIssuesFromPR parses PR body for issue references
func (c *Client) parseLinkedIssuesFromPR(repo string, prNumber int) ([]int, error) {
	cmd := exec.Command("gh", "pr", "view", fmt.Sprintf("%d", prNumber),
		"--repo", repo,
		"--json", "body")
	cmd.Env = filterEnv("GH_TOKEN")

	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var pr struct {
		Body string `json:"body"`
	}
	if err := json.Unmarshal(output, &pr); err != nil {
		return nil, err
	}

	// Parse "Closes #123", "Fixes #456", "Resolves #789" patterns
	var issues []int
	patterns := []string{"closes", "close", "fixes", "fix", "resolves", "resolve"}
	body := strings.ToLower(pr.Body)

	for _, pattern := range patterns {
		for i := 0; i < len(body); i++ {
			idx := strings.Index(body[i:], pattern+" #")
			if idx == -1 {
				break
			}
			i += idx + len(pattern) + 2
			var num int
			for i < len(body) && body[i] >= '0' && body[i] <= '9' {
				num = num*10 + int(body[i]-'0')
				i++
			}
			if num > 0 {
				issues = append(issues, num)
			}
		}
	}

	return issues, nil
}
