package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/kiracore/kanban/internal/paths"
	"github.com/spf13/cobra"
)

var migrateXDGCmd = &cobra.Command{
	Use:   "migrate-xdg",
	Short: "Migrate data from legacy paths to XDG directories",
	Long: `Migrate kanban data from legacy paths (~/.kanban/) to XDG directories.

This command will:
  - Move ~/.kanban/kanban.db to ~/.local/share/kanban/kanban.db
  - Move ~/.kanban/backups/ to ~/.local/share/kanban/backups/

The XDG Base Directory Specification is the standard for Linux applications:
  - Config: ~/.config/kanban/ (or $XDG_CONFIG_HOME/kanban/)
  - Data:   ~/.local/share/kanban/ (or $XDG_DATA_HOME/kanban/)

Note: Configuration files should be manually copied to ~/.config/kanban/config.yaml`,
	RunE: runMigrateXDG,
}

func init() {
	rootCmd.AddCommand(migrateXDGCmd)
}

func runMigrateXDG(cmd *cobra.Command, args []string) error {
	fmt.Println("Checking for legacy data...")
	fmt.Println()

	legacyDir := paths.LegacyDataDir()
	migrated := false

	// Check if legacy directory exists
	if _, err := os.Stat(legacyDir); os.IsNotExist(err) {
		fmt.Println("No legacy data found at:", legacyDir)
		fmt.Println()
		fmt.Println("Current paths:")
		fmt.Printf("  Config: %s\n", paths.ConfigDir())
		fmt.Printf("  Data:   %s\n", paths.DataDir())
		return nil
	}

	// Migrate database
	legacyDB := paths.LegacyDBPath()
	newDB := paths.DatabasePath()

	if _, err := os.Stat(legacyDB); err == nil {
		if _, err := os.Stat(newDB); err == nil {
			fmt.Printf("Warning: Both legacy and new databases exist:\n")
			fmt.Printf("  Legacy: %s\n", legacyDB)
			fmt.Printf("  New:    %s\n", newDB)
			fmt.Println("  Skipping database migration. Please resolve manually.")
			fmt.Println()
		} else {
			// Ensure data directory exists
			if err := paths.EnsureDataDir(); err != nil {
				return fmt.Errorf("failed to create data directory: %w", err)
			}

			fmt.Printf("Migrating database...\n")
			fmt.Printf("  From: %s\n", legacyDB)
			fmt.Printf("  To:   %s\n", newDB)

			if err := copyFile(legacyDB, newDB); err != nil {
				return fmt.Errorf("failed to migrate database: %w", err)
			}
			fmt.Println("  Done!")
			fmt.Println()
			migrated = true
		}
	}

	// Migrate WAL and SHM files if they exist
	for _, suffix := range []string{"-wal", "-shm"} {
		legacyFile := legacyDB + suffix
		newFile := newDB + suffix
		if _, err := os.Stat(legacyFile); err == nil {
			if err := copyFile(legacyFile, newFile); err != nil {
				fmt.Printf("Warning: failed to migrate %s: %v\n", suffix, err)
			}
		}
	}

	// Migrate backups directory
	legacyBackups := paths.LegacyBackupDir()
	newBackups := paths.BackupDir()

	if _, err := os.Stat(legacyBackups); err == nil {
		if _, err := os.Stat(newBackups); err == nil {
			fmt.Printf("Warning: Both legacy and new backup directories exist:\n")
			fmt.Printf("  Legacy: %s\n", legacyBackups)
			fmt.Printf("  New:    %s\n", newBackups)
			fmt.Println("  Skipping backups migration. Please resolve manually.")
			fmt.Println()
		} else {
			fmt.Printf("Migrating backups...\n")
			fmt.Printf("  From: %s\n", legacyBackups)
			fmt.Printf("  To:   %s\n", newBackups)

			if err := copyDir(legacyBackups, newBackups); err != nil {
				return fmt.Errorf("failed to migrate backups: %w", err)
			}
			fmt.Println("  Done!")
			fmt.Println()
			migrated = true
		}
	}

	if migrated {
		fmt.Println("Migration complete!")
		fmt.Println()
		fmt.Println("You can now remove the legacy directory:")
		fmt.Printf("  rm -rf %s\n", legacyDir)
		fmt.Println()
	}

	fmt.Println("Current paths:")
	fmt.Printf("  Config: %s\n", paths.ConfigDir())
	fmt.Printf("  Data:   %s\n", paths.DataDir())

	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	// Copy permissions
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.Chmod(dst, info.Mode())
}

// copyDir recursively copies a directory from src to dst
func copyDir(src, dst string) error {
	// Get source directory info
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	// Create destination directory
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	// Read source directory
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}
