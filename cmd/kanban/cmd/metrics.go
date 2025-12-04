package cmd

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/kiracore/kanban/internal/config"
	"github.com/kiracore/kanban/internal/db"
	"github.com/kiracore/kanban/internal/github"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var days int

var metricsCmd = &cobra.Command{
	Use:   "metrics",
	Short: "Display comprehensive kanban metrics",
	Long: `Display kanban metrics and analytics including:

FLOW METRICS:
  - Lead Time: Time from creation to completion
  - Cycle Time: Time from in-progress to completion
  - Throughput: Items completed per time period
  - Flow Efficiency: Active time vs total time

WIP METRICS:
  - Work In Progress: Items in each state
  - WIP Age: Average age of items in progress
  - Little's Law: WIP = Throughput × Lead Time

RATE METRICS:
  - Arrival Rate: New items entering per period
  - Departure Rate: Items completed per period
  - Blocked Time: Time items spent blocked

DISTRIBUTION:
  - Density: Items per state as percentage
  - Flow Load: Total items in system

By default, uses cached data from the local database.
Use --live to fetch directly from GitHub API.

Examples:
  kanban metrics --org myorg --repo myrepo
  kanban metrics --org myorg --all --days 30
  kanban metrics --org myorg --repo myrepo --live

  # Show only aging issues sorted by assignee
  kanban metrics --org myorg --repo myrepo --aging --sort assignee

  # Filter by assignee
  kanban metrics --org myorg --repo myrepo --assignee username`,
	RunE: runMetrics,
}

var (
	metricsSortBy      string
	metricsAssignee    string
	showAgingOnly      bool
)

func init() {
	rootCmd.AddCommand(metricsCmd)
	metricsCmd.Flags().StringVarP(&repo, "repo", "r", "", "specific repository")
	metricsCmd.Flags().BoolVar(&allRepos, "all", false, "metrics for all repositories")
	metricsCmd.Flags().IntVar(&days, "days", 30, "time period in days")
	metricsCmd.Flags().StringVarP(&format, "format", "f", "table", "output format (table|json)")
	metricsCmd.Flags().BoolVar(&liveMode, "live", false, "fetch directly from GitHub API")
	metricsCmd.Flags().StringVarP(&metricsSortBy, "sort", "s", "age", "sort aging issues by: age, assignee, status, repo")
	metricsCmd.Flags().StringVarP(&metricsAssignee, "assignee", "a", "", "filter by assignee username")
	metricsCmd.Flags().BoolVar(&showAgingOnly, "aging", false, "show only aging issues (skip other metrics)")
}

// KanbanMetrics holds all kanban metrics
type KanbanMetrics struct {
	Repo      string    `json:"repo"`
	Generated time.Time `json:"generated"`
	Period    int       `json:"period_days"`

	// Flow Metrics
	LeadTime       TimeStats `json:"lead_time"`
	CycleTime      TimeStats `json:"cycle_time"`
	Throughput     RateStats `json:"throughput"`
	FlowEfficiency float64   `json:"flow_efficiency_percent"`

	// WIP Metrics
	WIP          map[string]int `json:"wip"`
	WIPLimits    map[string]int `json:"wip_limits,omitempty"`
	WIPAge       TimeStats      `json:"wip_age"`
	LittlesLaw   LittlesLaw     `json:"littles_law"`

	// Rate Metrics
	ArrivalRate   float64 `json:"arrival_rate_per_day"`
	DepartureRate float64 `json:"departure_rate_per_day"`
	BlockedTime   float64 `json:"blocked_time_hours"`

	// Distribution
	FlowLoad int                `json:"flow_load"`
	Density  map[string]float64 `json:"density_percent"`

	// Aging Issues
	AgingIssues []AgingIssue `json:"aging_issues"`

	// Bottlenecks
	Bottlenecks []string `json:"bottlenecks"`
}

type TimeStats struct {
	Average float64 `json:"average_days"`
	Median  float64 `json:"median_days"`
	P85     float64 `json:"p85_days"`
	Min     float64 `json:"min_days"`
	Max     float64 `json:"max_days"`
	StdDev  float64 `json:"std_dev_days"`
	Count   int     `json:"sample_count"`
}

