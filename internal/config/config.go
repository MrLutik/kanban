package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationResult holds all validation errors
type ValidationResult struct {
	Errors   []ValidationError
	Warnings []ValidationError
}

func (r *ValidationResult) AddError(field, message string) {
	r.Errors = append(r.Errors, ValidationError{Field: field, Message: message})
}

func (r *ValidationResult) AddWarning(field, message string) {
	r.Warnings = append(r.Warnings, ValidationError{Field: field, Message: message})
}

func (r *ValidationResult) IsValid() bool {
	return len(r.Errors) == 0
}

func (r *ValidationResult) HasWarnings() bool {
	return len(r.Warnings) > 0
}

// Validate validates the configuration
func (c *LabelConfig) Validate() *ValidationResult {
	result := &ValidationResult{}

	// Version check
	if c.Version == "" {
		result.AddWarning("version", "version not specified, assuming v1")
	} else if c.Version != "1" {
		result.AddWarning("version", fmt.Sprintf("unknown version %q, expected \"1\"", c.Version))
	}

	// Organization required
	if c.Organization == "" {
		result.AddError("organization", "organization is required")
	}

	// Validate labels
	c.validateLabels(result)

	// Validate repositories
	c.validateRepositories(result)

	// Validate migrations
	c.validateMigrations(result)

	// Validate settings
	c.validateSettings(result)

	return result
}

