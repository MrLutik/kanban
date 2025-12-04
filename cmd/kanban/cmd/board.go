package cmd

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/kiracore/kanban/internal/config"
	"github.com/kiracore/kanban/internal/db"
	"github.com/kiracore/kanban/internal/github"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	showClosed  bool
	maxIssues   int
	liveMode    bool
	sortBy      string
	filterAssignee string
)

var boardCmd = &cobra.Command{
	Use:   "board",
	Short: "Display kanban board in terminal",
	Long: `Display issues organized by kanban status columns.

Shows a terminal-friendly view of your kanban board with issues
grouped by their status labels.

By default, uses cached data from the local database.
Use --live to fetch directly from GitHub API.

Examples:
  # View board for a repo (from cache)
  kanban board --org myorg --repo myrepo

  # Sort by updated time (newest first)
  kanban board --org myorg --repo myrepo --sort updated

  # Sort by age (oldest first, like gh)
  kanban board --org myorg --repo myrepo --sort age

  # Filter by assignee
  kanban board --org myorg --repo myrepo --assignee username

  # View board directly from GitHub
  kanban board --org myorg --repo myrepo --live`,
	RunE: runBoard,
}

func init() {
	rootCmd.AddCommand(boardCmd)
	boardCmd.Flags().StringVarP(&repo, "repo", "r", "", "specific repository")
	boardCmd.Flags().BoolVar(&allRepos, "all", false, "show board for all repositories")
	boardCmd.Flags().BoolVar(&showClosed, "closed", false, "include closed issues")
	boardCmd.Flags().IntVarP(&maxIssues, "limit", "n", 10, "max issues per column")
	boardCmd.Flags().BoolVar(&liveMode, "live", false, "fetch directly from GitHub API")
	boardCmd.Flags().StringVarP(&sortBy, "sort", "s", "priority", "sort by: priority, updated, age, assignee, created")
	boardCmd.Flags().StringVarP(&filterAssignee, "assignee", "a", "", "filter by assignee username")
}

// DisplayIssue represents an issue for board display with repo info
type DisplayIssue struct {
	Number    int
	Title     string
	Repo      string
	Priority  string
	Type      string
	Assignee  string
	IsBlocked bool
	CreatedAt time.Time
	UpdatedAt time.Time
	AgeHours  float64
}

// BoardColumn represents a kanban column
type BoardColumn struct {
	Name   string
	Color  string
	Issues []DisplayIssue
}

