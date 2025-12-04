package cmd

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/kiracore/kanban/internal/config"
	"github.com/kiracore/kanban/internal/db"
	"github.com/kiracore/kanban/internal/github"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfdCmd = &cobra.Command{
	Use:   "cfd",
	Short: "Cumulative Flow Diagram data",
	Long:  `Manage CFD snapshots for tracking flow over time.`,
}

var cfdSnapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "Take a CFD snapshot",
	Long:  `Count current issues per status and save to database.`,
	RunE:  runCFDSnapshot,
}

var cfdShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Display CFD data",
	Long:  `Show cumulative flow data as ASCII chart.`,
	RunE:  runCFDShow,
}

var cfdExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export CFD data",
	Long:  `Export CFD data to CSV or JSON.`,
	RunE:  runCFDExport,
}

var cfdDays int

func init() {
	rootCmd.AddCommand(cfdCmd)
	cfdCmd.AddCommand(cfdSnapshotCmd)
	cfdCmd.AddCommand(cfdShowCmd)
	cfdCmd.AddCommand(cfdExportCmd)

	cfdSnapshotCmd.Flags().StringVarP(&repo, "repo", "r", "", "repository")
	cfdSnapshotCmd.Flags().BoolVar(&allRepos, "all", false, "all repositories")

	cfdShowCmd.Flags().StringVarP(&repo, "repo", "r", "", "repository")
	cfdShowCmd.Flags().IntVar(&cfdDays, "days", 30, "days of history")

	cfdExportCmd.Flags().StringVarP(&repo, "repo", "r", "", "repository")
	cfdExportCmd.Flags().IntVar(&cfdDays, "days", 30, "days of history")
	cfdExportCmd.Flags().StringVar(&format, "format", "csv", "output format (csv, json)")
}

func runCFDSnapshot(cmd *cobra.Command, args []string) error {
	organization := viper.GetString("organization")
	if organization == "" && org != "" {
		organization = org
	}
	if organization == "" {
		return fmt.Errorf("organization required")
	}

	database, err := db.Open(dbPath)
	if err != nil {
		return err
	}
	defer database.Close()

	cfg, _ := config.Load()
	client := github.NewClient()

	var repos []string
	if repo != "" {
		repos = []string{repo}
	} else if cfg != nil && cfg.HasExplicitRepos() {
		repos = cfg.GetRepos()
	} else if allRepos {
		repos, err = client.ListRepos(organization)
		if err != nil {
			return err
		}
	} else {
		return fmt.Errorf("specify --repo, --all, or define repositories.list in config")
	}

	today := time.Now().Truncate(24 * time.Hour)

	for _, r := range repos {
		fullName := fmt.Sprintf("%s/%s", organization, r)

		dbOrg, err := database.GetOrCreateOrg(organization)
		if err != nil {
			continue
		}
		dbRepo, err := database.GetOrCreateRepo(dbOrg.ID, r, fullName)
		if err != nil {
			continue
		}

		// Check if already snapshotted today
		lastSnapshot, _ := database.GetLastCFDSnapshot(dbRepo.ID)
		if lastSnapshot != nil && lastSnapshot.Truncate(24*time.Hour).Equal(today) {
			fmt.Printf("%s: already snapshotted today\n", fullName)
			continue
		}

		counts, err := database.GetStatusCounts(dbRepo.ID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", fullName, err)
			continue
		}

		if err := database.SaveCFDSnapshot(dbRepo.ID, today, counts); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", fullName, err)
			continue
		}

		fmt.Printf("%s: snapshot saved (%d statuses)\n", fullName, len(counts))
	}

	return nil
}

