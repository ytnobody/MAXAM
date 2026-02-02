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
	YOLOMode            bool          `yaml:"yolo_mode,omitempty"`
	WorkersPerAgent     int           `yaml:"workers_per_agent,omitempty"`
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
		return ContextModeSummary
	}
	return c.ContextMode
}

// IsSummaryMode returns true if context mode is summary
func (c *Config) IsSummaryMode() bool {
	return c.GetContextMode() == ContextModeSummary
}

// DefaultWorkersPerAgent is the default number of workers per agent
const DefaultWorkersPerAgent = 1

// GetWorkersPerAgent returns workers per agent with default fallback
func (c *Config) GetWorkersPerAgent() int {
	if c.WorkersPerAgent <= 0 {
		return DefaultWorkersPerAgent
	}
	return c.WorkersPerAgent
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

// ProjectConfigDir returns the path to project/.maxam/
func ProjectConfigDir(projectDir string) string {
	return filepath.Join(projectDir, ".maxam")
}

// ProjectAgentsDir returns the path to project/.maxam/agents/
func ProjectAgentsDir(projectDir string) string {
	return filepath.Join(ProjectConfigDir(projectDir), "agents")
}

// ProjectClaudeMD returns the path to project/.maxam/CLAUDE.md
func ProjectClaudeMD(projectDir string) string {
	return filepath.Join(ProjectConfigDir(projectDir), "CLAUDE.md")
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

// LoadWithProject loads configuration with project-local overrides
// Priority: project/.maxam/config.yaml > ~/.maxam/config.yaml > default
func LoadWithProject(projectDir string) (*Config, error) {
	// 1. Start with global config
	cfg, err := Load()
	if err != nil {
		return nil, err
	}

	// 2. Try to load project-local config
	projectConfigPath := filepath.Join(ProjectConfigDir(projectDir), "config.yaml")
	data, err := os.ReadFile(projectConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			// No project config, return global
			return cfg, nil
		}
		return nil, fmt.Errorf("read project config: %w", err)
	}

	var projectCfg Config
	if err := yaml.Unmarshal(data, &projectCfg); err != nil {
		return nil, fmt.Errorf("parse project config: %w", err)
	}

	// 3. Merge: project config overwrites global config
	merged := mergeConfigs(cfg, &projectCfg)
	return merged, nil
}

// mergeConfigs merges base and override configs
// Override values take precedence when set
func mergeConfigs(base, override *Config) *Config {
	result := *base // Copy base

	// Override version if set
	if override.Version != "" {
		result.Version = override.Version
	}

	// Override default agent if set
	if override.DefaultAgent != "" {
		result.DefaultAgent = override.DefaultAgent
	}

	// Override agents if specified (complete replacement)
	if len(override.Agents) > 0 {
		result.Agents = override.Agents
	}

	// Override analysis min messages if set
	if override.AnalysisMinMessages > 0 {
		result.AnalysisMinMessages = override.AnalysisMinMessages
	}

	// Override context mode if set
	if override.ContextMode != "" {
		result.ContextMode = override.ContextMode
	}

	// Override YOLO mode (explicit true takes precedence)
	if override.YOLOMode {
		result.YOLOMode = true
	}

	// Override workers per agent if set
	if override.WorkersPerAgent > 0 {
		result.WorkersPerAgent = override.WorkersPerAgent
	}

	return &result
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
