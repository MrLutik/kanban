package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		pattern  string
		str      string
		expected bool
	}{
		// Wildcard all
		{"*", "anything", true},
		{"*", "", true},

		// Prefix wildcard
		{"*.github.io", "myorg.github.io", true},
		{"*.github.io", "github.io", false}, // * requires at least one char
		{"*.github.io", "myrepo", false},

		// Suffix wildcard
		{"archived-*", "archived-repo", true},
		{"archived-*", "archived-", true},
		{"archived-*", "repo-archived", false},

		// Exact match
		{"myrepo", "myrepo", true},
		{"myrepo", "otherrepo", false},
	}

	for _, tc := range tests {
		t.Run(tc.pattern+"_"+tc.str, func(t *testing.T) {
			result := matchPattern(tc.pattern, tc.str)
			if result != tc.expected {
				t.Errorf("matchPattern(%q, %q) = %v, want %v", tc.pattern, tc.str, result, tc.expected)
			}
		})
	}
}

func TestFilterRepos(t *testing.T) {
	tests := []struct {
		name     string
		config   LabelConfig
		input    []string
		expected []string
	}{
		{
			name:     "no filters",
			config:   LabelConfig{},
			input:    []string{"repo1", "repo2", "repo3"},
			expected: []string{"repo1", "repo2", "repo3"},
		},
		{
			name: "include all",
			config: LabelConfig{
				Repositories: RepoConfig{
					Include: []string{"*"},
				},
			},
			input:    []string{"repo1", "repo2"},
			expected: []string{"repo1", "repo2"},
		},
		{
			name: "exclude github.io",
			config: LabelConfig{
				Repositories: RepoConfig{
					Include: []string{"*"},
					Exclude: []string{"*.github.io"},
				},
			},
			input:    []string{"myrepo", "docs", "myorg.github.io"},
			expected: []string{"myrepo", "docs"},
		},
		{
			name: "exclude multiple patterns",
			config: LabelConfig{
				Repositories: RepoConfig{
					Include: []string{"*"},
					Exclude: []string{"*.github.io", ".github", "archived-*"},
				},
			},
			input:    []string{"sekai", "interx", ".github", "myorg.github.io", "archived-old"},
			expected: []string{"sekai", "interx"},
		},
		{
			name: "specific includes",
			config: LabelConfig{
				Repositories: RepoConfig{
					Include: []string{"sekai", "interx"},
				},
			},
			input:    []string{"sekai", "interx", "other"},
			expected: []string{"sekai", "interx"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.config.FilterRepos(tc.input)
			if len(result) != len(tc.expected) {
				t.Errorf("FilterRepos() returned %d items, want %d", len(result), len(tc.expected))
				t.Errorf("Got: %v, Want: %v", result, tc.expected)
				return
			}
			for i, r := range result {
				if r != tc.expected[i] {
					t.Errorf("FilterRepos()[%d] = %q, want %q", i, r, tc.expected[i])
				}
			}
		})
	}
}

func TestAllLabels(t *testing.T) {
	config := LabelConfig{
		Labels: map[string][]Label{
			"status": {
				{Name: "status: backlog", Color: "d4d4d4"},
				{Name: "status: done", Color: "0e8a16"},
			},
			"priority": {
				{Name: "priority: high", Color: "b60205"},
			},
		},
	}

	labels := config.AllLabels()

	if len(labels) != 3 {
		t.Errorf("AllLabels() returned %d labels, want 3", len(labels))
	}
}

func TestHasExplicitRepos(t *testing.T) {
	tests := []struct {
		name     string
		config   LabelConfig
		expected bool
	}{
		{
			name:     "no repos",
			config:   LabelConfig{},
			expected: false,
		},
		{
			name: "empty list",
			config: LabelConfig{
				Repositories: RepoConfig{
					List: []string{},
				},
			},
			expected: false,
		},
		{
			name: "with repos",
			config: LabelConfig{
				Repositories: RepoConfig{
					List: []string{"repo1", "repo2"},
				},
			},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.config.HasExplicitRepos()
			if result != tc.expected {
				t.Errorf("HasExplicitRepos() = %v, want %v", result, tc.expected)
			}
		})
	}
}

func TestGetRepos(t *testing.T) {
	config := LabelConfig{
		Repositories: RepoConfig{
			List: []string{"repo1", "repo2"},
		},
	}

	repos := config.GetRepos()
	if len(repos) != 2 {
		t.Errorf("GetRepos() returned %d repos, want 2", len(repos))
	}
	if repos[0] != "repo1" || repos[1] != "repo2" {
		t.Errorf("GetRepos() = %v, want [repo1, repo2]", repos)
	}
}

