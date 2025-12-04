package cmd

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/kiracore/kanban/internal/config"
	"github.com/kiracore/kanban/internal/db"
	"github.com/kiracore/kanban/internal/github"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Synchronize labels and issues",
	Long: `Sync labels from configuration to GitHub repositories and
cache issues in the local database for board and metrics.

Creates missing labels, updates existing ones if different,
and optionally removes labels not in config (with --prune).`,
	RunE: runSync,
}

var (
	prune        bool
	labelsOnly   bool
	issuesOnly   bool
	fullSync     bool
	withTimeline bool
	withPRs      bool
)

func init() {
	rootCmd.AddCommand(syncCmd)
	syncCmd.Flags().StringVarP(&repo, "repo", "r", "", "specific repository")
	syncCmd.Flags().BoolVar(&allRepos, "all", false, "apply to all repositories")
	syncCmd.Flags().BoolVar(&prune, "prune", false, "remove labels not in config")
	syncCmd.Flags().BoolVar(&labelsOnly, "labels-only", false, "only sync labels, skip issues")
	syncCmd.Flags().BoolVar(&issuesOnly, "issues-only", false, "only sync issues, skip labels")
	syncCmd.Flags().BoolVar(&fullSync, "full", false, "full sync (ignore last sync time)")
	syncCmd.Flags().BoolVar(&withTimeline, "with-timeline", false, "fetch timeline for accurate cycle time (slower)")
	syncCmd.Flags().BoolVar(&withPRs, "with-prs", false, "also sync pull requests and link them to issues")
}

