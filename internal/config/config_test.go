package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Version != "1" {
		t.Errorf("Version = %q, want %q", cfg.Version, "1")
	}
	if len(cfg.Agents) != 6 {
		t.Errorf("Agents count = %d, want 6", len(cfg.Agents))
	}
	if cfg.AnalysisMinMessages != DefaultAnalysisMinMessages {
		t.Errorf("AnalysisMinMessages = %d, want %d", cfg.AnalysisMinMessages, DefaultAnalysisMinMessages)
	}
}

func TestGetAnalysisMinMessages(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *Config
		expected int
	}{
		{
			name:     "デフォルト設定",
			cfg:      DefaultConfig(),
			expected: DefaultAnalysisMinMessages,
		},
		{
			name:     "カスタム値",
			cfg:      &Config{AnalysisMinMessages: 20},
			expected: 20,
		},
		{
			name:     "0の場合はデフォルト",
			cfg:      &Config{AnalysisMinMessages: 0},
			expected: DefaultAnalysisMinMessages,
		},
		{
			name:     "負の値の場合はデフォルト",
			cfg:      &Config{AnalysisMinMessages: -5},
			expected: DefaultAnalysisMinMessages,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.GetAnalysisMinMessages()
			if got != tt.expected {
				t.Errorf("GetAnalysisMinMessages() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestConfigDir(t *testing.T) {
	dir, err := ConfigDir()
	if err != nil {
		t.Fatalf("ConfigDir() error: %v", err)
	}

	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".maxam")
	if dir != expected {
		t.Errorf("ConfigDir() = %q, want %q", dir, expected)
	}
}

func TestAgentsDir(t *testing.T) {
	dir, err := AgentsDir()
	if err != nil {
		t.Fatalf("AgentsDir() error: %v", err)
	}

	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".maxam", "agents")
	if dir != expected {
		t.Errorf("AgentsDir() = %q, want %q", dir, expected)
	}
}

func TestLoadNonExistent(t *testing.T) {
	// When config doesn't exist, should return default
	// Note: This test assumes ~/.maxam/config.yaml might or might not exist
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg == nil {
		t.Error("Load() returned nil config")
	}
}

func TestGetEmbeddedAgents(t *testing.T) {
	agents, err := GetEmbeddedAgents()
	if err != nil {
		t.Fatalf("GetEmbeddedAgents() error: %v", err)
	}

	// Check all default agents are embedded
	for _, name := range DefaultAgents {
		if _, ok := agents[name]; !ok {
			t.Errorf("Agent %q not found in embedded agents", name)
		}
	}
}

func TestEnsureInitialized(t *testing.T) {
	// Create a temp dir to simulate home
	tmpHome := t.TempDir()

	// Override HOME for this test
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", originalHome)

	// Get embedded agents
	agents, err := GetEmbeddedAgents()
	if err != nil {
		t.Fatalf("GetEmbeddedAgents() error: %v", err)
	}

	// Initialize
	if err := EnsureInitialized(agents); err != nil {
		t.Fatalf("EnsureInitialized() error: %v", err)
	}

	// Check config.yaml was created
	configPath := filepath.Join(tmpHome, ".maxam", "config.yaml")
	if _, err := os.Stat(configPath); err != nil {
		t.Errorf("config.yaml not created: %v", err)
	}

	// Check agents were installed
	for name := range agents {
		claudeMD := filepath.Join(tmpHome, ".maxam", "agents", name, "CLAUDE.md")
		if _, err := os.Stat(claudeMD); err != nil {
			t.Errorf("Agent %s CLAUDE.md not installed: %v", name, err)
		}
	}
}

func TestListAgents(t *testing.T) {
	// Create a temp dir
	tmpHome := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", originalHome)

	// Initialize with embedded agents
	agents, _ := GetEmbeddedAgents()
	EnsureInitialized(agents)

	// List agents
	list, err := ListAgents()
	if err != nil {
		t.Fatalf("ListAgents() error: %v", err)
	}

	if len(list) != len(DefaultAgents) {
		t.Errorf("ListAgents() returned %d agents, want %d", len(list), len(DefaultAgents))
	}
}

func TestGetContextMode(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *Config
		expected ContextMode
	}{
		{
			name:     "デフォルト（空）はsummary",
			cfg:      &Config{},
			expected: ContextModeSummary,
		},
		{
			name:     "明示的にfull",
			cfg:      &Config{ContextMode: ContextModeFull},
			expected: ContextModeFull,
		},
		{
			name:     "summary",
			cfg:      &Config{ContextMode: ContextModeSummary},
			expected: ContextModeSummary,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.GetContextMode()
			if got != tt.expected {
				t.Errorf("GetContextMode() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestIsSummaryMode(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *Config
		expected bool
	}{
		{
			name:     "デフォルトはtrue（summaryがデフォルト）",
			cfg:      &Config{},
			expected: true,
		},
		{
			name:     "fullはfalse",
			cfg:      &Config{ContextMode: ContextModeFull},
			expected: false,
		},
		{
			name:     "summaryはtrue",
			cfg:      &Config{ContextMode: ContextModeSummary},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.IsSummaryMode()
			if got != tt.expected {
				t.Errorf("IsSummaryMode() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAgentConfigModel(t *testing.T) {
	tests := []struct {
		name     string
		config   AgentConfig
		hasModel bool
	}{
		{
			name:     "モデル未指定",
			config:   AgentConfig{Name: "test", FullName: "Test Agent", Role: "test"},
			hasModel: false,
		},
		{
			name:     "Sonnet指定",
			config:   AgentConfig{Name: "test", FullName: "Test Agent", Role: "test", Model: "sonnet"},
			hasModel: true,
		},
		{
			name:     "Opus指定",
			config:   AgentConfig{Name: "test", FullName: "Test Agent", Role: "test", Model: "opus"},
			hasModel: true,
		},
		{
			name:     "Haiku指定",
			config:   AgentConfig{Name: "test", FullName: "Test Agent", Role: "test", Model: "haiku"},
			hasModel: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.hasModel && tt.config.Model == "" {
				t.Error("Expected model to be set")
			}
			if !tt.hasModel && tt.config.Model != "" {
				t.Error("Expected model to be empty")
			}
		})
	}
}