var (
	hexColorRegex = regexp.MustCompile(`^[0-9a-fA-F]{6}$`)
	labelNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9 :\-_\.]*$`)
)

func (c *LabelConfig) validateLabels(result *ValidationResult) {
	if len(c.Labels) == 0 {
		result.AddWarning("labels", "no labels defined")
		return
	}

	seenNames := make(map[string]bool)

	for category, labels := range c.Labels {
		if len(labels) == 0 {
			result.AddWarning(fmt.Sprintf("labels.%s", category), "empty label category")
			continue
		}

		for i, label := range labels {
			field := fmt.Sprintf("labels.%s[%d]", category, i)

			// Name required
			if label.Name == "" {
				result.AddError(field+".name", "label name is required")
				continue
			}

			// Name format
			if !labelNameRegex.MatchString(label.Name) {
				result.AddError(field+".name", fmt.Sprintf("invalid label name %q (must start with alphanumeric, contain only alphanumeric, spaces, colons, hyphens, underscores, dots)", label.Name))
			}

			// Duplicate check
			if seenNames[strings.ToLower(label.Name)] {
				result.AddError(field+".name", fmt.Sprintf("duplicate label name %q", label.Name))
			}
			seenNames[strings.ToLower(label.Name)] = true

			// Color format
			if label.Color == "" {
				result.AddWarning(field+".color", "color not specified, will use default")
			} else if !hexColorRegex.MatchString(label.Color) {
				result.AddError(field+".color", fmt.Sprintf("invalid color %q (must be 6-digit hex without #)", label.Color))
			}

			// Description check
			if label.Description == "" {
				result.AddWarning(field+".description", "description not specified")
			} else if len(label.Description) > 100 {
				result.AddWarning(field+".description", "description exceeds 100 characters (GitHub limit)")
			}
		}
	}
}

func (c *LabelConfig) validateRepositories(result *ValidationResult) {
	hasExplicit := len(c.Repositories.List) > 0
	hasPatterns := len(c.Repositories.Include) > 0 || len(c.Repositories.Exclude) > 0

	if !hasExplicit && !hasPatterns {
		result.AddWarning("repositories", "no repository configuration, will require --repo or --all flag")
	}

	if hasExplicit && hasPatterns {
		result.AddWarning("repositories", "both explicit list and patterns defined, list takes precedence")
	}

	// Validate patterns
	for i, pattern := range c.Repositories.Include {
		if pattern == "" {
			result.AddError(fmt.Sprintf("repositories.include[%d]", i), "empty pattern")
		}
	}
	for i, pattern := range c.Repositories.Exclude {
		if pattern == "" {
			result.AddError(fmt.Sprintf("repositories.exclude[%d]", i), "empty pattern")
		}
	}

	// Validate explicit repos
	for i, repo := range c.Repositories.List {
		if repo == "" {
			result.AddError(fmt.Sprintf("repositories.list[%d]", i), "empty repository name")
		}
	}
}

func (c *LabelConfig) validateMigrations(result *ValidationResult) {
	seenFrom := make(map[string]bool)

	for i, m := range c.Migrations {
		field := fmt.Sprintf("migrations[%d]", i)

		if m.From == "" {
			result.AddError(field+".from", "from label is required")
		}
		if m.To == "" {
			result.AddError(field+".to", "to label is required")
		}

		if m.From != "" && seenFrom[strings.ToLower(m.From)] {
			result.AddWarning(field+".from", fmt.Sprintf("duplicate migration from %q", m.From))
		}
		seenFrom[strings.ToLower(m.From)] = true

		if m.From == m.To {
			result.AddWarning(field, "from and to are the same")
		}
	}
}

func (c *LabelConfig) validateSettings(result *ValidationResult) {
	if c.Settings.Concurrency < 1 {
		result.AddWarning("settings.concurrency", "concurrency < 1, will use default (5)")
	} else if c.Settings.Concurrency > 20 {
		result.AddWarning("settings.concurrency", "concurrency > 20 may cause rate limiting")
	}

	for status, limit := range c.Settings.WIPLimits {
		if limit < 1 {
			result.AddWarning(fmt.Sprintf("settings.wip_limits.%s", status), "WIP limit < 1 is not useful")
		}
	}
}

// Label represents a GitHub label
type Label struct {
	Name        string `yaml:"name" json:"name"`
	Color       string `yaml:"color" json:"color"`
	Description string `yaml:"description" json:"description"`
}

// LabelConfig represents the label configuration file
type LabelConfig struct {
	Version      string              `yaml:"version" json:"version"`
	Organization string              `yaml:"organization" json:"organization"`
	Repositories RepoConfig          `yaml:"repositories" json:"repositories"`
	Maintainers  []string            `yaml:"maintainers" json:"maintainers"`
	Labels       map[string][]Label  `yaml:"labels" json:"labels"`
	Migrations   []Migration         `yaml:"migrations" json:"migrations"`
	Settings     Settings            `yaml:"settings" json:"settings"`
}

// RepoConfig defines which repos to include/exclude
type RepoConfig struct {
	List    []string `yaml:"list" json:"list"`       // Explicit list of repos
	Include []string `yaml:"include" json:"include"` // Pattern-based include
	Exclude []string `yaml:"exclude" json:"exclude"` // Pattern-based exclude
}

// GetRepos returns the explicit repo list or nil if using patterns
func (c *LabelConfig) GetRepos() []string {
	return c.Repositories.List
}

// HasExplicitRepos returns true if explicit repo list is defined
func (c *LabelConfig) HasExplicitRepos() bool {
	return len(c.Repositories.List) > 0
}

// Migration defines label migration mapping
type Migration struct {
	From string `yaml:"from" json:"from"`
	To   string `yaml:"to" json:"to"`
}

// Settings holds configuration settings
type Settings struct {
	PreserveUnknown bool           `yaml:"preserve_unknown" json:"preserve_unknown"`
	Concurrency     int            `yaml:"concurrency" json:"concurrency"`
	WIPLimits       map[string]int `yaml:"wip_limits" json:"wip_limits"`
}

// Load loads configuration from viper
func Load() (*LabelConfig, error) {
	cfg := &LabelConfig{
		Settings: Settings{
			PreserveUnknown: true,
			Concurrency:     5,
		},
	}

	if err := viper.Unmarshal(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// LoadLabelsFromFile loads labels from a yaml/json file
func LoadLabelsFromFile(path string) (*LabelConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &LabelConfig{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// AllLabels returns all labels from all categories
func (c *LabelConfig) AllLabels() []Label {
	var labels []Label
	for _, categoryLabels := range c.Labels {
		labels = append(labels, categoryLabels...)
	}
	return labels
}

// FilterRepos filters repos based on include/exclude patterns
func (c *LabelConfig) FilterRepos(repos []string) []string {
	if len(c.Repositories.Include) == 0 && len(c.Repositories.Exclude) == 0 {
		return repos
	}

	var filtered []string
	for _, repo := range repos {
		if c.shouldIncludeRepo(repo) {
			filtered = append(filtered, repo)
		}
	}
	return filtered
}

func (c *LabelConfig) shouldIncludeRepo(repo string) bool {
	// Check excludes first
	for _, pattern := range c.Repositories.Exclude {
		if matchPattern(pattern, repo) {
			return false
		}
	}

	// Check includes
	if len(c.Repositories.Include) == 0 {
		return true
	}

	for _, pattern := range c.Repositories.Include {
		if matchPattern(pattern, repo) {
			return true
		}
	}

	return false
}

// matchPattern does simple glob matching
func matchPattern(pattern, str string) bool {
	if pattern == "*" {
		return true
	}

	// Handle prefix wildcard (*.github.io)
	if strings.HasPrefix(pattern, "*") {
		return strings.HasSuffix(str, pattern[1:])
	}

	// Handle suffix wildcard (archived-*)
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(str, pattern[:len(pattern)-1])
	}

	// Handle full glob with filepath.Match
	matched, _ := filepath.Match(pattern, str)
	return matched
}