func runBoard(cmd *cobra.Command, args []string) error {
	organization := viper.GetString("organization")
	if organization == "" && org != "" {
		organization = org
	}

	if organization == "" {
		return fmt.Errorf("organization required: use --org flag or set in config")
	}

	// Define columns (status labels)
	columns := []BoardColumn{
		{Name: "backlog", Color: "\033[90m"},      // Gray
		{Name: "ready", Color: "\033[34m"},        // Blue
		{Name: "in-progress", Color: "\033[33m"},  // Yellow
		{Name: "review", Color: "\033[31m"},       // Red/Orange
		{Name: "testing", Color: "\033[35m"},      // Purple
		{Name: "done", Color: "\033[32m"},         // Green
	}

	var repos []string
	var err error

	if liveMode {
		// Live mode: fetch directly from GitHub
		columns, repos, err = runBoardLive(organization, columns)
	} else {
		// Cached mode: use database
		columns, repos, err = runBoardCached(organization, columns)
	}

	if err != nil {
		return err
	}

	// Apply filtering and sorting to each column
	for i := range columns {
		// Filter by assignee if specified
		if filterAssignee != "" {
			filtered := []DisplayIssue{}
			for _, issue := range columns[i].Issues {
				if strings.EqualFold(issue.Assignee, filterAssignee) {
					filtered = append(filtered, issue)
				}
			}
			columns[i].Issues = filtered
		}

		// Sort issues within column
		sortIssues(columns[i].Issues, sortBy)

		// Apply limit
		if maxIssues > 0 && len(columns[i].Issues) > maxIssues {
			columns[i].Issues = columns[i].Issues[:maxIssues]
		}
	}

	// Print board header
	reset := "\033[0m"
	bold := "\033[1m"
	dim := "\033[90m"

	source := "cached"
	if liveMode {
		source = "live"
	}

	sortInfo := ""
	if sortBy != "priority" {
		sortInfo = fmt.Sprintf(", sorted by %s", sortBy)
	}

	filterInfo := ""
	if filterAssignee != "" {
		filterInfo = fmt.Sprintf(", @%s only", filterAssignee)
	}

	if len(repos) == 1 {
		fmt.Printf("\n%s%s/%s - Kanban Board%s %s(%s%s%s)%s\n", bold, organization, repos[0], reset, dim, source, sortInfo, filterInfo, reset)
	} else {
		fmt.Printf("\n%s%s - Kanban Board (%d repos)%s %s(%s%s%s)%s\n", bold, organization, len(repos), reset, dim, source, sortInfo, filterInfo, reset)
	}
	fmt.Println(strings.Repeat("─", 80))

	// Print each column
	for _, col := range columns {
		count := len(col.Issues)
		fmt.Printf("\n%s%s● %s%s (%d)\n", col.Color, bold, strings.ToUpper(col.Name), reset, count)

		if count == 0 {
			fmt.Printf("  %s(empty)%s\n", "\033[90m", reset)
			continue
		}

		for _, issue := range col.Issues {
			repoPrefix := ""
			if len(repos) > 1 {
				repoPrefix = fmt.Sprintf("[%s] ", issue.Repo)
			}

			priorityBadge := ""
			if issue.Priority != "" {
				switch issue.Priority {
				case "critical":
					priorityBadge = "\033[91m!!\033[0m "
				case "high":
					priorityBadge = "\033[33m!\033[0m "
				}
			}

			blockedBadge := ""
			if issue.IsBlocked {
				blockedBadge = "\033[91m⊘\033[0m "
			}

			assigneePart := ""
			if issue.Assignee != "" {
				assigneePart = fmt.Sprintf(" \033[36m@%s\033[0m", issue.Assignee)
			}

			// Show age when sorting by time-based fields
			agePart := ""
			if sortBy == "age" || sortBy == "updated" || sortBy == "created" {
				agePart = fmt.Sprintf(" %s%s%s", dim, formatAge(issue.AgeHours), reset)
			}

			fmt.Printf("  %s#%-4d %s%s%s%s%s%s\n", repoPrefix, issue.Number, blockedBadge, priorityBadge, issue.Title, assigneePart, agePart, reset)
		}
	}

	// Print summary
	fmt.Println()
	fmt.Println(strings.Repeat("─", 80))

	total := 0
	summaryParts := []string{}
	for _, col := range columns {
		count := len(col.Issues)
		total += count
		if count > 0 {
			summaryParts = append(summaryParts, fmt.Sprintf("%s%s:%s %d", col.Color, col.Name, reset, count))
		}
	}

	fmt.Printf("Total: %d issues  │  %s\n\n", total, strings.Join(summaryParts, "  "))

	return nil
}

// runBoardCached fetches board data from the local database
func runBoardCached(organization string, columns []BoardColumn) ([]BoardColumn, []string, error) {
	database, err := db.Open(dbPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open database: %w (run 'kanban sync' first or use --live)", err)
	}
	defer database.Close()

	// Determine repo filter
	repoFilter := ""
	if repo != "" {
		repoFilter = fmt.Sprintf("%s/%s", organization, repo)
	}

	// Get issues from database for each status
	repoSet := make(map[string]bool)
	for i := range columns {
		issues, err := database.GetBoardIssues(repoFilter, columns[i].Name)
		if err != nil {
			continue
		}
		for _, issue := range issues {
			if !showClosed && columns[i].Name == "done" {
				// Skip done issues unless --closed is specified
				continue
			}
			columns[i].Issues = append(columns[i].Issues, DisplayIssue{
				Number:    issue.Number,
				Title:     truncate(issue.Title, 40),
				Repo:      strings.TrimPrefix(issue.Repo, organization+"/"),
				Priority:  issue.Priority,
				Type:      issue.Type,
				Assignee:  issue.Assignee,
				IsBlocked: issue.IsBlocked,
				CreatedAt: issue.CreatedAt,
				UpdatedAt: issue.UpdatedAt,
				AgeHours:  issue.AgeHours,
			})
			repoSet[issue.Repo] = true
		}
	}

	var repos []string
	if repo != "" {
		repos = []string{repo}
	} else {
		for r := range repoSet {
			repos = append(repos, strings.TrimPrefix(r, organization+"/"))
		}
	}

	return columns, repos, nil
}

