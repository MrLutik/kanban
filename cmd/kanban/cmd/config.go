package cmd

import (
	"fmt"
	"os"

	"github.com/kiracore/kanban/internal/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
	Long:  `Commands for managing kanban configuration files.`,
}

var validateCmd = &cobra.Command{
	Use:   "validate [file]",
	Short: "Validate configuration file",
	Long: `Validate the configuration file for errors and warnings.

Examples:
  kanban config validate
  kanban config validate .kanban.yaml
  kanban config validate --config myconfig.yaml`,
	RunE: runValidate,
}

var showCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	Long:  `Display the current configuration that would be used.`,
	RunE:  runShowConfig,
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(validateCmd)
	configCmd.AddCommand(showCmd)
}

func runValidate(cmd *cobra.Command, args []string) error {
	// Determine config file path
	configFile := cfgFile
	if len(args) > 0 {
		configFile = args[0]
	}
	if configFile == "" {
		configFile = ".kanban.yaml"
	}

	// Load config
	cfg, err := config.LoadLabelsFromFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	fmt.Printf("Validating: %s\n\n", configFile)

	// Validate
	result := cfg.Validate()

	// Print errors
	if len(result.Errors) > 0 {
		fmt.Printf("\033[31m✗ %d error(s):\033[0m\n", len(result.Errors))
		for _, e := range result.Errors {
			fmt.Printf("  \033[31m• %s\033[0m\n", e.Error())
		}
		fmt.Println()
	}

	// Print warnings
	if len(result.Warnings) > 0 {
		fmt.Printf("\033[33m⚠ %d warning(s):\033[0m\n", len(result.Warnings))
		for _, w := range result.Warnings {
			fmt.Printf("  \033[33m• %s\033[0m\n", w.Error())
		}
		fmt.Println()
	}

	// Summary
	labels := cfg.AllLabels()
	fmt.Printf("Configuration summary:\n")
	fmt.Printf("  Organization: %s\n", cfg.Organization)
	fmt.Printf("  Labels: %d\n", len(labels))
	fmt.Printf("  Repositories: %d explicit, %d include patterns, %d exclude patterns\n",
		len(cfg.Repositories.List),
		len(cfg.Repositories.Include),
		len(cfg.Repositories.Exclude))
	fmt.Printf("  Migrations: %d\n", len(cfg.Migrations))
	fmt.Printf("  Maintainers: %d\n", len(cfg.Maintainers))
	fmt.Println()

	if result.IsValid() {
		fmt.Printf("\033[32m✓ Configuration is valid\033[0m\n")
		return nil
	}

	fmt.Printf("\033[31m✗ Configuration has errors\033[0m\n")
	os.Exit(1)
	return nil
}

func runShowConfig(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	fmt.Printf("Organization: %s\n", cfg.Organization)
	fmt.Printf("Version: %s\n", cfg.Version)
	fmt.Println()

	fmt.Println("Repositories:")
	if len(cfg.Repositories.List) > 0 {
		fmt.Printf("  Explicit list: %v\n", cfg.Repositories.List)
	}
	if len(cfg.Repositories.Include) > 0 {
		fmt.Printf("  Include: %v\n", cfg.Repositories.Include)
	}
	if len(cfg.Repositories.Exclude) > 0 {
		fmt.Printf("  Exclude: %v\n", cfg.Repositories.Exclude)
	}
	fmt.Println()

	fmt.Println("Labels:")
	for category, labels := range cfg.Labels {
		fmt.Printf("  %s: %d labels\n", category, len(labels))
	}
	fmt.Println()

	if len(cfg.Maintainers) > 0 {
		fmt.Printf("Maintainers: %v\n", cfg.Maintainers)
		fmt.Println()
	}

	fmt.Println("Settings:")
	fmt.Printf("  Preserve unknown: %v\n", cfg.Settings.PreserveUnknown)
	fmt.Printf("  Concurrency: %d\n", cfg.Settings.Concurrency)
	if len(cfg.Settings.WIPLimits) > 0 {
		fmt.Printf("  WIP Limits: %v\n", cfg.Settings.WIPLimits)
	}

	return nil
}