func TestLoadLabelsFromFile(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.yaml")

	configContent := `version: "1"
organization: testorg
repositories:
  include: ["*"]
  exclude: ["*.github.io"]
labels:
  status:
    - name: "status: backlog"
      color: "d4d4d4"
      description: "Prioritized but not started"
    - name: "status: done"
      color: "0e8a16"
      description: "Completed"
  priority:
    - name: "priority: high"
      color: "b60205"
      description: "High priority"
migrations:
  - from: "bug"
    to: "type: bug"
settings:
  preserve_unknown: true
  concurrency: 10
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Test loading
	cfg, err := LoadLabelsFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadLabelsFromFile() error: %v", err)
	}

	// Verify config values
	if cfg.Version != "1" {
		t.Errorf("Version = %q, want %q", cfg.Version, "1")
	}
	if cfg.Organization != "testorg" {
		t.Errorf("Organization = %q, want %q", cfg.Organization, "testorg")
	}
	if len(cfg.Repositories.Include) != 1 || cfg.Repositories.Include[0] != "*" {
		t.Errorf("Repositories.Include = %v, want [*]", cfg.Repositories.Include)
	}
	if len(cfg.Labels["status"]) != 2 {
		t.Errorf("len(Labels[status]) = %d, want 2", len(cfg.Labels["status"]))
	}
	if len(cfg.Labels["priority"]) != 1 {
		t.Errorf("len(Labels[priority]) = %d, want 1", len(cfg.Labels["priority"]))
	}
	if len(cfg.Migrations) != 1 {
		t.Errorf("len(Migrations) = %d, want 1", len(cfg.Migrations))
	}
	if cfg.Migrations[0].From != "bug" || cfg.Migrations[0].To != "type: bug" {
		t.Errorf("Migrations[0] = %+v, want from:bug to:type: bug", cfg.Migrations[0])
	}
	if cfg.Settings.Concurrency != 10 {
		t.Errorf("Settings.Concurrency = %d, want 10", cfg.Settings.Concurrency)
	}
}

func TestLoadLabelsFromFile_InvalidFile(t *testing.T) {
	_, err := LoadLabelsFromFile("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("LoadLabelsFromFile() should return error for nonexistent file")
	}
}

func TestLoadLabelsFromFile_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yaml")

	// Write invalid YAML
	if err := os.WriteFile(configPath, []byte("invalid: yaml: content: ["), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	_, err := LoadLabelsFromFile(configPath)
	if err == nil {
		t.Error("LoadLabelsFromFile() should return error for invalid YAML")
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	cfg := &LabelConfig{
		Version:      "1",
		Organization: "testorg",
		Labels: map[string][]Label{
			"status": {
				{Name: "status: backlog", Color: "d4d4d4", Description: "In backlog"},
				{Name: "status: done", Color: "0e8a16", Description: "Completed"},
			},
		},
		Repositories: RepoConfig{
			Include: []string{"*"},
		},
		Settings: Settings{
			Concurrency: 5,
		},
	}

	result := cfg.Validate()

	if !result.IsValid() {
		t.Errorf("Validate() returned errors for valid config: %v", result.Errors)
	}
}

func TestValidate_MissingOrganization(t *testing.T) {
	cfg := &LabelConfig{
		Version: "1",
		Labels: map[string][]Label{
			"status": {{Name: "status: backlog", Color: "d4d4d4"}},
		},
	}

	result := cfg.Validate()

	if result.IsValid() {
		t.Error("Validate() should return error for missing organization")
	}

	found := false
	for _, e := range result.Errors {
		if e.Field == "organization" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Validate() should have organization error")
	}
}

func TestValidate_InvalidLabelName(t *testing.T) {
	cfg := &LabelConfig{
		Version:      "1",
		Organization: "testorg",
		Labels: map[string][]Label{
			"status": {
				{Name: "", Color: "d4d4d4"},           // empty name
				{Name: "-invalid", Color: "d4d4d4"},   // starts with hyphen
				{Name: "valid-name", Color: "d4d4d4"}, // valid
			},
		},
	}

	result := cfg.Validate()

	if result.IsValid() {
		t.Error("Validate() should return errors for invalid label names")
	}

	// Should have error for empty name and invalid format
	if len(result.Errors) < 2 {
		t.Errorf("Expected at least 2 errors, got %d", len(result.Errors))
	}
}

func TestValidate_InvalidLabelColor(t *testing.T) {
	cfg := &LabelConfig{
		Version:      "1",
		Organization: "testorg",
		Labels: map[string][]Label{
			"status": {
				{Name: "label1", Color: "#ff0000"},  // has # prefix
				{Name: "label2", Color: "ff00"},    // too short
				{Name: "label3", Color: "gggggg"}, // invalid hex chars
				{Name: "label4", Color: "ff0000"}, // valid
			},
		},
	}

	result := cfg.Validate()

	// Should have errors for invalid colors
	colorErrors := 0
	for _, e := range result.Errors {
		if e.Field != "" && len(e.Message) > 0 {
			colorErrors++
		}
	}
	if colorErrors < 3 {
		t.Errorf("Expected at least 3 color errors, got %d", colorErrors)
	}
}

func TestValidate_DuplicateLabelNames(t *testing.T) {
	cfg := &LabelConfig{
		Version:      "1",
		Organization: "testorg",
		Labels: map[string][]Label{
			"status": {
				{Name: "status: backlog", Color: "d4d4d4"},
				{Name: "STATUS: BACKLOG", Color: "ff0000"}, // duplicate (case insensitive)
			},
		},
	}

	result := cfg.Validate()

	if result.IsValid() {
		t.Error("Validate() should return error for duplicate label names")
	}

	found := false
	for _, e := range result.Errors {
		if e.Message == "duplicate label name \"STATUS: BACKLOG\"" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Validate() should have duplicate name error")
	}
}

func TestValidate_Migrations(t *testing.T) {
	cfg := &LabelConfig{
		Version:      "1",
		Organization: "testorg",
		Labels: map[string][]Label{
			"status": {{Name: "status: backlog", Color: "d4d4d4"}},
		},
		Migrations: []Migration{
			{From: "", To: "new-label"},       // missing from
			{From: "old-label", To: ""},       // missing to
			{From: "same", To: "same"},        // same from/to
			{From: "valid", To: "new-valid"},  // valid
		},
	}

	result := cfg.Validate()

	// Check for errors on missing from/to
	fromError := false
	toError := false
	for _, e := range result.Errors {
		if e.Field == "migrations[0].from" {
			fromError = true
		}
		if e.Field == "migrations[1].to" {
			toError = true
		}
	}
	if !fromError {
		t.Error("Validate() should have error for missing from")
	}
	if !toError {
		t.Error("Validate() should have error for missing to")
	}

	// Check for warning on same from/to
	sameWarning := false
	for _, w := range result.Warnings {
		if w.Field == "migrations[2]" && w.Message == "from and to are the same" {
			sameWarning = true
		}
	}
	if !sameWarning {
		t.Error("Validate() should have warning for same from/to")
	}
}

func TestValidate_Settings(t *testing.T) {
	tests := []struct {
		name        string
		concurrency int
		wantWarning bool
	}{
		{"zero concurrency", 0, true},
		{"valid concurrency", 5, false},
		{"high concurrency", 25, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &LabelConfig{
				Version:      "1",
				Organization: "testorg",
				Labels: map[string][]Label{
					"status": {{Name: "status: backlog", Color: "d4d4d4"}},
				},
				Settings: Settings{
					Concurrency: tc.concurrency,
				},
			}

			result := cfg.Validate()

			hasWarning := false
			for _, w := range result.Warnings {
				if w.Field == "settings.concurrency" {
					hasWarning = true
					break
				}
			}

			if tc.wantWarning && !hasWarning {
				t.Errorf("Expected warning for concurrency=%d", tc.concurrency)
			}
			if !tc.wantWarning && hasWarning {
				t.Errorf("Unexpected warning for concurrency=%d", tc.concurrency)
			}
		})
	}
}

func TestValidate_Repositories(t *testing.T) {
	tests := []struct {
		name         string
		repos        RepoConfig
		wantWarnings int
		wantErrors   int
	}{
		{
			name:         "no repos",
			repos:        RepoConfig{},
			wantWarnings: 1, // "no repository configuration"
		},
		{
			name: "both list and patterns",
			repos: RepoConfig{
				List:    []string{"repo1"},
				Include: []string{"*"},
			},
			wantWarnings: 1, // "both explicit list and patterns"
		},
		{
			name: "empty pattern",
			repos: RepoConfig{
				Include: []string{"*", ""},
			},
			wantErrors: 1, // "empty pattern"
		},
		{
			name: "empty repo name",
			repos: RepoConfig{
				List: []string{"repo1", ""},
			},
			wantErrors: 1, // "empty repository name"
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &LabelConfig{
				Version:      "1",
				Organization: "testorg",
				Labels: map[string][]Label{
					"status": {{Name: "status: backlog", Color: "d4d4d4"}},
				},
				Repositories: tc.repos,
				Settings:     Settings{Concurrency: 5},
			}

			result := cfg.Validate()

			if len(result.Warnings) < tc.wantWarnings {
				t.Errorf("Expected at least %d warnings, got %d: %v", tc.wantWarnings, len(result.Warnings), result.Warnings)
			}
			if len(result.Errors) < tc.wantErrors {
				t.Errorf("Expected at least %d errors, got %d: %v", tc.wantErrors, len(result.Errors), result.Errors)
			}
		})
	}
}

func TestValidationResult_Methods(t *testing.T) {
	result := &ValidationResult{}

	if !result.IsValid() {
		t.Error("Empty result should be valid")
	}
	if result.HasWarnings() {
		t.Error("Empty result should not have warnings")
	}

	result.AddError("field1", "error message")
	if result.IsValid() {
		t.Error("Result with error should not be valid")
	}

	result.AddWarning("field2", "warning message")
	if !result.HasWarnings() {
		t.Error("Result with warning should have warnings")
	}
}

func TestValidationError_Error(t *testing.T) {
	e := ValidationError{Field: "test.field", Message: "test message"}
	expected := "test.field: test message"
	if e.Error() != expected {
		t.Errorf("Error() = %q, want %q", e.Error(), expected)
	}
}
