package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kiracore/kanban/internal/db"
	"github.com/spf13/cobra"
)

var (
	dbPath     string
	backupPath string
)

// dbCmd represents the db command
var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Database management commands",
	Long: `Manage the kanban SQLite database.

The database stores cached GitHub data, metrics history, and status transitions
for efficient querying and historical analysis.

Examples:
  kanban db init                    # Initialize database
  kanban db status                  # Show database status
  kanban db backup -o backup.db     # Backup database
  kanban db restore -i backup.db    # Restore from backup
  kanban db export > data.json      # Export to JSON
  kanban db import < data.json      # Import from JSON`,
}

// dbInitCmd initializes the database
var dbInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the database",
	Long:  `Creates the kanban database with the required schema.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer database.Close()

		if err := database.Init(); err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}

		fmt.Printf("✓ Database initialized at: %s\n", database.Path())
		return nil
	},
}

// dbStatusCmd shows database status
var dbStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show database status and statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer database.Close()

		stats, err := database.GetStats()
		if err != nil {
			return fmt.Errorf("failed to get stats: %w", err)
		}

		fmt.Println("╔════════════════════════════════════════════════════════════╗")
		fmt.Println("║                    DATABASE STATUS                         ║")
		fmt.Println("╠════════════════════════════════════════════════════════════╣")
		fmt.Printf("║  Path:           %-40s ║\n", truncateStr(stats.Path, 40))
		fmt.Printf("║  Size:           %-40s ║\n", formatBytes(stats.Size))
		fmt.Printf("║  Schema Version: %-40d ║\n", stats.SchemaVersion)
		fmt.Println("╠════════════════════════════════════════════════════════════╣")
		fmt.Printf("║  Organizations:  %-40d ║\n", stats.Organizations)
		fmt.Printf("║  Repositories:   %-40d ║\n", stats.Repositories)
		fmt.Printf("║  Issues:         %-40d ║\n", stats.Issues)
		fmt.Printf("║  Pull Requests:  %-40d ║\n", stats.PullRequests)
		fmt.Printf("║  Labels:         %-40d ║\n", stats.Labels)
		fmt.Printf("║  Transitions:    %-40d ║\n", stats.Transitions)
		fmt.Printf("║  Metrics Days:   %-40d ║\n", stats.MetricsDays)
		fmt.Println("╠════════════════════════════════════════════════════════════╣")
		lastSync := "Never"
		if !stats.LastSync.IsZero() {
			lastSync = stats.LastSync.Format("2006-01-02 15:04:05")
		}
		fmt.Printf("║  Last Sync:      %-40s ║\n", lastSync)
		fmt.Println("╚════════════════════════════════════════════════════════════╝")

		return nil
	},
}

// dbPathCmd shows the database path
var dbPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show the database file path",
	Run: func(cmd *cobra.Command, args []string) {
		if dbPath != "" {
			fmt.Println(dbPath)
		} else {
			fmt.Println(db.DefaultDBPath())
		}
	},
}

// dbBackupCmd backs up the database
var dbBackupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Backup the database",
	Long: `Creates a backup copy of the database.

If no output path is specified, creates a timestamped backup in ~/.kanban/backups/`,
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer database.Close()

		// Generate backup path if not specified
		dest := backupPath
		if dest == "" {
			home, _ := os.UserHomeDir()
			backupDir := filepath.Join(home, ".kanban", "backups")
			timestamp := time.Now().Format("20060102-150405")
			dest = filepath.Join(backupDir, fmt.Sprintf("kanban-%s.db", timestamp))
		}

		if err := database.Backup(dest); err != nil {
			return fmt.Errorf("failed to backup database: %w", err)
		}

		info, _ := os.Stat(dest)
		fmt.Printf("✓ Database backed up to: %s (%s)\n", dest, formatBytes(info.Size()))
		return nil
	},
}

// dbRestoreCmd restores the database from backup
var dbRestoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Restore database from backup",
	Long:  `Restores the database from a backup file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if backupPath == "" {
			return fmt.Errorf("backup path required: use --input or -i")
		}

		// Verify backup file exists
		if _, err := os.Stat(backupPath); os.IsNotExist(err) {
			return fmt.Errorf("backup file not found: %s", backupPath)
		}

		database, err := db.Open(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}

		if err := database.Restore(backupPath); err != nil {
			return fmt.Errorf("failed to restore database: %w", err)
		}

		fmt.Printf("✓ Database restored from: %s\n", backupPath)
		return nil
	},
}