type RateStats struct {
	Total   int     `json:"total"`
	PerDay  float64 `json:"per_day"`
	PerWeek float64 `json:"per_week"`
}

type LittlesLaw struct {
	CalculatedWIP float64 `json:"calculated_wip"`
	ActualWIP     int     `json:"actual_wip"`
	Variance      float64 `json:"variance_percent"`
}

type AgingIssue struct {
	Repo         string  `json:"repo,omitempty"`
	Number       int     `json:"number"`
	Title        string  `json:"title"`
	Status       string  `json:"status"`
	Assignee     string  `json:"assignee,omitempty"`
	AgeDays      float64 `json:"age_days"`
	BlockedHours float64 `json:"blocked_hours,omitempty"`
	IsBlocked    bool    `json:"is_blocked,omitempty"`
}

func runMetrics(cmd *cobra.Command, args []string) error {
	organization := viper.GetString("organization")
	if organization == "" && org != "" {
		organization = org
	}

	if organization == "" {
		return fmt.Errorf("organization required: use --org flag or set in config")
	}

	// Load WIP limits
	wipLimits := make(map[string]int)
	cfg, _ := config.Load()
	if cfg != nil {
		wipLimits = cfg.Settings.WIPLimits
	}

	var allMetrics []KanbanMetrics
	var err error

	if liveMode {
		// Live mode: fetch directly from GitHub
		allMetrics, err = collectMetricsLive(organization, days, wipLimits)
	} else {
		// Cached mode: use database
		allMetrics, err = collectMetricsCached(organization, days, wipLimits)
	}

	if err != nil {
		return err
	}

	// Apply filtering and sorting to aging issues
	for i := range allMetrics {
		// Filter by assignee if specified
		if metricsAssignee != "" {
			filtered := []AgingIssue{}
			for _, issue := range allMetrics[i].AgingIssues {
				if strings.EqualFold(issue.Assignee, metricsAssignee) {
					filtered = append(filtered, issue)
				}
			}
			allMetrics[i].AgingIssues = filtered
		}

		// Sort aging issues
		sortAgingIssues(allMetrics[i].AgingIssues, metricsSortBy)
	}

	source := "cached"
	if liveMode {
		source = "live"
	}

	if format == "json" {
		output, _ := json.MarshalIndent(allMetrics, "", "  ")
		fmt.Println(string(output))
	} else {
		sortInfo := ""
		if metricsSortBy != "age" {
			sortInfo = fmt.Sprintf(", sorted by %s", metricsSortBy)
		}
		filterInfo := ""
		if metricsAssignee != "" {
			filterInfo = fmt.Sprintf(", @%s", metricsAssignee)
		}
		fmt.Printf("\n[Data source: %s%s%s]\n", source, sortInfo, filterInfo)

		for _, m := range allMetrics {
			if showAgingOnly {
				printAgingIssuesOnly(m)
			} else {
				printKanbanMetrics(m)
			}
		}
	}

	return nil
}

// sortAgingIssues sorts aging issues based on the specified sort method
func sortAgingIssues(issues []AgingIssue, sortMethod string) {
	switch sortMethod {
	case "repo":
		// Group by repo alphabetically
		sort.Slice(issues, func(i, j int) bool {
			if issues[i].Repo != issues[j].Repo {
				return issues[i].Repo < issues[j].Repo
			}
			return issues[i].AgeDays > issues[j].AgeDays
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
			if issues[i].Assignee != issues[j].Assignee {
				return issues[i].Assignee < issues[j].Assignee
			}
			return issues[i].AgeDays > issues[j].AgeDays
		})
	case "status":
		// Group by status
		statusOrder := map[string]int{
			"in-progress": 0,
			"review":      1,
			"testing":     2,
			"ready":       3,
			"backlog":     4,
		}
		sort.Slice(issues, func(i, j int) bool {
			si := statusOrder[issues[i].Status]
			sj := statusOrder[issues[j].Status]
			if si != sj {
				return si < sj
			}
			return issues[i].AgeDays > issues[j].AgeDays
		})
	case "age":
		fallthrough
	default:
		// Oldest first
		sort.Slice(issues, func(i, j int) bool {
			return issues[i].AgeDays > issues[j].AgeDays
		})
	}
}

