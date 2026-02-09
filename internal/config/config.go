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

// Mode represents the MAXAM operating mode
type Mode string

const (
	// ModeInteractive is for free-form conversation with the team
	ModeInteractive Mode = "interactive"
	// ModePlan is for automated project analysis and planning
	ModePlan Mode = "plan"
	// ModeAuto is for automated implementation (only accessible from plan mode)
	ModeAuto Mode = "auto"
)

// ValidModes returns all valid mode values
func ValidModes() []Mode {
	return []Mode{ModeInteractive, ModePlan, ModeAuto}
}

// IsValidMode checks if the given mode is valid
func IsValidMode(m Mode) bool {
	for _, valid := range ValidModes() {
		if m == valid {
			return true
		}
	}
	return false
}

// HeartbeatConfig holds heartbeat monitoring settings
type HeartbeatConfig struct {
	Interval   string `yaml:"interval,omitempty"`    // Monitoring interval (e.g., "10s")
	Timeout    string `yaml:"timeout,omitempty"`     // Response timeout (e.g., "30s")
	MaxRetries int    `yaml:"max_retries,omitempty"` // Max restart attempts
}

// GitHubConfig holds GitHub repository settings
type GitHubConfig struct {
	Owner string `yaml:"owner"` // Repository owner (e.g., "ytnobody")
	Repo  string `yaml:"repo"`  // Repository name (e.g., "MAXAM")
}

// Config represents the MAXAM configuration
type Config struct {
	Version               string           `yaml:"version"`
	TeamName              string           `yaml:"team_name,omitempty"`
	DefaultAgent          string           `yaml:"default_agent,omitempty"`
	DefaultMode           Mode             `yaml:"default_mode,omitempty"`
	Agents                []AgentConfig    `yaml:"agents"`
	AnalysisMinMessages   int              `yaml:"analysis_min_messages,omitempty"`
	ContextMode           ContextMode      `yaml:"context_mode,omitempty"`
	YOLOMode              bool             `yaml:"yolo_mode,omitempty"`
	WorkersPerAgent       int              `yaml:"workers_per_agent,omitempty"`
	MentionCheckerEnabled *bool            `yaml:"mention_checker_enabled,omitempty"`
	LogLevel              string           `yaml:"log_level,omitempty"`
	Heartbeat             *HeartbeatConfig `yaml:"heartbeat,omitempty"`
	GitHub                *GitHubConfig    `yaml:"github,omitempty"`
}

// AgentConfig represents an agent configuration
type AgentConfig struct {
	Name       string `yaml:"name"`
	FullName   string `yaml:"full_name"`
	Role       string `yaml:"role"`
	Model      string `yaml:"model,omitempty"`       // opus, sonnet, haiku
	Color      string `yaml:"color,omitempty"`       // hex color code (e.g., "#FF9933")
	InstanceID string `yaml:"instance_id,omitempty"` // for multi-instance support (e.g., "1", "2")
}

// TODO: Move to config.yaml when multi-instance feature is stabilized
// DefaultMaxInstances is the default maximum number of instances per agent
const DefaultMaxInstances = 3

// GetInstanceKey returns the unique key for this agent instance
// Format: "name" (no instance) or "name-instanceID" (with instance)
func (a *AgentConfig) GetInstanceKey() string {
	if a.InstanceID == "" {
		return a.Name
	}
	return a.Name + "-" + a.InstanceID
}

// DefaultAnalysisMinMessages is the default minimum message count for analysis
const DefaultAnalysisMinMessages = 10

// DefaultConfig returns the default configuration with no agents
// Use "maxam team init" to set up team members
func DefaultConfig() *Config {
	return &Config{
		Version:             "1",
		AnalysisMinMessages: DefaultAnalysisMinMessages,
		Agents:              []AgentConfig{},
	}
}

// ErrNoAgentsConfigured is returned when no agents are configured
var ErrNoAgentsConfigured = fmt.Errorf("no agents configured. Please run 'maxam team init' first")

