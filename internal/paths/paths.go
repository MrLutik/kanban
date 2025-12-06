package paths

import (
	"os"
	"path/filepath"
)

const (
	// AppName is the application name used in XDG directories
	AppName = "kanban"
)

// DataDir returns the XDG data directory for kanban.
// Priority: $XDG_DATA_HOME/kanban -> ~/.local/share/kanban
func DataDir() string {
	if xdgData := os.Getenv("XDG_DATA_HOME"); xdgData != "" {
		return filepath.Join(xdgData, AppName)
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", AppName)
}

// ConfigDir returns the XDG config directory for kanban.
// Priority: $XDG_CONFIG_HOME/kanban -> ~/.config/kanban
func ConfigDir() string {
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(xdgConfig, AppName)
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", AppName)
}

// DatabasePath returns the default database file path.
// Returns: $XDG_DATA_HOME/kanban/kanban.db or ~/.local/share/kanban/kanban.db
func DatabasePath() string {
	return filepath.Join(DataDir(), "kanban.db")
}

// BackupDir returns the default backup directory.
// Returns: $XDG_DATA_HOME/kanban/backups or ~/.local/share/kanban/backups
func BackupDir() string {
	return filepath.Join(DataDir(), "backups")
}

// ConfigFilePath returns the default config file path in XDG config dir.
// Returns: $XDG_CONFIG_HOME/kanban/config.yaml or ~/.config/kanban/config.yaml
func ConfigFilePath() string {
	return filepath.Join(ConfigDir(), "config.yaml")
}

// LegacyDataDir returns the old data directory for migration.
// Returns: ~/.kanban
func LegacyDataDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".kanban")
}

// LegacyDBPath returns the old database path for migration.
// Returns: ~/.kanban/kanban.db
func LegacyDBPath() string {
	return filepath.Join(LegacyDataDir(), "kanban.db")
}

// LegacyBackupDir returns the old backup directory for migration.
// Returns: ~/.kanban/backups
func LegacyBackupDir() string {
	return filepath.Join(LegacyDataDir(), "backups")
}

// EnsureDataDir creates the data directory if it doesn't exist.
func EnsureDataDir() error {
	return os.MkdirAll(DataDir(), 0755)
}

// EnsureConfigDir creates the config directory if it doesn't exist.
func EnsureConfigDir() error {
	return os.MkdirAll(ConfigDir(), 0755)
}

// EnsureBackupDir creates the backup directory if it doesn't exist.
func EnsureBackupDir() error {
	return os.MkdirAll(BackupDir(), 0755)
}