// printAgingIssuesOnly prints just the aging issues section
func printAgingIssuesOnly(m KanbanMetrics) {
	reset := "\033[0m"
	bold := "\033[1m"
	yellow := "\033[33m"
	dim := "\033[90m"

	fmt.Printf("\n%s%s══════════════════════════════════════════════════════════════%s\n", bold, yellow, reset)
	fmt.Printf("%s%s  AGING ISSUES: %s%s\n", bold, yellow, m.Repo, reset)
	fmt.Printf("%s%s══════════════════════════════════════════════════════════════%s\n", bold, yellow, reset)

	if len(m.AgingIssues) == 0 {
		fmt.Printf("%sNo aging issues%s\n", dim, reset)
		return
	}

	// Group by assignee if sorted by assignee
	if metricsSortBy == "assignee" {
		currentAssignee := ""
		for _, issue := range m.AgingIssues {
			if issue.Assignee != currentAssignee {
				currentAssignee = issue.Assignee
				if currentAssignee == "" {
					fmt.Printf("\n%s%s@unassigned%s\n", bold, dim, reset)
				} else {
					fmt.Printf("\n%s@%s%s\n", bold, currentAssignee, reset)
				}
			}
			ageColor := getAgeColor(issue.AgeDays)
			blockedStr := formatBlockedTime(issue.BlockedHours, issue.IsBlocked)
			fmt.Printf("  #%-4d %s%5.1fd%s %-11s %s%s\n",
				issue.Number, ageColor, issue.AgeDays, reset, issue.Status, issue.Title, blockedStr)
		}
	} else {
		for _, issue := range m.AgingIssues {
			assignee := ""
			if issue.Assignee != "" {
				assignee = fmt.Sprintf(" @%s", issue.Assignee)
			}
			ageColor := getAgeColor(issue.AgeDays)
			blockedStr := formatBlockedTime(issue.BlockedHours, issue.IsBlocked)
			fmt.Printf("#%-4d %s%5.1fd%s %-11s %-30s%s%s%s%s\n",
				issue.Number, ageColor, issue.AgeDays, reset,
				issue.Status, issue.Title, blockedStr, dim, assignee, reset)
		}
	}
	fmt.Println()
}

// formatBlockedTime returns a formatted string for blocked time
func formatBlockedTime(hours float64, isCurrentlyBlocked bool) string {
	if hours == 0 && !isCurrentlyBlocked {
		return ""
	}
	red := "\033[31m"
	reset := "\033[0m"

	if isCurrentlyBlocked {
		if hours > 0 {
			return fmt.Sprintf(" %s[⊘ blocked %.0fh]%s", red, hours, reset)
		}
		return fmt.Sprintf(" %s[⊘ blocked]%s", red, reset)
	}

	// Was blocked but not anymore
	if hours >= 24 {
		return fmt.Sprintf(" [was blocked %.1fd]", hours/24)
	}
	return fmt.Sprintf(" [was blocked %.0fh]", hours)
}