// dbExportCmd exports database to JSON
var dbExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export database to JSON",
	Long: `Exports all database data to JSON format.

Output goes to stdout by default. Redirect to a file:
  kanban db export > backup.json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer database.Close()

		if err := database.Export(os.Stdout); err != nil {
			return fmt.Errorf("failed to export database: %w", err)
		}

		return nil
	},
}

// dbImportCmd imports database from JSON
var dbImportCmd = &cobra.Command{
	Use:   "import",
	Short: "Import database from JSON",
	Long: `Imports data from JSON format.

Input comes from stdin by default:
  kanban db import < backup.json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer database.Close()

		// Initialize schema if needed
		if err := database.Init(); err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}

		if err := database.Import(os.Stdin); err != nil {
			return fmt.Errorf("failed to import database: %w", err)
		}

		fmt.Fprintln(os.Stderr, "✓ Database imported successfully")
		return nil
	},
}

// dbResetCmd resets the database
var dbResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset the database (destroys all data)",
	Long:  `Removes and reinitializes the database. All data will be lost!`,
	RunE: func(cmd *cobra.Command, args []string) error {
		path := dbPath
		if path == "" {
			path = db.DefaultDBPath()
		}

		// Remove existing database files
		os.Remove(path)
		os.Remove(path + "-wal")
		os.Remove(path + "-shm")

		// Reinitialize
		database, err := db.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer database.Close()

		if err := database.Init(); err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}

		fmt.Printf("✓ Database reset at: %s\n", path)
		return nil
	},
}

// dbOptimizeCmd optimizes the database
var dbOptimizeCmd = &cobra.Command{
	Use:   "optimize",
	Short: "Optimize database performance",
	Long: `Runs VACUUM and ANALYZE to optimize database performance.

VACUUM reclaims unused space and defragments the database file.
ANALYZE updates statistics used by the query planner.

Run this periodically (weekly) or after large data changes.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer database.Close()

		// Get size before
		statsBefore, _ := database.GetStats()

		fmt.Println("Optimizing database...")
		fmt.Println("  Running VACUUM...")
		if err := database.Vacuum(); err != nil {
			return fmt.Errorf("VACUUM failed: %w", err)
		}

		fmt.Println("  Running ANALYZE...")
		if err := database.Analyze(); err != nil {
			return fmt.Errorf("ANALYZE failed: %w", err)
		}

		// Get size after
		statsAfter, _ := database.GetStats()

		saved := statsBefore.Size - statsAfter.Size
		if saved > 0 {
			fmt.Printf("✓ Optimization complete. Reclaimed %s\n", formatBytes(saved))
		} else {
			fmt.Println("✓ Optimization complete. Database was already optimized.")
		}
		fmt.Printf("  Size: %s\n", formatBytes(statsAfter.Size))

		return nil
	},
}

func init() {
	rootCmd.AddCommand(dbCmd)

	// Add subcommands
	dbCmd.AddCommand(dbInitCmd)
	dbCmd.AddCommand(dbStatusCmd)
	dbCmd.AddCommand(dbPathCmd)
	dbCmd.AddCommand(dbBackupCmd)
	dbCmd.AddCommand(dbRestoreCmd)
	dbCmd.AddCommand(dbExportCmd)
	dbCmd.AddCommand(dbImportCmd)
	dbCmd.AddCommand(dbResetCmd)
	dbCmd.AddCommand(dbOptimizeCmd)

	// Flags
	dbCmd.PersistentFlags().StringVar(&dbPath, "db", "", "database path (default ~/.kanban/kanban.db)")
	dbBackupCmd.Flags().StringVar(&backupPath, "output", "", "backup output path")
	dbRestoreCmd.Flags().StringVar(&backupPath, "input", "", "backup input path")
}

// Helper functions

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return "..." + s[len(s)-maxLen+3:]
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
