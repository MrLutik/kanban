package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/kiracore/kanban/internal/config"
	"github.com/kiracore/kanban/internal/github"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Check label consistency across repositories",
	Long: `Audit repositories for label consistency.

Reports missing, extra, and different labels compared to the config.`,
	RunE: runAudit,
}

func init() {
	rootCmd.AddCommand(auditCmd)
	auditCmd.Flags().StringVarP(&repo, "repo", "r", "", "specific repository")
	auditCmd.Flags().BoolVar(&allRepos, "all", false, "audit all repositories")
	auditCmd.Flags().StringVarP(&format, "format", "f", "table", "output format (table|json)")
}

type AuditResult struct {
	Repo     string   `json:"repo"`
	Missing  []string `json:"missing"`
	Extra    []string `json:"extra"`
	Modified []string `json:"modified"`
}

func runAudit(cmd *cobra.Command, args []string) error {
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

	expectedLabels := cfg.AllLabels()
	expectedMap := make(map[string]config.Label)
	for _, l := range expectedLabels {
		expectedMap[l.Name] = l
	}

	client := github.NewClient()

	// Determine target repos
	var repos []string
	if repo != "" {
		repos = []string{repo}
	} else if allRepos {
		repos, err = client.ListRepos(organization)
		if err != nil {
			return err
		}
		repos = cfg.FilterRepos(repos)
	} else {
		return fmt.Errorf("specify --repo or --all")
	}

	var results []AuditResult

	for _, r := range repos {
		current, err := client.ListLabels(organization, r)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to audit %s: %v\n", r, err)
			continue
		}

		currentMap := make(map[string]config.Label)
		for _, l := range current {
			currentMap[l.Name] = l
		}

		result := AuditResult{Repo: r}

		// Find missing and modified
		for name, expected := range expectedMap {
			if actual, exists := currentMap[name]; !exists {
				result.Missing = append(result.Missing, name)
			} else if actual.Color != expected.Color || actual.Description != expected.Description {
				result.Modified = append(result.Modified, name)
			}
		}

		// Find extra (only if preserve_unknown is false)
		if !viper.GetBool("settings.preserve_unknown") {
			for name := range currentMap {
				if _, exists := expectedMap[name]; !exists {
					result.Extra = append(result.Extra, name)
				}
			}
		}

		results = append(results, result)
	}

	// Output results
	switch format {
	case "json":
		output, _ := json.MarshalIndent(results, "", "  ")
		fmt.Println(string(output))
	default:
		printAuditTable(results)
	}

	return nil
}

func printAuditTable(results []AuditResult) {
	for _, r := range results {
		fmt.Printf("\n%s:\n", r.Repo)

		if len(r.Missing) == 0 && len(r.Extra) == 0 && len(r.Modified) == 0 {
			fmt.Println("  ✓ All labels match config")
			continue
		}

		if len(r.Missing) > 0 {
			fmt.Println("  Missing labels:")
			for _, l := range r.Missing {
				fmt.Printf("    - %s\n", l)
			}
		}

		if len(r.Modified) > 0 {
			fmt.Println("  Modified labels (color/description differs):")
			for _, l := range r.Modified {
				fmt.Printf("    ~ %s\n", l)
			}
		}

		if len(r.Extra) > 0 {
			fmt.Println("  Extra labels (not in config):")
			for _, l := range r.Extra {
				fmt.Printf("    + %s\n", l)
			}
		}
	}
}