func runCFDShow(cmd *cobra.Command, args []string) error {
	organization := viper.GetString("organization")
	if organization == "" && org != "" {
		organization = org
	}
	if organization == "" {
		return fmt.Errorf("organization required")
	}
	if repo == "" {
		return fmt.Errorf("--repo required")
	}

	database, err := db.Open(dbPath)
	if err != nil {
		return err
	}
	defer database.Close()

	fullName := fmt.Sprintf("%s/%s", organization, repo)
	dbOrg, err := database.GetOrCreateOrg(organization)
	if err != nil {
		return err
	}
	dbRepo, err := database.GetOrCreateRepo(dbOrg.ID, repo, fullName)
	if err != nil {
		return err
	}

	data, err := database.GetCFDData(dbRepo.ID, cfdDays)
	if err != nil {
		return err
	}

	if len(data) == 0 {
		fmt.Println("No CFD data. Run 'kanban cfd snapshot' first.")
		return nil
	}

	// Group by date
	byDate := make(map[string]map[string]int)
	statuses := make(map[string]bool)
	var dates []string

	for _, d := range data {
		if byDate[d.Date] == nil {
			byDate[d.Date] = make(map[string]int)
			dates = append(dates, d.Date)
		}
		byDate[d.Date][d.Status] = d.Count
		statuses[d.Status] = true
	}

	sort.Strings(dates)

	// Get ordered status list
	statusOrder := []string{"backlog", "ready", "in-progress", "review", "testing", "done", "none"}
	var orderedStatuses []string
	for _, s := range statusOrder {
		if statuses[s] {
			orderedStatuses = append(orderedStatuses, s)
		}
	}

	// Print header
	fmt.Printf("\n%s - Cumulative Flow (%d days)\n", fullName, cfdDays)
	fmt.Println(strings.Repeat("─", 60))

	// Simple ASCII chart
	maxTotal := 0
	for _, counts := range byDate {
		total := 0
		for _, c := range counts {
			total += c
		}
		if total > maxTotal {
			maxTotal = total
		}
	}

	chartWidth := 40
	for _, date := range dates {
		counts := byDate[date]
		total := 0
		for _, c := range counts {
			total += c
		}

		// Scale to chart width
		bar := ""
		for _, status := range orderedStatuses {
			count := counts[status]
			if count == 0 {
				continue
			}
			width := count * chartWidth / maxTotal
			if width == 0 && count > 0 {
				width = 1
			}
			char := getStatusChar(status)
			bar += strings.Repeat(char, width)
		}

		fmt.Printf("%s │%s│ %d\n", date[5:], bar, total)
	}

	// Legend
	fmt.Println(strings.Repeat("─", 60))
	fmt.Print("Legend: ")
	for _, s := range orderedStatuses {
		fmt.Printf("%s=%s ", getStatusChar(s), s)
	}
	fmt.Println()

	return nil
}

func getStatusChar(status string) string {
	switch status {
	case "backlog":
		return "░"
	case "ready":
		return "▒"
	case "in-progress":
		return "▓"
	case "review":
		return "█"
	case "testing":
		return "▄"
	case "done":
		return "●"
	default:
		return "·"
	}
}

func runCFDExport(cmd *cobra.Command, args []string) error {
	organization := viper.GetString("organization")
	if organization == "" && org != "" {
		organization = org
	}
	if organization == "" {
		return fmt.Errorf("organization required")
	}
	if repo == "" {
		return fmt.Errorf("--repo required")
	}

	database, err := db.Open(dbPath)
	if err != nil {
		return err
	}
	defer database.Close()

	fullName := fmt.Sprintf("%s/%s", organization, repo)
	dbOrg, err := database.GetOrCreateOrg(organization)
	if err != nil {
		return err
	}
	dbRepo, err := database.GetOrCreateRepo(dbOrg.ID, repo, fullName)
	if err != nil {
		return err
	}

	data, err := database.GetCFDData(dbRepo.ID, cfdDays)
	if err != nil {
		return err
	}

	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(data)
	}

	// CSV output
	w := csv.NewWriter(os.Stdout)
	w.Write([]string{"date", "status", "count"})
	for _, d := range data {
		w.Write([]string{d.Date, d.Status, fmt.Sprintf("%d", d.Count)})
	}
	w.Flush()
	return nil
}