// collectMetricsCached collects metrics from the local database
func collectMetricsCached(organization string, days int, wipLimits map[string]int) ([]KanbanMetrics, error) {
	database, err := db.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w (run 'kanban sync' first or use --live)", err)
	}
	defer database.Close()

	// Get WIP summary from database
	repoFilter := ""
	if repo != "" {
		repoFilter = fmt.Sprintf("%s/%s", organization, repo)
	}

	wipSummary, err := database.GetWIPSummary(repoFilter)
	if err != nil {
		return nil, fmt.Errorf("failed to get WIP summary: %w", err)
	}

	// Group by repo
	repoWIP := make(map[string]map[string]int)
	for _, w := range wipSummary {
		if repoWIP[w.Repo] == nil {
			repoWIP[w.Repo] = make(map[string]int)
		}
		repoWIP[w.Repo][w.Status] = w.Count
	}

	// Get board issues for aging info
	boardIssues, err := database.GetBoardIssues(repoFilter, "")
	if err != nil {
		return nil, fmt.Errorf("failed to get board issues: %w", err)
	}

	// Group issues by repo
	repoIssues := make(map[string][]db.BoardIssue)
	for _, issue := range boardIssues {
		repoIssues[issue.Repo] = append(repoIssues[issue.Repo], issue)
	}

	// Get arrival data (new issues created in period)
	arrivalByRepo, _ := database.GetArrivalByRepo(days)

	var allMetrics []KanbanMetrics

	for repoName, wip := range repoWIP {
		m := KanbanMetrics{
			Repo:      strings.TrimPrefix(repoName, organization+"/"),
			Generated: time.Now().UTC(),
			Period:    days,
			WIP:       wip,
			WIPLimits: wipLimits,
			Density:   make(map[string]float64),
		}

		// Calculate metrics from cached data
		statuses := []string{"backlog", "ready", "in-progress", "review", "testing", "done"}
		var allAges []float64

		for _, issue := range repoIssues[repoName] {
			if issue.Status != "done" && issue.Status != "backlog" && issue.Status != "" {
				age := issue.AgeHours / 24
				allAges = append(allAges, age)

				m.AgingIssues = append(m.AgingIssues, AgingIssue{
					Repo:         m.Repo,
					Number:       issue.Number,
					Title:        truncate(issue.Title, 35),
					Status:       issue.Status,
					Assignee:     issue.Assignee,
					AgeDays:      math.Round(age*10) / 10,
					BlockedHours: issue.BlockedTimeHours,
					IsBlocked:    issue.IsBlocked,
				})
			}
		}

		// Sort aging (oldest first) and limit
		sort.Slice(m.AgingIssues, func(i, j int) bool {
			return m.AgingIssues[i].AgeDays > m.AgingIssues[j].AgeDays
		})
		if len(m.AgingIssues) > 10 {
			m.AgingIssues = m.AgingIssues[:10]
		}

		// Calculate WIP Age
		if len(allAges) > 0 {
			m.WIPAge = calculateTimeStats(allAges)
		}

		// Calculate Flow Load and Density
		for _, status := range statuses {
			m.FlowLoad += m.WIP[status]
		}
		if m.FlowLoad > 0 {
			for status, count := range m.WIP {
				m.Density[status] = math.Round(float64(count)/float64(m.FlowLoad)*1000) / 10
			}
		}

		// Calculate flow metrics from cached data
		closedIssues, err := database.GetClosedIssuesInPeriod(repoName, days)
		if err == nil && len(closedIssues) > 0 {
			// Throughput
			m.Throughput.Total = len(closedIssues)
			m.Throughput.PerDay = float64(len(closedIssues)) / float64(days)
			m.Throughput.PerWeek = m.Throughput.PerDay * 7

			// Departure Rate
			m.DepartureRate = m.Throughput.PerDay

			// Lead Time
			var leadTimes []float64
			for _, issue := range closedIssues {
				if issue.LeadTimeHours > 0 {
					leadTimes = append(leadTimes, issue.LeadTimeHours/24)
				}
			}
			if len(leadTimes) > 0 {
				m.LeadTime = calculateTimeStats(leadTimes)
			}

			// Cycle Time (only for issues that went through workflow)
			var cycleTimes []float64
			var workflowLeadTimes []float64
			for _, issue := range closedIssues {
				if issue.CycleTimeHours > 0 {
					cycleTimes = append(cycleTimes, issue.CycleTimeHours/24)
					// Also track lead time for these same issues (for accurate flow efficiency)
					if issue.LeadTimeHours > 0 {
						workflowLeadTimes = append(workflowLeadTimes, issue.LeadTimeHours/24)
					}
				}
			}
			if len(cycleTimes) > 0 {
				m.CycleTime = calculateTimeStats(cycleTimes)
				// Flow Efficiency: compare cycle/lead for SAME issues only
				if len(workflowLeadTimes) > 0 {
					workflowLead := calculateTimeStats(workflowLeadTimes)
					if workflowLead.Average > 0 {
						m.FlowEfficiency = math.Round(m.CycleTime.Average / workflowLead.Average * 100)
					}
				}
			}
		}

		// Arrival Rate (new issues created in period)
		if arrivalCount, ok := arrivalByRepo[repoName]; ok {
			m.ArrivalRate = float64(arrivalCount) / float64(days)
		}

		// Identify bottlenecks based on WIP
		m.Bottlenecks = identifyBottlenecks(m)

		allMetrics = append(allMetrics, m)
	}

	if len(allMetrics) == 0 {
		return nil, fmt.Errorf("no data found. Run 'kanban sync' first to populate the database")
	}

	return allMetrics, nil
}

