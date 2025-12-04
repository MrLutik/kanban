package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/kiracore/kanban/internal/config"
	"github.com/kiracore/kanban/internal/github"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

var (
	repo             string
	allRepos         bool
	labelsFormat     string
	labelsOutputFile string
)

var labelsCmd = &cobra.Command{
	Use:   "labels",
	Short: "Manage labels across repositories",
	Long:  `List, export, import, and manage labels across GitHub repositories.`,
}

var labelsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List labels in repositories",
	RunE:  runLabelsList,
}

var labelsExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export labels from a repository",
	RunE:  runLabelsExport,
}

var labelsImportCmd = &cobra.Command{
	Use:   "import <file>",
	Short: "Import labels to repositories",
	Args:  cobra.ExactArgs(1),
	RunE:  runLabelsImport,
}

func init() {
	rootCmd.AddCommand(labelsCmd)
	labelsCmd.AddCommand(labelsListCmd)
	labelsCmd.AddCommand(labelsExportCmd)
	labelsCmd.AddCommand(labelsImportCmd)

	// Flags for labels commands
	labelsCmd.PersistentFlags().StringVarP(&repo, "repo", "r", "", "specific repository")
	labelsCmd.PersistentFlags().BoolVar(&allRepos, "all", false, "apply to all repositories")

	// Export specific flags
	labelsExportCmd.Flags().StringVarP(&labelsFormat, "format", "f", "yaml", "output format (yaml|json)")
	labelsExportCmd.Flags().StringVar(&labelsOutputFile, "output", "", "output file (default stdout)")
}

func runLabelsList(cmd *cobra.Command, args []string) error {
	organization := viper.GetString("organization")
	if organization == "" && org != "" {
		organization = org
	}

	if organization == "" {
		return fmt.Errorf("organization required: use --org flag or set in config")
	}

	client := github.NewClient()

	if repo != "" {
		// List labels for specific repo
		labels, err := client.ListLabels(organization, repo)
		if err != nil {
			return err
		}
		printLabels(organization, repo, labels)
	} else if allRepos {
		// List labels for all repos
		repos, err := client.ListRepos(organization)
		if err != nil {
			return err
		}
		for _, r := range repos {
			labels, err := client.ListLabels(organization, r)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to list labels for %s: %v\n", r, err)
				continue
			}
			printLabels(organization, r, labels)
		}
	} else {
		return fmt.Errorf("specify --repo or --all")
	}

	return nil
}

func runLabelsExport(cmd *cobra.Command, args []string) error {
	organization := viper.GetString("organization")
	if organization == "" && org != "" {
		organization = org
	}

	if organization == "" {
		return fmt.Errorf("organization required")
	}

	if repo == "" {
		return fmt.Errorf("repository required: use --repo flag")
	}

	client := github.NewClient()
	labels, err := client.ListLabels(organization, repo)
	if err != nil {
		return err
	}

	// Convert to config format
	cfg := config.LabelConfig{
		Version: "1",
		Labels:  make(map[string][]config.Label),
	}
	cfg.Labels["exported"] = labels

	var output []byte
	switch labelsFormat {
	case "json":
		output, err = json.MarshalIndent(cfg, "", "  ")
	case "yaml":
		output, err = yaml.Marshal(cfg)
	default:
		return fmt.Errorf("unsupported format: %s", labelsFormat)
	}
	if err != nil {
		return err
	}

	if labelsOutputFile != "" {
		return os.WriteFile(labelsOutputFile, output, 0644)
	}
	fmt.Println(string(output))
	return nil
}

func runLabelsImport(cmd *cobra.Command, args []string) error {
	organization := viper.GetString("organization")
	if organization == "" && org != "" {
		organization = org
	}

	if organization == "" {
		return fmt.Errorf("organization required")
	}

	// Load labels from file
	cfg, err := config.LoadLabelsFromFile(args[0])
	if err != nil {
		return err
	}

	client := github.NewClient()
	labels := cfg.AllLabels()

	if dryRun {
		fmt.Println("Dry run - would import the following labels:")
		for _, l := range labels {
			fmt.Printf("  - %s (#%s): %s\n", l.Name, l.Color, l.Description)
		}
		return nil
	}

	var repos []string
	if repo != "" {
		repos = []string{repo}
	} else if allRepos {
		repos, err = client.ListRepos(organization)
		if err != nil {
			return err
		}
	} else {
		return fmt.Errorf("specify --repo or --all")
	}

	for _, r := range repos {
		fmt.Printf("Importing labels to %s/%s...\n", organization, r)
		if err := client.SyncLabels(organization, r, labels, dryRun); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to sync labels for %s: %v\n", r, err)
		}
	}

	return nil
}

func printLabels(org, repo string, labels []config.Label) {
	fmt.Printf("\n%s/%s (%d labels):\n", org, repo, len(labels))
	for _, l := range labels {
		fmt.Printf("  - %-30s #%s  %s\n", l.Name, l.Color, l.Description)
	}
}