func runSync(cmd *cobra.Command, args []string) error {
	organization := viper.GetString("organization")
	if organization == "" && org != "" {
		organization = org
	}

	if organization == "" {
		return fmt.Errorf("organization required: use --org flag or set in config")
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Open database
	database, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer database.Close()

	// Initialize database if needed
	if err := database.Init(); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	labels := cfg.AllLabels()
	if len(labels) == 0 && !issuesOnly {
		return fmt.Errorf("no labels defined in config")
	}

	if !issuesOnly {
		fmt.Printf("Loaded %d labels from config\n", len(labels))
	}

	client := github.NewClient()

	// Determine target repos
	var repos []string
	if repo != "" {
		repos = []string{repo}
	} else if cfg.HasExplicitRepos() {
		// Use explicit repo list from config
		repos = cfg.GetRepos()
	} else if allRepos {
		// Fetch all and filter by patterns
		repos, err = client.ListRepos(organization)
		if err != nil {
			return err
		}
		repos = cfg.FilterRepos(repos)
	} else {
		return fmt.Errorf("specify --repo, --all, or define repositories.list in config")
	}

	if len(repos) == 0 {
		return fmt.Errorf("no repositories to sync")
	}

	fmt.Printf("Syncing %d repositories...\n", len(repos))

	if dryRun {
		fmt.Println("\n[DRY RUN - no changes will be made]")
	}

	// Get or create organization in DB
	dbOrg, err := database.GetOrCreateOrg(organization)
	if err != nil {
		return fmt.Errorf("failed to create organization in DB: %w", err)
	}

	// Sync repos (with concurrency limit)
	concurrency := viper.GetInt("settings.concurrency")
	if concurrency == 0 {
		concurrency = 5
	}

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var syncErrors []string
	var totalIssues int

	for _, r := range repos {
		wg.Add(1)
		go func(repoName string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			fullName := fmt.Sprintf("%s/%s", organization, repoName)
			fmt.Printf("\nSyncing %s...\n", fullName)

			// Get or create repo in DB
			dbRepo, err := database.GetOrCreateRepo(dbOrg.ID, repoName, fullName)
			if err != nil {
				mu.Lock()
				syncErrors = append(syncErrors, fmt.Sprintf("%s: %v", repoName, err))
				mu.Unlock()
				return
			}

			// Record sync start
			syncID, _ := database.RecordSyncStart(&dbRepo.ID, "full")

			var itemsSynced int
			var syncErr string

			// Sync labels to GitHub (only if needed)
			if !issuesOnly && !dryRun {
				// Check if labels need syncing by comparing with DB cache
				names := make([]string, len(labels))
				colors := make([]string, len(labels))
				descriptions := make([]string, len(labels))
				for i, l := range labels {
					names[i] = l.Name
					colors[i] = l.Color
					descriptions[i] = l.Description
				}

				needsSync, _ := database.LabelsNeedSync(dbRepo.ID, names, colors, descriptions)
				if needsSync {
					if err := client.SyncLabels(organization, repoName, labels, dryRun); err != nil {
						mu.Lock()
						syncErrors = append(syncErrors, fmt.Sprintf("%s labels: %v", repoName, err))
						mu.Unlock()
						fmt.Fprintf(os.Stderr, "  Labels error: %v\n", err)
					} else {
						fmt.Printf("  Labels synced\n")
					}

					// Sync labels to DB
					for _, l := range labels {
						dbLabel := &db.Label{
							RepoID:      dbRepo.ID,
							Name:        l.Name,
							Color:       l.Color,
							Description: l.Description,
						}
						database.UpsertLabel(dbLabel)
					}
				} else {
					fmt.Printf("  Labels up-to-date (skipped)\n")
				}
			}

			// Sync issues from GitHub to DB
			if !labelsOnly {
				issues, err := client.ListAllIssues(organization, repoName, 500)
				if err != nil {
					mu.Lock()
					syncErrors = append(syncErrors, fmt.Sprintf("%s issues: %v", repoName, err))
					mu.Unlock()
					fmt.Fprintf(os.Stderr, "  Issues error: %v\n", err)
					syncErr = err.Error()
				} else {
					for _, issue := range issues {
						if dryRun {
							continue
						}

						dbIssue := &db.Issue{
							RepoID:      dbRepo.ID,
							Number:      issue.Number,
							Title:       issue.Title,
							State:       strings.ToLower(issue.State),
							GHCreatedAt: issue.CreatedAt,
							GHUpdatedAt: issue.UpdatedAt,
							Assignee:    issue.Assignee,
						}

						if !issue.ClosedAt.IsZero() {
							dbIssue.GHClosedAt = &issue.ClosedAt
						}

						// Parse labels for status, priority, type, size
						for _, label := range issue.Labels {
							lower := strings.ToLower(label)
							if strings.HasPrefix(lower, "status:") {
								dbIssue.CurrentStatus = strings.TrimPrefix(lower, "status:")
								dbIssue.CurrentStatus = strings.TrimSpace(dbIssue.CurrentStatus)
							} else if strings.HasPrefix(lower, "priority:") {
								dbIssue.CurrentPriority = strings.TrimPrefix(lower, "priority:")
								dbIssue.CurrentPriority = strings.TrimSpace(dbIssue.CurrentPriority)
							} else if strings.HasPrefix(lower, "type:") {
								dbIssue.CurrentType = strings.TrimPrefix(lower, "type:")
								dbIssue.CurrentType = strings.TrimSpace(dbIssue.CurrentType)
							} else if strings.HasPrefix(lower, "size:") {
								dbIssue.CurrentSize = strings.TrimPrefix(lower, "size:")
								dbIssue.CurrentSize = strings.TrimSpace(dbIssue.CurrentSize)
							} else if label == "blocked" {
								dbIssue.IsBlocked = true
							}
						}

						// Calculate lead time for closed issues
						if dbIssue.GHClosedAt != nil {
							dbIssue.LeadTimeHours = dbIssue.GHClosedAt.Sub(dbIssue.GHCreatedAt).Hours()
							// Treat closed as "done" for status if no done label
							if dbIssue.CurrentStatus == "" {
								dbIssue.CurrentStatus = "done"
							}
						}

						if err := database.UpsertIssue(dbIssue); err != nil {
							fmt.Fprintf(os.Stderr, "  Warning: failed to save issue #%d: %v\n", issue.Number, err)
							continue
						}

						// Recalc cycle time for closed issues (uses closed_at as done time)
						if dbIssue.GHClosedAt != nil {
							database.RecalcCycleTime(dbIssue.ID)
						}

						// Fetch timeline for accurate timestamps if requested
						if withTimeline && dbIssue.CurrentStatus != "" {
							timeline, err := client.GetIssueTimeline(organization, repoName, issue.Number)
							if err == nil && timeline != nil {
								// Update status timestamps
								var ready, progress, review, testing, done *time.Time
								if t, ok := timeline.StatusChanges["ready"]; ok {
									ready = &t
								}
								if t, ok := timeline.StatusChanges["in-progress"]; ok {
									progress = &t
								}
								if t, ok := timeline.StatusChanges["review"]; ok {
									review = &t
								}
								if t, ok := timeline.StatusChanges["testing"]; ok {
									testing = &t
								}
								if t, ok := timeline.StatusChanges["done"]; ok {
									done = &t
								}
								database.UpdateIssueTimestamps(dbIssue.ID, ready, progress, review, testing, done)

								// Record blocked periods
								for _, bp := range timeline.BlockedPeriods {
									start := bp.Start
									var end *time.Time
									if !bp.End.IsZero() {
										end = &bp.End
									}
									database.RecordBlockedPeriod(dbIssue.ID, &start, end, "")
								}

								// Update blocked time and recalc cycle time
								if timeline.TotalBlocked > 0 {
									database.UpdateIssueBlockedTime(dbIssue.ID, timeline.TotalBlocked)
								}
								database.RecalcCycleTime(dbIssue.ID)
							}
						}
						itemsSynced++
					}

					mu.Lock()
					totalIssues += len(issues)
					mu.Unlock()

					if withTimeline {
						fmt.Printf("  %d issues synced (with timeline)\n", len(issues))
					} else {
						fmt.Printf("  %d issues synced\n", len(issues))
					}
				}
			}

			// Sync PRs if requested
			if withPRs && !labelsOnly {
				prs, err := client.ListPRs(organization, repoName, 200)
				if err != nil {
					mu.Lock()
					syncErrors = append(syncErrors, fmt.Sprintf("%s PRs: %v", repoName, err))
					mu.Unlock()
					fmt.Fprintf(os.Stderr, "  PRs error: %v\n", err)
				} else {
					prCount := 0
					for _, pr := range prs {
						if dryRun {
							continue
						}

						dbPR := &db.PullRequest{
							RepoID:       dbRepo.ID,
							Number:       pr.Number,
							Title:        pr.Title,
							State:        pr.State,
							IsDraft:      pr.IsDraft,
							GHCreatedAt:  pr.CreatedAt,
							GHUpdatedAt:  pr.UpdatedAt,
							Author:       pr.Author,
							Additions:    pr.Additions,
							Deletions:    pr.Deletions,
							ChangedFiles: pr.ChangedFiles,
						}

						if !pr.MergedAt.IsZero() {
							dbPR.GHMergedAt = &pr.MergedAt
						}
						if !pr.ClosedAt.IsZero() {
							dbPR.GHClosedAt = &pr.ClosedAt
						}

						if err := database.UpsertPR(dbPR); err != nil {
							fmt.Fprintf(os.Stderr, "  Warning: failed to save PR #%d: %v\n", pr.Number, err)
							continue
						}

						// Get linked issues and create links
						linkedIssues, err := client.GetPRLinkedIssues(organization, repoName, pr.Number)
						if err == nil {
							for _, issueNum := range linkedIssues {
								issueID, err := database.GetIssueIDByNumber(dbRepo.ID, issueNum)
								if err == nil && issueID > 0 {
									database.LinkPRToIssue(dbPR.ID, issueID)
								}
							}
						}

						prCount++
					}
					fmt.Printf("  %d PRs synced\n", prCount)
				}
			}

			// Record sync completion
			if !dryRun {
				database.RecordSyncComplete(syncID, itemsSynced, syncErr)
				database.UpdateRepoSyncTime(dbRepo.ID)

				// Auto CFD snapshot if >24h since last
				today := time.Now().Truncate(24 * time.Hour)
				lastSnapshot, _ := database.GetLastCFDSnapshot(dbRepo.ID)
				if lastSnapshot == nil || !lastSnapshot.Truncate(24*time.Hour).Equal(today) {
					counts, err := database.GetStatusCounts(dbRepo.ID)
					if err == nil && len(counts) > 0 {
						database.SaveCFDSnapshot(dbRepo.ID, today, counts)
					}
				}
			}
		}(r)
	}

	wg.Wait()

	if len(syncErrors) > 0 {
		fmt.Fprintf(os.Stderr, "\nCompleted with %d errors:\n", len(syncErrors))
		for _, e := range syncErrors {
			fmt.Fprintf(os.Stderr, "  - %s\n", e)
		}
		return fmt.Errorf("sync completed with errors")
	}

	fmt.Printf("\nSync completed! %d issues cached.\n", totalIssues)
	return nil
}

// extractLabelValue extracts the value from a prefixed label
func extractLabelValue(labels []string, prefix string) string {
	for _, label := range labels {
		lower := strings.ToLower(label)
		if strings.HasPrefix(lower, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(lower, prefix))
		}
	}
	return ""
}

// hasLabel checks if a label exists in the list
func hasLabel(labels []string, target string) bool {
	for _, label := range labels {
		if strings.EqualFold(label, target) {
			return true
		}
	}
	return false
}

// Unused import prevention
var _ = time.Now