// collectMetricsLive collects metrics directly from GitHub API
func collectMetricsLive(organization string, days int, wipLimits map[string]int) ([]KanbanMetrics, error) {
	client := github.NewClient()
	cfg, _ := config.Load()

	var repos []string
	var err error
	if repo != "" {
		repos = []string{repo}
	} else if cfg != nil && cfg.HasExplicitRepos() {
		repos = cfg.GetRepos()
	} else if allRepos {
		repos, err = client.ListRepos(organization)
		if err != nil {
			return nil, err
		}
		if cfg != nil {
			repos = cfg.FilterRepos(repos)
		}
	} else {
		return nil, fmt.Errorf("specify --repo, --all, or define repositories.list in config")
	}

	var allMetrics []KanbanMetrics

	for _, r := range repos {
		m, err := collectKanbanMetrics(client, organization, r, days, wipLimits)
		if err != nil {
			fmt.Printf("Warning: %s: %v\n", r, err)
			continue
		}
		allMetrics = append(allMetrics, m)
	}

	return allMetrics, nil
}

func collectKanbanMetrics(client *github.Client, org, repo string, days int, wipLimits map[string]int) (KanbanMetrics, error) {
	m := KanbanMetrics{
		Repo:      repo,
		Generated: time.Now().UTC(),
		Period:    days,
		WIP:       make(map[string]int),
		WIPLimits: wipLimits,
		Density:   make(map[string]float64),
	}

	statuses := []string{"backlog", "ready", "in-progress", "review", "testing", "done"}

	// Collect WIP and aging for each status
	var allAges []float64
	for _, status := range statuses {
		label := "status: " + status
		issues, err := client.ListIssuesForBoard(org, repo, label, false, 500)
		if err != nil {
			continue
		}
		m.WIP[status] = len(issues)

		// Collect aging for active items
		if status != "done" && status != "backlog" {
			for _, issue := range issues {
				details, err := client.GetIssueDetails(org, repo, issue.Number)
				if err != nil {
					continue
				}
				age := time.Since(details.CreatedAt).Hours() / 24
				allAges = append(allAges, age)

				m.AgingIssues = append(m.AgingIssues, AgingIssue{
					Repo:     m.Repo,
					Number:   issue.Number,
					Title:    truncate(issue.Title, 35),
					Status:   status,
					Assignee: issue.Assignee,
					AgeDays:  math.Round(age*10) / 10,
				})
			}
		}
	}

	// Sort aging (oldest first) and limit
	sort.Slice(m.AgingIssues, func(i, j int) bool {
		return m.AgingIssues[i].AgeDays > m.AgingIssues[j].AgeDays
	})
	if len(m.AgingIssues) > 10 {
		m.AgingIssues = m.AgingIssues[:10]
	}

	// Calculate WIP Age
	if len(allAges) > 0 {
		m.WIPAge = calculateTimeStats(allAges)
	}

	// Calculate Flow Load and Density
	for _, count := range m.WIP {
		m.FlowLoad += count
	}
	if m.FlowLoad > 0 {
		for status, count := range m.WIP {
			m.Density[status] = math.Round(float64(count)/float64(m.FlowLoad)*1000) / 10
		}
	}

	// Get closed issues for throughput and lead time
	closedIssues, err := client.ListClosedIssuesWithTimes(org, repo, days)
	if err == nil && len(closedIssues) > 0 {
		// Throughput
		m.Throughput.Total = len(closedIssues)
		m.Throughput.PerDay = float64(len(closedIssues)) / float64(days)
		m.Throughput.PerWeek = m.Throughput.PerDay * 7

		// Departure Rate
		m.DepartureRate = m.Throughput.PerDay

		// Lead Time (creation → closed)
		var leadTimes []float64
		for _, issue := range closedIssues {
			if !issue.ClosedAt.IsZero() && !issue.CreatedAt.IsZero() {
				lt := issue.ClosedAt.Sub(issue.CreatedAt).Hours() / 24
				if lt > 0 {
					leadTimes = append(leadTimes, lt)
				}
			}
		}
		if len(leadTimes) > 0 {
			m.LeadTime = calculateTimeStats(leadTimes)
		}
		// Cycle time not available in live mode - requires timeline data
		// Use cached mode with 'kanban sync --with-timeline' for cycle time
	}

	// Arrival Rate (new issues created in period)
	allIssues, err := client.ListAllIssues(org, repo, 500)
	if err == nil {
		cutoff := time.Now().AddDate(0, 0, -days)
		newCount := 0
		for _, issue := range allIssues {
			if issue.CreatedAt.After(cutoff) {
				newCount++
			}
		}
		m.ArrivalRate = float64(newCount) / float64(days)
	}

	// Flow Efficiency (only when we have real cycle time data)
	if m.LeadTime.Average > 0 && m.CycleTime.Count > 0 {
		m.FlowEfficiency = math.Round((m.CycleTime.Average/m.LeadTime.Average)*1000) / 10
	}

	// Little's Law: WIP = Throughput × Lead Time
	activeWIP := m.WIP["ready"] + m.WIP["in-progress"] + m.WIP["review"] + m.WIP["testing"]
	if m.Throughput.PerDay > 0 && m.LeadTime.Average > 0 {
		m.LittlesLaw.CalculatedWIP = m.Throughput.PerDay * m.LeadTime.Average
		m.LittlesLaw.ActualWIP = activeWIP
		if m.LittlesLaw.CalculatedWIP > 0 {
			m.LittlesLaw.Variance = math.Round((float64(activeWIP)-m.LittlesLaw.CalculatedWIP)/m.LittlesLaw.CalculatedWIP*1000) / 10
		}
	}

	// Identify bottlenecks
	m.Bottlenecks = identifyBottlenecks(m)

	return m, nil
}

