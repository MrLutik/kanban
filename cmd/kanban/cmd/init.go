package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var preset string

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize kanban configuration",
	Long: `Initialize a new .kanban.yaml configuration file.

Available presets:
  minimal  - Basic status and priority labels only
  standard - Full kanban workflow with all label types (default)
  full     - Everything including size estimation labels`,
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().StringVar(&preset, "preset", "standard", "configuration preset (minimal|standard|full)")
}

func runInit(cmd *cobra.Command, args []string) error {
	configFile := ".kanban.yaml"

	// Check if config already exists
	if _, err := os.Stat(configFile); err == nil {
		return fmt.Errorf("config file %s already exists", configFile)
	}

	// Get organization from flag
	organization := org
	if organization == "" {
		organization = "your-organization"
	}

	// Generate config based on preset
	config := generateConfig(preset, organization)

	// Write config file
	if err := os.WriteFile(configFile, []byte(config), 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	fmt.Printf("Created %s with %s preset\n", configFile, preset)
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Edit .kanban.yaml to customize labels")
	fmt.Println("  2. Run: kanban audit --all --org", organization)
	fmt.Println("  3. Run: kanban sync --all --dry-run")
	fmt.Println("  4. Run: kanban sync --all")

	return nil
}

func generateConfig(preset, org string) string {
	switch preset {
	case "minimal":
		return generateMinimalConfig(org)
	case "full":
		return generateFullConfig(org)
	default:
		return generateStandardConfig(org)
	}
}

func generateMinimalConfig(org string) string {
	return fmt.Sprintf(`# Kanban CLI Configuration (minimal preset)
version: "1"

organization: "%s"

repositories:
  include: ["*"]
  exclude: []

labels:
  status:
    - name: "status: todo"
      color: "d4d4d4"
      description: "To be done"
    - name: "status: doing"
      color: "fbca04"
      description: "In progress"
    - name: "status: done"
      color: "0e8a16"
      description: "Completed"

  priority:
    - name: "priority: high"
      color: "b60205"
      description: "High priority"
    - name: "priority: low"
      color: "0e8a16"
      description: "Low priority"

settings:
  preserve_unknown: true
  concurrency: 5
`, org)
}

func generateStandardConfig(org string) string {
	return fmt.Sprintf(`# Kanban CLI Configuration (standard preset)
version: "1"

organization: "%s"

repositories:
  include: ["*"]
  exclude:
    - "*.github.io"
    - ".github"

labels:
  status:
    - name: "status: backlog"
      color: "d4d4d4"
      description: "Prioritized but not started"
    - name: "status: ready"
      color: "0075ca"
      description: "Ready to be worked on"
    - name: "status: in-progress"
      color: "fbca04"
      description: "Actively being worked on"
    - name: "status: review"
      color: "d93f0b"
      description: "Waiting for code review"
    - name: "status: done"
      color: "0e8a16"
      description: "Completed and merged"

  priority:
    - name: "priority: critical"
      color: "b60205"
      description: "Drop everything"
    - name: "priority: high"
      color: "d93f0b"
      description: "Next up"
    - name: "priority: medium"
      color: "fbca04"
      description: "Normal priority"
    - name: "priority: low"
      color: "0e8a16"
      description: "When time permits"

  type:
    - name: "type: bug"
      color: "d73a4a"
      description: "Something is broken"
    - name: "type: feature"
      color: "a2eeef"
      description: "New functionality"
    - name: "type: docs"
      color: "0075ca"
      description: "Documentation work"
    - name: "type: chore"
      color: "fef2c0"
      description: "Maintenance tasks"

  special:
    - name: "blocked"
      color: "000000"
      description: "Blocked by dependency"
    - name: "good-first-issue"
      color: "7057ff"
      description: "Good for newcomers"

settings:
  preserve_unknown: true
  concurrency: 5
  wip_limits:
    "status: ready": 10
    "status: in-progress": 2
`, org)
}

func generateFullConfig(org string) string {
	return fmt.Sprintf(`# Kanban CLI Configuration (full preset)
version: "1"

organization: "%s"

repositories:
  include: ["*"]
  exclude:
    - "*.github.io"
    - ".github"

labels:
  status:
    - name: "status: backlog"
      color: "d4d4d4"
      description: "Prioritized but not started"
    - name: "status: ready"
      color: "0075ca"
      description: "Ready to be worked on"
    - name: "status: in-progress"
      color: "fbca04"
      description: "Actively being worked on"
    - name: "status: review"
      color: "d93f0b"
      description: "Waiting for code review"
    - name: "status: testing"
      color: "a371f7"
      description: "Being tested/validated"
    - name: "status: done"
      color: "0e8a16"
      description: "Completed and merged"

  priority:
    - name: "priority: critical"
      color: "b60205"
      description: "Drop everything"
    - name: "priority: high"
      color: "d93f0b"
      description: "Next up"
    - name: "priority: medium"
      color: "fbca04"
      description: "Normal priority"
    - name: "priority: low"
      color: "0e8a16"
      description: "When time permits"

  type:
    - name: "type: bug"
      color: "d73a4a"
      description: "Something is broken"
    - name: "type: feature"
      color: "a2eeef"
      description: "New functionality"
    - name: "type: improvement"
      color: "84b6eb"
      description: "Enhancement to existing"
    - name: "type: docs"
      color: "0075ca"
      description: "Documentation work"
    - name: "type: refactor"
      color: "5319e7"
      description: "Code quality improvement"
    - name: "type: chore"
      color: "fef2c0"
      description: "Maintenance tasks"

  size:
    - name: "size: XS"
      color: "ededed"
      description: "< 1 hour"
    - name: "size: S"
      color: "d4d4d4"
      description: "1-4 hours"
    - name: "size: M"
      color: "bdbdbd"
      description: "1-2 days"
    - name: "size: L"
      color: "9e9e9e"
      description: "3-5 days"
    - name: "size: XL"
      color: "757575"
      description: "> 1 week"

  special:
    - name: "blocked"
      color: "000000"
      description: "Blocked by dependency"
    - name: "needs-triage"
      color: "fbca04"
      description: "Needs initial review"
    - name: "good-first-issue"
      color: "7057ff"
      description: "Good for newcomers"
    - name: "help-wanted"
      color: "008672"
      description: "Extra attention needed"
    - name: "wontfix"
      color: "ffffff"
      description: "This will not be worked on"
    - name: "duplicate"
      color: "cfd3d7"
      description: "This issue or PR already exists"

settings:
  preserve_unknown: true
  concurrency: 5
  wip_limits:
    "status: ready": 10
    "status: in-progress": 2
    "status: review": 10
    "status: testing": 5
`, org)
}