// runBoardLive fetches board data directly from GitHub API
func runBoardLive(organization string, columns []BoardColumn) ([]BoardColumn, []string, error) {
	client := github.NewClient()

	// Determine target repos
	var repos []string
	var err error
	if repo != "" {
		repos = []string{repo}
	} else if allRepos {
		repos, err = client.ListRepos(organization)
		if err != nil {
			return nil, nil, err
		}
		cfg, _ := config.Load()
		if cfg != nil {
			repos = cfg.FilterRepos(repos)
		}
	} else {
		return nil, nil, fmt.Errorf("specify --repo or --all")
	}

	// Collect issues for each column
	for i := range columns {
		label := "status: " + columns[i].Name
		for _, r := range repos {
			issues, err := client.ListIssuesForBoard(organization, r, label, showClosed, maxIssues)
			if err != nil {
				continue
			}
			for _, issue := range issues {
				columns[i].Issues = append(columns[i].Issues, DisplayIssue{
					Number:    issue.Number,
					Title:     truncate(issue.Title, 40),
					Repo:      r,
					Priority:  extractLabel(issue.Labels, "priority:"),
					Type:      extractLabel(issue.Labels, "type:"),
					Assignee:  issue.Assignee,
					IsBlocked: hasLabelInList(issue.Labels, "blocked"),
				})
			}
		}
	}

	return columns, repos, nil
}

func hasLabelInList(labels []string, target string) bool {
	for _, l := range labels {
		if strings.EqualFold(l, target) {
			return true
		}
	}
	return false
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func extractLabel(labels []string, prefix string) string {
	for _, l := range labels {
		if strings.HasPrefix(l, prefix) {
			return strings.TrimPrefix(l, prefix)
		}
		// Also check with space after colon
		prefixWithSpace := strings.TrimSuffix(prefix, ":") + ": "
		if strings.HasPrefix(l, prefixWithSpace) {
			return strings.TrimPrefix(l, prefixWithSpace)
		}
	}
	return ""
}

// sortIssues sorts issues based on the specified sort method
func sortIssues(issues []DisplayIssue, sortMethod string) {
	switch sortMethod {
	case "updated":
		// Newest updated first
		sort.Slice(issues, func(i, j int) bool {
			return issues[i].UpdatedAt.After(issues[j].UpdatedAt)
		})
	case "age":
		// Oldest first (longest in column) - like gh style
		sort.Slice(issues, func(i, j int) bool {
			return issues[i].AgeHours > issues[j].AgeHours
		})
	case "created":
		// Newest created first
		sort.Slice(issues, func(i, j int) bool {
			return issues[i].CreatedAt.After(issues[j].CreatedAt)
		})
	case "assignee":
		// Group by assignee alphabetically, unassigned last
		sort.Slice(issues, func(i, j int) bool {
			if issues[i].Assignee == "" && issues[j].Assignee != "" {
				return false
			}
			if issues[i].Assignee != "" && issues[j].Assignee == "" {
				return true
			}
			return issues[i].Assignee < issues[j].Assignee
		})
	case "priority":
		fallthrough
	default:
		// Priority order: critical > high > medium > low > none
		priorityOrder := map[string]int{
			"critical": 0,
			"high":     1,
			"medium":   2,
			"low":      3,
			"":         4,
		}
		sort.Slice(issues, func(i, j int) bool {
			pi := priorityOrder[issues[i].Priority]
			pj := priorityOrder[issues[j].Priority]
			if pi != pj {
				return pi < pj
			}
			// Secondary sort by age (oldest first)
			return issues[i].AgeHours > issues[j].AgeHours
		})
	}
}

// formatAge formats hours into a human-readable relative time (like GitHub)
func formatAge(hours float64) string {
	if hours < 1 {
		mins := int(hours * 60)
		if mins < 1 {
			return "now"
		}
		return fmt.Sprintf("%dm", mins)
	}
	if hours < 24 {
		return fmt.Sprintf("%dh", int(hours))
	}
	days := int(hours / 24)
	if days < 30 {
		return fmt.Sprintf("%dd", days)
	}
	months := days / 30
	if months < 12 {
		return fmt.Sprintf("%dmo", months)
	}
	years := months / 12
	return fmt.Sprintf("%dy", years)
}