func calculateTimeStats(values []float64) TimeStats {
	if len(values) == 0 {
		return TimeStats{}
	}

	sort.Float64s(values)
	n := len(values)

	// Sum and mean
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(n)

	// Standard deviation
	sumSquares := 0.0
	for _, v := range values {
		sumSquares += (v - mean) * (v - mean)
	}
	stdDev := math.Sqrt(sumSquares / float64(n))

	// Percentiles
	p50idx := n / 2
	p85idx := int(float64(n) * 0.85)
	if p85idx >= n {
		p85idx = n - 1
	}

	stats := TimeStats{
		Count:   n,
		Average: math.Round(mean*10) / 10,
		Min:     math.Round(values[0]*10) / 10,
		Max:     math.Round(values[n-1]*10) / 10,
		StdDev:  math.Round(stdDev*10) / 10,
		P85:     math.Round(values[p85idx]*10) / 10,
	}

	// Median
	if n%2 == 0 {
		stats.Median = math.Round((values[p50idx-1]+values[p50idx])/2*10) / 10
	} else {
		stats.Median = math.Round(values[p50idx]*10) / 10
	}

	return stats
}

func identifyBottlenecks(m KanbanMetrics) []string {
	var bottlenecks []string

	// WIP limit violations
	for status, count := range m.WIP {
		if limit, ok := m.WIPLimits["status: "+status]; ok && count > limit {
			bottlenecks = append(bottlenecks, fmt.Sprintf("WIP LIMIT: %s has %d items (limit: %d)", status, count, limit))
		}
	}

	// Arrival > Departure (system overload)
	if m.ArrivalRate > m.DepartureRate*1.5 && m.ArrivalRate > 0.5 {
		bottlenecks = append(bottlenecks, fmt.Sprintf("OVERLOAD: Arrival rate (%.1f/day) > Departure rate (%.1f/day)", m.ArrivalRate, m.DepartureRate))
	}

	// Review bottleneck
	if m.WIP["review"] > m.WIP["in-progress"]*2 && m.WIP["review"] > 2 {
		bottlenecks = append(bottlenecks, "REVIEW BOTTLENECK: Consider prioritizing code reviews")
	}

	// Testing bottleneck
	if m.WIP["testing"] > m.WIP["review"]*2 && m.WIP["testing"] > 2 {
		bottlenecks = append(bottlenecks, "TESTING BOTTLENECK: Consider prioritizing QA")
	}

	// Stale items
	staleCount := 0
	for _, issue := range m.AgingIssues {
		if issue.AgeDays > 14 {
			staleCount++
		}
	}
	if staleCount > 0 {
		bottlenecks = append(bottlenecks, fmt.Sprintf("STALE ITEMS: %d issues stuck >14 days", staleCount))
	}

	// Little's Law variance
	if math.Abs(m.LittlesLaw.Variance) > 50 {
		bottlenecks = append(bottlenecks, fmt.Sprintf("FLOW INSTABILITY: Actual WIP deviates %.0f%% from predicted", m.LittlesLaw.Variance))
	}

	return bottlenecks
}

