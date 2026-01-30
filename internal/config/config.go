// Package config manages MAXAM configuration in ~/.maxam/
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ContextMode represents the context mode for system prompts
type ContextMode string

const (
	ContextModeFull    ContextMode = "full"
	ContextModeSummary ContextMode = "summary"
)

// Config represents the MAXAM configuration
type Config struct {
	Version             string        `yaml:"version"`
	DefaultAgent        string        `yaml:"default_agent,omitempty"`
	Agents              []AgentConfig `yaml:"agents"`
	AnalysisMinMessages int           `yaml:"analysis_min_messages,omitempty"`
	ContextMode         ContextMode   `yaml:"context_mode,omitempty"`
}

// AgentConfig represents an agent configuration
type AgentConfig struct {
	Name     string `yaml:"name"`
	FullName string `yaml:"full_name"`
	Role     string `yaml:"role"`
	Model    string `yaml:"model,omitempty"` // opus, sonnet, haiku
}

// DefaultAnalysisMinMessages is the default minimum message count for analysis
const DefaultAnalysisMinMessages = 10

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Version:             "1",
		DefaultAgent:        "mei",
		AnalysisMinMessages: DefaultAnalysisMinMessages,
		Agents: []AgentConfig{
			{Name: "mei", FullName: "Mei Chen", Role: "PM / 要件定義"},
			{Name: "yuki", FullName: "Yuki Tanaka", Role: "バックエンド / インフラ"},
			{Name: "rin", FullName: "Rin Sato", Role: "フロントエンド"},
			{Name: "shiori", FullName: "Shiori Tanaka", Role: "テスト / ドキュメント"},
			{Name: "priya", FullName: "Priya Sharma", Role: "レビュー / セキュリティ / QA"},
			{Name: "amara", FullName: "Amara Okonkwo", Role: "分析"},
		},
	}
}

// GetAnalysisMinMessages returns the analysis minimum messages with default fallback
func (c *Config) GetAnalysisMinMessages() int {
	if c.AnalysisMinMessages <= 0 {
		return DefaultAnalysisMinMessages
	}
	return c.AnalysisMinMessages
}

// GetContextMode returns the context mode with default fallback
func (c *Config) GetContextMode() ContextMode {
	if c.ContextMode == "" {
		return ContextModeFull
	}
	return c.ContextMode
}

// IsSummaryMode returns true if context mode is summary
func (c *Config) IsSummaryMode() bool {
	return c.GetContextMode() == ContextModeSummary
}

// ConfigDir returns the path to ~/.maxam/
func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, ".maxam"), nil
}

// AgentsDir returns the path to ~/.maxam/agents/
func AgentsDir() (string, error) {
	configDir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "agents"), nil
}

// Load reads configuration from ~/.maxam/config.yaml
func Load() (*Config, error) {
	configDir, err := ConfigDir()
	if err != nil {
		return nil, err
	}

	configPath := filepath.Join(configDir, "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	return &cfg, nil
}

// Save writes configuration to ~/.maxam/config.yaml
func Save(cfg *Config) error {
	configDir, err := ConfigDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	configPath := filepath.Join(configDir, "config.yaml")
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}