// HasAgents returns true if at least one agent is configured
func (c *Config) HasAgents() bool {
	return len(c.Agents) > 0
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

	// Override default mode if set
	if override.DefaultMode != "" {
		result.DefaultMode = override.DefaultMode
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

	// Override mention checker enabled if explicitly set
	if override.MentionCheckerEnabled != nil {
		result.MentionCheckerEnabled = override.MentionCheckerEnabled
	}

	// Override log level if set
	if override.LogLevel != "" {
		result.LogLevel = override.LogLevel
	}

	// Override GitHub config if set
	if override.GitHub != nil {
		result.GitHub = override.GitHub
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

// SaveToProject writes configuration to project/.maxam/config.yaml
func SaveToProject(projectDir string, cfg *Config) error {
	configDir := ProjectConfigDir(projectDir)

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("create project config dir: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	configPath := filepath.Join(configDir, "config.yaml")
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("write project config: %w", err)
	}

	return nil
}

// LoadProjectConfig loads only the project-local config (not merged)
func LoadProjectConfig(projectDir string) (*Config, error) {
	projectConfigPath := filepath.Join(ProjectConfigDir(projectDir), "config.yaml")
	data, err := os.ReadFile(projectConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{Version: "1"}, nil
		}
		return nil, fmt.Errorf("read project config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse project config: %w", err)
	}

	return &cfg, nil
}

// GetAgentColor returns the color for the specified agent
// Returns empty string if not found or not set
func (c *Config) GetAgentColor(name string) string {
	for _, agent := range c.Agents {
		if agent.Name == name {
			return agent.Color
		}
	}
	return ""
}

// GetAgentByInstanceKey returns the agent with the specified instance key
// Supports both "name" and "name-instanceID" formats
func (c *Config) GetAgentByInstanceKey(key string) *AgentConfig {
	for i := range c.Agents {
		if c.Agents[i].GetInstanceKey() == key {
			return &c.Agents[i]
		}
	}
	return nil
}

// GetAgentInstances returns all instances of the specified agent name
func (c *Config) GetAgentInstances(name string) []AgentConfig {
	var result []AgentConfig
	for _, agent := range c.Agents {
		if agent.Name == name {
			result = append(result, agent)
		}
	}
	return result
}

// GetAllInstanceKeys returns all unique instance keys for agents
func (c *Config) GetAllInstanceKeys() []string {
	var keys []string
	for _, agent := range c.Agents {
		keys = append(keys, agent.GetInstanceKey())
	}
	return keys
}

// GetUniqueAgentNames returns unique agent names (without instance IDs)
func (c *Config) GetUniqueAgentNames() []string {
	seen := make(map[string]bool)
	var names []string
	for _, agent := range c.Agents {
		if !seen[agent.Name] {
			seen[agent.Name] = true
			names = append(names, agent.Name)
		}
	}
	return names
}

// GetAgentByRole returns the first agent with the specified role
// Returns nil if no agent with the role is found
func (c *Config) GetAgentByRole(role string) *AgentConfig {
	for i := range c.Agents {
		if c.Agents[i].Role == role {
			return &c.Agents[i]
		}
	}
	return nil
}

// GetAgentsByRole returns all agents with the specified role
func (c *Config) GetAgentsByRole(role string) []AgentConfig {
	var result []AgentConfig
	for _, agent := range c.Agents {
		if agent.Role == role {
			result = append(result, agent)
		}
	}
	return result
}

// GetTeamName returns the team name with default fallback
func (c *Config) GetTeamName() string {
	if c.TeamName == "" {
		return "Team"
	}
	return c.TeamName
}

// GetDefaultMode returns the default operating mode with fallback
func (c *Config) GetDefaultMode() Mode {
	if c.DefaultMode == "" {
		return ModeInteractive
	}
	return c.DefaultMode
}

// IsMentionCheckerEnabled returns whether mention checker is enabled (default: true)
func (c *Config) IsMentionCheckerEnabled() bool {
	if c.MentionCheckerEnabled == nil {
		return true // default enabled
	}
	return *c.MentionCheckerEnabled
}

// SetMentionCheckerEnabled sets the mention checker enabled state
func (c *Config) SetMentionCheckerEnabled(enabled bool) {
	c.MentionCheckerEnabled = &enabled
}

// GetLogLevel returns the log level with default fallback ("info")
func (c *Config) GetLogLevel() string {
	if c.LogLevel == "" {
		return "info"
	}
	return c.LogLevel
}

// LoadFromPath loads configuration from a specific file path
func LoadFromPath(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	return &cfg, nil
}

// DefaultHeartbeatInterval is the default heartbeat monitoring interval
const DefaultHeartbeatInterval = "10s"

// DefaultHeartbeatTimeout is the default heartbeat response timeout
const DefaultHeartbeatTimeout = "30s"

// DefaultHeartbeatMaxRetries is the default max restart attempts
const DefaultHeartbeatMaxRetries = 3

// GetHeartbeatConfig returns the heartbeat configuration with defaults
func (c *Config) GetHeartbeatConfig() HeartbeatConfig {
	if c.Heartbeat == nil {
		return HeartbeatConfig{
			Interval:   DefaultHeartbeatInterval,
			Timeout:    DefaultHeartbeatTimeout,
			MaxRetries: DefaultHeartbeatMaxRetries,
		}
	}

	cfg := *c.Heartbeat

	if cfg.Interval == "" {
		cfg.Interval = DefaultHeartbeatInterval
	}
	if cfg.Timeout == "" {
		cfg.Timeout = DefaultHeartbeatTimeout
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = DefaultHeartbeatMaxRetries
	}

	return cfg
}

// GetGitHubConfig returns the GitHub configuration
// Returns nil if not configured
func (c *Config) GetGitHubConfig() *GitHubConfig {
	return c.GitHub
}