func printKanbanMetrics(m KanbanMetrics) {
	reset := "\033[0m"
	bold := "\033[1m"
	cyan := "\033[36m"
	yellow := "\033[33m"
	red := "\033[31m"
	green := "\033[32m"
	dim := "\033[90m"

	fmt.Printf("\n%s%s══════════════════════════════════════════════════════════════%s\n", bold, cyan, reset)
	fmt.Printf("%s%s  KANBAN METRICS: %s%s\n", bold, cyan, m.Repo, reset)
	fmt.Printf("%s%s══════════════════════════════════════════════════════════════%s\n", bold, cyan, reset)
	fmt.Printf("%sGenerated: %s │ Period: %d days%s\n\n", dim, m.Generated.Format("2006-01-02 15:04 UTC"), m.Period, reset)

	// ═══ FLOW METRICS ═══
	fmt.Printf("%s%s┌─ FLOW METRICS ─────────────────────────────────────────────┐%s\n", bold, cyan, reset)

	fmt.Printf("│ %sLead Time%s (creation → done):\n", bold, reset)
	if m.LeadTime.Count > 0 {
		fmt.Printf("│   Average: %s%.1f days%s  Median: %.1f  P85: %.1f  (n=%d)\n",
			bold, m.LeadTime.Average, reset, m.LeadTime.Median, m.LeadTime.P85, m.LeadTime.Count)
	} else {
		fmt.Printf("│   %sNo completed issues in period%s\n", dim, reset)
	}

	fmt.Printf("│ %sCycle Time%s (in-progress → done):\n", bold, reset)
	if m.CycleTime.Count > 0 {
		fmt.Printf("│   Average: %s%.1f days%s  Median: %.1f  P85: %.1f\n",
			bold, m.CycleTime.Average, reset, m.CycleTime.Median, m.CycleTime.P85)
	} else {
		fmt.Printf("│   %sNo data%s\n", dim, reset)
	}

	fmt.Printf("│ %sThroughput%s:\n", bold, reset)
	fmt.Printf("│   %s%d items%s completed │ %.2f/day │ %.1f/week\n",
		bold, m.Throughput.Total, reset, m.Throughput.PerDay, m.Throughput.PerWeek)

	if m.CycleTime.Count > 0 {
		fmt.Printf("│ %sFlow Efficiency%s: %s%.0f%%%s\n", bold, reset, bold, m.FlowEfficiency, reset)
	} else {
		fmt.Printf("│ %sFlow Efficiency%s: %sN/A%s (need cycle time data)\n", bold, reset, dim, reset)
	}
	fmt.Printf("%s└────────────────────────────────────────────────────────────┘%s\n\n", cyan, reset)

	// ═══ WIP METRICS ═══
	fmt.Printf("%s%s┌─ WORK IN PROGRESS (WIP) ───────────────────────────────────┐%s\n", bold, yellow, reset)

	totalWIP := 0
	for _, status := range []string{"backlog", "ready", "in-progress", "review", "testing", "done"} {
		count := m.WIP[status]
		totalWIP += count

		limitStr := ""
		barColor := ""
		if limit, ok := m.WIPLimits["status: "+status]; ok {
			if count > limit {
				barColor = red
				limitStr = fmt.Sprintf(" %s⚠ OVER LIMIT (%d)%s", red, limit, reset)
			} else {
				limitStr = fmt.Sprintf(" %s(limit: %d)%s", dim, limit, reset)
			}
		}

		bar := strings.Repeat("█", minInt(count, 20))
		density := m.Density[status]
		fmt.Printf("│ %-12s %s%3d%s %s%-20s%s %5.1f%%%s\n",
			status, barColor+bold, count, reset, barColor, bar, reset, density, limitStr)
	}
	fmt.Printf("│ %s%-12s %3d%s (Flow Load)\n", bold, "TOTAL", totalWIP, reset)

	if m.WIPAge.Count > 0 {
		fmt.Printf("│\n│ %sWIP Age%s: avg %.1f days │ median %.1f │ max %.1f\n",
			bold, reset, m.WIPAge.Average, m.WIPAge.Median, m.WIPAge.Max)
	}
	fmt.Printf("%s└────────────────────────────────────────────────────────────┘%s\n\n", yellow, reset)

	// ═══ RATE METRICS ═══
	fmt.Printf("%s%s┌─ RATE METRICS ─────────────────────────────────────────────┐%s\n", bold, green, reset)
	fmt.Printf("│ %sArrival Rate%s:   %.2f items/day (new issues entering)\n", bold, reset, m.ArrivalRate)
	fmt.Printf("│ %sDeparture Rate%s: %.2f items/day (issues completed)\n", bold, reset, m.DepartureRate)

	// Balance indicator
	if m.ArrivalRate > 0 || m.DepartureRate > 0 {
		balance := m.DepartureRate - m.ArrivalRate
		if balance > 0.1 {
			fmt.Printf("│ %s→ System draining (good)%s\n", green, reset)
		} else if balance < -0.1 {
			fmt.Printf("│ %s→ System accumulating (watch WIP)%s\n", yellow, reset)
		} else {
			fmt.Printf("│ → System balanced\n")
		}
	}
	fmt.Printf("%s└────────────────────────────────────────────────────────────┘%s\n\n", green, reset)

	// ═══ LITTLE'S LAW ═══
	if m.LittlesLaw.CalculatedWIP > 0 {
		fmt.Printf("%s%s┌─ LITTLE'S LAW ─────────────────────────────────────────────┐%s\n", bold, cyan, reset)
		fmt.Printf("│ WIP = Throughput × Lead Time\n")
		fmt.Printf("│ Predicted WIP: %.1f │ Actual WIP: %d │ Variance: %s%.0f%%%s\n",
			m.LittlesLaw.CalculatedWIP, m.LittlesLaw.ActualWIP,
			getVarianceColor(m.LittlesLaw.Variance), m.LittlesLaw.Variance, reset)
		fmt.Printf("%s└────────────────────────────────────────────────────────────┘%s\n\n", cyan, reset)
	}

	// ═══ AGING ISSUES ═══
	if len(m.AgingIssues) > 0 {
		fmt.Printf("%s%s┌─ AGING ISSUES (oldest first) ─────────────────────────────┐%s\n", bold, yellow, reset)
		for _, issue := range m.AgingIssues {
			assignee := ""
			if issue.Assignee != "" {
				assignee = fmt.Sprintf(" @%s", issue.Assignee)
			}
			ageColor := getAgeColor(issue.AgeDays)
			blockedStr := formatBlockedTime(issue.BlockedHours, issue.IsBlocked)
			fmt.Printf("│ #%-4d %s%5.1fd%s %-11s %-25s%s%s%s\n",
				issue.Number, ageColor, issue.AgeDays, reset,
				issue.Status, issue.Title, blockedStr, dim, assignee+reset)
		}
		fmt.Printf("%s└────────────────────────────────────────────────────────────┘%s\n\n", yellow, reset)
	}

	// ═══ BOTTLENECKS ═══
	if len(m.Bottlenecks) > 0 {
		fmt.Printf("%s%s┌─ ⚠ BOTTLENECKS & WARNINGS ─────────────────────────────────┐%s\n", bold, red, reset)
		for _, b := range m.Bottlenecks {
			fmt.Printf("│ %s⚠%s %s\n", red, reset, b)
		}
		fmt.Printf("%s└────────────────────────────────────────────────────────────┘%s\n", red, reset)
	} else {
		fmt.Printf("%s%s✓ No bottlenecks detected - flow is healthy%s\n", bold, green, reset)
	}

	fmt.Println()
}

func getAgeColor(days float64) string {
	if days > 14 {
		return "\033[31m" // red
	} else if days > 7 {
		return "\033[33m" // yellow
	}
	return ""
}

func getVarianceColor(variance float64) string {
	if math.Abs(variance) > 50 {
		return "\033[31m" // red
	} else if math.Abs(variance) > 25 {
		return "\033[33m" // yellow
	}
	return "\033[32m" // green
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
