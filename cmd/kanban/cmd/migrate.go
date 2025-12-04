package cmd

import (
	"fmt"
	"os"

	"github.com/kiracore/kanban/internal/config"
	"github.com/kiracore/kanban/internal/github"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	fromLabel string
	toLabel   string
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate issues between label schemes",
	Long: `Migrate issues from old labels to new labels.

Can use mappings from config file or specify a single migration via flags.

Examples:
  # Migrate using mappings from config
  kanban migrate --repo myrepo --config .kanban.yaml

  # Migrate a single label
  kanban migrate --from "bug" --to "type: bug" --repo myrepo

  # Migrate across all repos
  kanban migrate --all --config .kanban.yaml --dry-run`,
	RunE: runMigrate,
}

func init() {
	rootCmd.AddCommand(migrateCmd)
	migrateCmd.Flags().StringVarP(&repo, "repo", "r", "", "specific repository")
	migrateCmd.Flags().BoolVar(&allRepos, "all", false, "apply to all repositories")
	migrateCmd.Flags().StringVar(&fromLabel, "from", "", "source label name")
	migrateCmd.Flags().StringVar(&toLabel, "to", "", "target label name")
}

func runMigrate(cmd *cobra.Command, args []string) error {
	organization := viper.GetString("organization")
	if organization == "" && org != "" {
		organization = org
	}

	if organization == "" {
		return fmt.Errorf("organization required: use --org flag or set in config")
	}

	// Determine migrations to perform
	var migrations []config.Migration

	if fromLabel != "" && toLabel != "" {
		// Single migration from flags
		migrations = append(migrations, config.Migration{
			From: fromLabel,
			To:   toLabel,
		})
	} else {
		// Load migrations from config
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		migrations = cfg.Migrations
	}

	if len(migrations) == 0 {
		return fmt.Errorf("no migrations defined: use --from/--to flags or define migrations in config")
	}

	fmt.Printf("Loaded %d migration(s)\n", len(migrations))
	for _, m := range migrations {
		fmt.Printf("  %s -> %s\n", m.From, m.To)
	}

	client := github.NewClient()

	// Determine target repos
	var repos []string
	var err error
	if repo != "" {
		repos = []string{repo}
	} else if allRepos {
		repos, err = client.ListRepos(organization)
		if err != nil {
			return err
		}
		// Filter repos based on config
		cfg, _ := config.Load()
		if cfg != nil {
			repos = cfg.FilterRepos(repos)
		}
	} else {
		return fmt.Errorf("specify --repo or --all")
	}

	if len(repos) == 0 {
		return fmt.Errorf("no repositories to migrate")
	}

	fmt.Printf("\nMigrating labels in %d repositories...\n", len(repos))

	if dryRun {
		fmt.Println("\n[DRY RUN - no changes will be made]")
	}

	var totalMigrated int
	var errors []string

	for _, r := range repos {
		fmt.Printf("\n%s/%s:\n", organization, r)

		for _, m := range migrations {
			count, err := client.MigrateIssueLabels(organization, r, m.From, m.To, dryRun)
			if err != nil {
				errors = append(errors, fmt.Sprintf("%s: %s->%s: %v", r, m.From, m.To, err))
				fmt.Printf("  Error migrating %s -> %s: %v\n", m.From, m.To, err)
				continue
			}
			if count > 0 {
				fmt.Printf("  %s -> %s: %d issue(s)\n", m.From, m.To, count)
				totalMigrated += count
			}
		}
	}

	fmt.Printf("\nMigration complete: %d issue(s) updated\n", totalMigrated)

	if len(errors) > 0 {
		fmt.Fprintf(os.Stderr, "\nCompleted with %d error(s):\n", len(errors))
		for _, e := range errors {
			fmt.Fprintf(os.Stderr, "  - %s\n", e)
		}
		return fmt.Errorf("migration completed with errors")
	}

	return nil
}
