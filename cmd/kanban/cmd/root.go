package cmd

import (
	"fmt"
	"os"

	"github.com/kiracore/kanban/internal/paths"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// Version info (set by ldflags)
	Version   = "dev"
	GitCommit = "none"
	BuildDate = "unknown"

	// Global flags
	cfgFile string
	org     string
	dryRun  bool
	verbose bool

	// Shared command flags
	format string
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "kanban",
	Short: "GitHub Kanban workflow CLI tool",
	Long: `Kanban is a CLI tool for applying Kanban methodology to GitHub organizations.

It manages labels, workflows, and issue tracking across multiple repositories
with a single configuration file.

Example:
  kanban init --org myorg
  kanban sync --all
  kanban audit`,
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default .kanban.yaml)")
	rootCmd.PersistentFlags().StringVarP(&org, "org", "o", "", "GitHub organization")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "show what would happen without making changes")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	// Bind flags to viper
	viper.BindPFlag("organization", rootCmd.PersistentFlags().Lookup("org"))
	viper.BindPFlag("dry_run", rootCmd.PersistentFlags().Lookup("dry-run"))
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
}

// initConfig reads in config file
func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		// Search order:
		// 1. Current directory (.kanban.yaml) - project-specific config
		// 2. XDG config dir (config.yaml) - user default config
		viper.AddConfigPath(".")
		viper.AddConfigPath(paths.ConfigDir())
		viper.SetConfigType("yaml")
		viper.SetConfigName(".kanban") // matches .kanban.yaml in current dir
	}

	// Read environment variables
	viper.SetEnvPrefix("KANBAN")
	viper.AutomaticEnv()

	// Try to read config file
	if err := viper.ReadInConfig(); err == nil {
		if verbose {
			fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
		}
	} else {
		// If .kanban.yaml not found, try config.yaml in XDG dir
		viper.SetConfigName("config")
		if err := viper.ReadInConfig(); err == nil {
			if verbose {
				fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
			}
		}
	}
}
