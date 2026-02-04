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
	// DefaultConfig now returns empty agents (use 'maxam team init' to configure)
	if len(cfg.Agents) != 0 {
		t.Errorf("Agents count = %d, want 0 (empty by default)", len(cfg.Agents))
	}
	if cfg.AnalysisMinMessages != DefaultAnalysisMinMessages {
		t.Errorf("AnalysisMinMessages = %d, want %d", cfg.AnalysisMinMessages, DefaultAnalysisMinMessages)
	}
}

func TestHasAgents(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *Config
		expected bool
	}{
		{
			name:     "空の設定",
			cfg:      DefaultConfig(),
			expected: false,
		},
		{
			name:     "エージェントあり",
			cfg:      &Config{Agents: []AgentConfig{{Name: "test", FullName: "Test", Role: "Test"}}},
			expected: true,
		},
		{
			name:     "空のエージェントリスト",
			cfg:      &Config{Agents: []AgentConfig{}},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.HasAgents()
			if got != tt.expected {
				t.Errorf("HasAgents() = %v, want %v", got, tt.expected)
			}
		})
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

func TestYOLOMode(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *Config
		expected bool
	}{
		{
			name:     "デフォルトはfalse",
			cfg:      &Config{},
			expected: false,
		},
		{
			name:     "明示的にtrue",
			cfg:      &Config{YOLOMode: true},
			expected: true,
		},
		{
			name:     "明示的にfalse",
			cfg:      &Config{YOLOMode: false},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cfg.YOLOMode != tt.expected {
				t.Errorf("YOLOMode = %v, want %v", tt.cfg.YOLOMode, tt.expected)
			}
		})
	}
}

func TestGetWorkersPerAgent(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *Config
		expected int
	}{
		{
			name:     "デフォルト（0）は1",
			cfg:      &Config{},
			expected: DefaultWorkersPerAgent,
		},
		{
			name:     "カスタム値",
			cfg:      &Config{WorkersPerAgent: 3},
			expected: 3,
		},
		{
			name:     "負の値はデフォルト",
			cfg:      &Config{WorkersPerAgent: -1},
			expected: DefaultWorkersPerAgent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.GetWorkersPerAgent()
			if got != tt.expected {
				t.Errorf("GetWorkersPerAgent() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestProjectConfigDir(t *testing.T) {
	projectDir := "/tmp/myproject"
	got := ProjectConfigDir(projectDir)
	expected := "/tmp/myproject/.maxam"
	if got != expected {
		t.Errorf("ProjectConfigDir() = %q, want %q", got, expected)
	}
}

func TestProjectAgentsDir(t *testing.T) {
	projectDir := "/tmp/myproject"
	got := ProjectAgentsDir(projectDir)
	expected := "/tmp/myproject/.maxam/agents"
	if got != expected {
		t.Errorf("ProjectAgentsDir() = %q, want %q", got, expected)
	}
}

func TestProjectClaudeMD(t *testing.T) {
	projectDir := "/tmp/myproject"
	got := ProjectClaudeMD(projectDir)
	expected := "/tmp/myproject/.maxam/CLAUDE.md"
	if got != expected {
		t.Errorf("ProjectClaudeMD() = %q, want %q", got, expected)
	}
}

func TestLoadWithProject(t *testing.T) {
	// Create temp dirs
	tmpHome := t.TempDir()
	projectDir := t.TempDir()

	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", originalHome)

	// Initialize global config
	agents, _ := GetEmbeddedAgents()
	EnsureInitialized(agents)

	// Test 1: No project config - should return global config
	cfg, err := LoadWithProject(projectDir)
	if err != nil {
		t.Fatalf("LoadWithProject() error: %v", err)
	}
	if cfg == nil {
		t.Fatal("LoadWithProject() returned nil")
	}

	// Test 2: With project config - should override
	projectMaxamDir := filepath.Join(projectDir, ".maxam")
	os.MkdirAll(projectMaxamDir, 0755)

	projectConfigContent := `
version: "2"
default_agent: pixel
agents:
  - name: pixel
    full_name: Pixel Artist
    role: Art
  - name: chiptune
    full_name: Chiptune Composer
    role: Music
yolo_mode: true
`
	os.WriteFile(filepath.Join(projectMaxamDir, "config.yaml"), []byte(projectConfigContent), 0644)

	cfg, err = LoadWithProject(projectDir)
	if err != nil {
		t.Fatalf("LoadWithProject() with project config error: %v", err)
	}

	if cfg.Version != "2" {
		t.Errorf("Version = %q, want %q", cfg.Version, "2")
	}
	if cfg.DefaultAgent != "pixel" {
		t.Errorf("DefaultAgent = %q, want %q", cfg.DefaultAgent, "pixel")
	}
	if len(cfg.Agents) != 2 {
		t.Errorf("Agents count = %d, want 2", len(cfg.Agents))
	}
	if !cfg.YOLOMode {
		t.Error("YOLOMode should be true")
	}
}

func TestMergeConfigs(t *testing.T) {
	base := &Config{
		Version:             "1",
		DefaultAgent:        "mei",
		AnalysisMinMessages: 10,
		ContextMode:         ContextModeFull,
		WorkersPerAgent:     1,
		Agents: []AgentConfig{
			{Name: "mei", FullName: "Mei Chen", Role: "PM"},
		},
	}

	override := &Config{
		Version:      "2",
		DefaultAgent: "pixel",
		YOLOMode:     true,
		Agents: []AgentConfig{
			{Name: "pixel", FullName: "Pixel", Role: "Art"},
		},
	}

	merged := mergeConfigs(base, override)

	if merged.Version != "2" {
		t.Errorf("Version = %q, want %q", merged.Version, "2")
	}
	if merged.DefaultAgent != "pixel" {
		t.Errorf("DefaultAgent = %q, want %q", merged.DefaultAgent, "pixel")
	}
	if merged.AnalysisMinMessages != 10 {
		t.Errorf("AnalysisMinMessages = %d, want 10", merged.AnalysisMinMessages)
	}
	if merged.ContextMode != ContextModeFull {
		t.Errorf("ContextMode = %q, want %q", merged.ContextMode, ContextModeFull)
	}
	if !merged.YOLOMode {
		t.Error("YOLOMode should be true")
	}
	if len(merged.Agents) != 1 || merged.Agents[0].Name != "pixel" {
		t.Error("Agents should be overridden to [pixel]")
	}
}

func TestListAgentsWithProject(t *testing.T) {
	tmpHome := t.TempDir()
	projectDir := t.TempDir()

	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", originalHome)

	// Initialize global agents
	agents, _ := GetEmbeddedAgents()
	EnsureInitialized(agents)

	// Create project-local agent
	projectAgentsDir := filepath.Join(projectDir, ".maxam", "agents", "pixel")
	os.MkdirAll(projectAgentsDir, 0755)
	os.WriteFile(filepath.Join(projectAgentsDir, "CLAUDE.md"), []byte("# Pixel Agent"), 0644)

	list, err := ListAgentsWithProject(projectDir)
	if err != nil {
		t.Fatalf("ListAgentsWithProject() error: %v", err)
	}

	// Should include both global agents and project-local agent
	hasPixel := false
	for _, name := range list {
		if name == "pixel" {
			hasPixel = true
			break
		}
	}
	if !hasPixel {
		t.Error("ListAgentsWithProject() should include project-local agent 'pixel'")
	}

	if len(list) < len(DefaultAgents)+1 {
		t.Errorf("ListAgentsWithProject() returned %d agents, want at least %d", len(list), len(DefaultAgents)+1)
	}
}

func TestGetAgentClaudeMDWithProject(t *testing.T) {
	tmpHome := t.TempDir()
	projectDir := t.TempDir()

	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", originalHome)

	// Initialize global agents
	agents, _ := GetEmbeddedAgents()
	EnsureInitialized(agents)

	// Test 1: Global agent
	path, err := GetAgentClaudeMDWithProject(projectDir, "mei")
	if err != nil {
		t.Errorf("GetAgentClaudeMDWithProject(mei) error: %v", err)
	}
	if path == "" {
		t.Error("GetAgentClaudeMDWithProject(mei) returned empty path")
	}

	// Test 2: Project-local agent (should take priority)
	projectAgentDir := filepath.Join(projectDir, ".maxam", "agents", "mei")
	os.MkdirAll(projectAgentDir, 0755)
	projectClaudeMD := filepath.Join(projectAgentDir, "CLAUDE.md")
	os.WriteFile(projectClaudeMD, []byte("# Project Mei"), 0644)

	path, err = GetAgentClaudeMDWithProject(projectDir, "mei")
	if err != nil {
		t.Errorf("GetAgentClaudeMDWithProject(mei) with project error: %v", err)
	}
	if path != projectClaudeMD {
		t.Errorf("GetAgentClaudeMDWithProject(mei) = %q, want %q (project should take priority)", path, projectClaudeMD)
	}
}

func TestGetAgentDirWithProject(t *testing.T) {
	tmpHome := t.TempDir()
	projectDir := t.TempDir()

	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", originalHome)

	// Initialize global agents
	agents, _ := GetEmbeddedAgents()
	EnsureInitialized(agents)

	// Test 1: Global agent
	dir, err := GetAgentDirWithProject(projectDir, "yuki")
	if err != nil {
		t.Errorf("GetAgentDirWithProject(yuki) error: %v", err)
	}
	if dir == "" {
		t.Error("GetAgentDirWithProject(yuki) returned empty dir")
	}

	// Test 2: Project-local agent takes priority
	projectAgentDir := filepath.Join(projectDir, ".maxam", "agents", "yuki")
	os.MkdirAll(projectAgentDir, 0755)
	os.WriteFile(filepath.Join(projectAgentDir, "CLAUDE.md"), []byte("# Project Yuki"), 0644)

	dir, err = GetAgentDirWithProject(projectDir, "yuki")
	if err != nil {
		t.Errorf("GetAgentDirWithProject(yuki) with project error: %v", err)
	}
	if dir != projectAgentDir {
		t.Errorf("GetAgentDirWithProject(yuki) = %q, want %q", dir, projectAgentDir)
	}
}

func TestAgentConfigColor(t *testing.T) {
	tests := []struct {
		name     string
		config   AgentConfig
		hasColor bool
		color    string
	}{
		{
			name:     "色未指定",
			config:   AgentConfig{Name: "test", FullName: "Test Agent", Role: "test"},
			hasColor: false,
		},
		{
			name:     "色指定あり",
			config:   AgentConfig{Name: "test", FullName: "Test Agent", Role: "test", Color: "#FF9933"},
			hasColor: true,
			color:    "#FF9933",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.hasColor && tt.config.Color != tt.color {
				t.Errorf("Color = %q, want %q", tt.config.Color, tt.color)
			}
			if !tt.hasColor && tt.config.Color != "" {
				t.Error("Expected color to be empty")
			}
		})
	}
}

func TestGetAgentColor(t *testing.T) {
	cfg := &Config{
		Agents: []AgentConfig{
			{Name: "mei", Color: "#FFB7C5"},
			{Name: "yuki", Color: "#87CEEB"},
			{Name: "priya", Color: ""},
		},
	}

	tests := []struct {
		name     string
		agent    string
		expected string
	}{
		{
			name:     "色が設定されているエージェント",
			agent:    "mei",
			expected: "#FFB7C5",
		},
		{
			name:     "別の色が設定されているエージェント",
			agent:    "yuki",
			expected: "#87CEEB",
		},
		{
			name:     "色が空のエージェント",
			agent:    "priya",
			expected: "",
		},
		{
			name:     "存在しないエージェント",
			agent:    "unknown",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cfg.GetAgentColor(tt.agent)
			if got != tt.expected {
				t.Errorf("GetAgentColor(%q) = %q, want %q", tt.agent, got, tt.expected)
			}
		})
	}
}

func TestGetAgentByRole(t *testing.T) {
	cfg := &Config{
		Agents: []AgentConfig{
			{Name: "dev1", FullName: "Developer One", Role: "developer"},
			{Name: "dev2", FullName: "Developer Two", Role: "developer"},
			{Name: "rev", FullName: "Reviewer", Role: "reviewer"},
			{Name: "pm", FullName: "Project Manager", Role: "pm"},
		},
	}

	tests := []struct {
		name         string
		role         string
		expectedName string
		shouldFind   bool
	}{
		{
			name:         "developer役割のエージェントを取得（最初のもの）",
			role:         "developer",
			expectedName: "dev1",
			shouldFind:   true,
		},
		{
			name:         "reviewer役割のエージェントを取得",
			role:         "reviewer",
			expectedName: "rev",
			shouldFind:   true,
		},
		{
			name:         "pm役割のエージェントを取得",
			role:         "pm",
			expectedName: "pm",
			shouldFind:   true,
		},
		{
			name:         "存在しない役割",
			role:         "unknown",
			expectedName: "",
			shouldFind:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cfg.GetAgentByRole(tt.role)
			if tt.shouldFind {
				if got == nil {
					t.Errorf("GetAgentByRole(%q) returned nil, want agent", tt.role)
					return
				}
				if got.Name != tt.expectedName {
					t.Errorf("GetAgentByRole(%q).Name = %q, want %q", tt.role, got.Name, tt.expectedName)
				}
			} else {
				if got != nil {
					t.Errorf("GetAgentByRole(%q) = %v, want nil", tt.role, got)
				}
			}
		})
	}
}

func TestGetAgentsByRole(t *testing.T) {
	cfg := &Config{
		Agents: []AgentConfig{
			{Name: "dev1", Role: "developer"},
			{Name: "dev2", Role: "developer"},
			{Name: "rev", Role: "reviewer"},
		},
	}

	tests := []struct {
		name          string
		role          string
		expectedCount int
	}{
		{
			name:          "developer役割は2人",
			role:          "developer",
			expectedCount: 2,
		},
		{
			name:          "reviewer役割は1人",
			role:          "reviewer",
			expectedCount: 1,
		},
		{
			name:          "存在しない役割は0人",
			role:          "unknown",
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cfg.GetAgentsByRole(tt.role)
			if len(got) != tt.expectedCount {
				t.Errorf("GetAgentsByRole(%q) returned %d agents, want %d", tt.role, len(got), tt.expectedCount)
			}
		})
	}
}

func TestGetTeamName(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *Config
		expected string
	}{
		{
			name:     "デフォルト（空）はTeam",
			cfg:      &Config{},
			expected: "Team",
		},
		{
			name:     "カスタムチーム名",
			cfg:      &Config{TeamName: "MAXAM"},
			expected: "MAXAM",
		},
		{
			name:     "別のチーム名",
			cfg:      &Config{TeamName: "Alpha Team"},
			expected: "Alpha Team",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.GetTeamName()
			if got != tt.expected {
				t.Errorf("GetTeamName() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestEnsureProjectInitialized(t *testing.T) {
	projectDir := t.TempDir()

	// 初期化前は.maxamが存在しない
	configDir := ProjectConfigDir(projectDir)
	if _, err := os.Stat(configDir); !os.IsNotExist(err) {
		t.Error(".maxam should not exist before initialization")
	}

	// 初期化実行
	if err := EnsureProjectInitialized(projectDir); err != nil {
		t.Fatalf("EnsureProjectInitialized() error: %v", err)
	}

	// config.yamlが作成されている
	configPath := filepath.Join(configDir, "config.yaml")
	if _, err := os.Stat(configPath); err != nil {
		t.Errorf("config.yaml not created: %v", err)
	}

	// CLAUDE.mdが作成されている
	claudeMDPath := filepath.Join(configDir, "CLAUDE.md")
	if _, err := os.Stat(claudeMDPath); err != nil {
		t.Errorf("CLAUDE.md not created: %v", err)
	}

	// 内容確認（config.yaml）
	configContent, _ := os.ReadFile(configPath)
	if !contains(string(configContent), "version:") {
		t.Error("config.yaml should contain version")
	}

	// 内容確認（CLAUDE.md）
	claudeContent, _ := os.ReadFile(claudeMDPath)
	if !contains(string(claudeContent), "## Team Members") {
		t.Error("CLAUDE.md should contain Team Members section")
	}
}

func TestEnsureProjectInitializedNoOverwrite(t *testing.T) {
	projectDir := t.TempDir()

	// 既存ファイルを作成
	configDir := ProjectConfigDir(projectDir)
	os.MkdirAll(configDir, 0755)

	existingConfig := "# existing config\nversion: \"99\"\n"
	existingClaudeMD := "# existing CLAUDE.md\n"

	os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(existingConfig), 0644)
	os.WriteFile(filepath.Join(configDir, "CLAUDE.md"), []byte(existingClaudeMD), 0644)

	// 初期化実行
	if err := EnsureProjectInitialized(projectDir); err != nil {
		t.Fatalf("EnsureProjectInitialized() error: %v", err)
	}

	// 既存ファイルが上書きされていない
	configContent, _ := os.ReadFile(filepath.Join(configDir, "config.yaml"))
	if string(configContent) != existingConfig {
		t.Errorf("config.yaml was overwritten: got %q, want %q", string(configContent), existingConfig)
	}

	claudeContent, _ := os.ReadFile(filepath.Join(configDir, "CLAUDE.md"))
	if string(claudeContent) != existingClaudeMD {
		t.Errorf("CLAUDE.md was overwritten: got %q, want %q", string(claudeContent), existingClaudeMD)
	}
}

func TestIsProjectInitialized(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(string)
		expected  bool
	}{
		{
			name: "初期化されていない",
			setup: func(dir string) {
				// 何もしない
			},
			expected: false,
		},
		{
			name: "初期化済み",
			setup: func(dir string) {
				EnsureProjectInitialized(dir)
			},
			expected: true,
		},
		{
			name: ".maxamディレクトリのみ（config.yamlなし）",
			setup: func(dir string) {
				os.MkdirAll(ProjectConfigDir(dir), 0755)
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectDir := t.TempDir()
			tt.setup(projectDir)

			got := IsProjectInitialized(projectDir)
			if got != tt.expected {
				t.Errorf("IsProjectInitialized() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestAgentConfigGetInstanceKey(t *testing.T) {
	tests := []struct {
		name     string
		config   AgentConfig
		expected string
	}{
		{
			name:     "インスタンスIDなし",
			config:   AgentConfig{Name: "yuki"},
			expected: "yuki",
		},
		{
			name:     "インスタンスIDあり",
			config:   AgentConfig{Name: "yuki", InstanceID: "1"},
			expected: "yuki-1",
		},
		{
			name:     "インスタンスID=2",
			config:   AgentConfig{Name: "yuki", InstanceID: "2"},
			expected: "yuki-2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.GetInstanceKey()
			if got != tt.expected {
				t.Errorf("GetInstanceKey() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestGetAgentByInstanceKey(t *testing.T) {
	cfg := &Config{
		Agents: []AgentConfig{
			{Name: "yuki", FullName: "Yuki Tanaka"},
			{Name: "yuki", FullName: "Yuki Tanaka", InstanceID: "1"},
			{Name: "yuki", FullName: "Yuki Tanaka", InstanceID: "2"},
			{Name: "mei", FullName: "Mei Chen"},
		},
	}

	tests := []struct {
		name       string
		key        string
		shouldFind bool
		wantName   string
		wantID     string
	}{
		{
			name:       "インスタンスIDなしで検索",
			key:        "yuki",
			shouldFind: true,
			wantName:   "yuki",
			wantID:     "",
		},
		{
			name:       "インスタンスID=1で検索",
			key:        "yuki-1",
			shouldFind: true,
			wantName:   "yuki",
			wantID:     "1",
		},
		{
			name:       "インスタンスID=2で検索",
			key:        "yuki-2",
			shouldFind: true,
			wantName:   "yuki",
			wantID:     "2",
		},
		{
			name:       "存在しないキー",
			key:        "yuki-3",
			shouldFind: false,
		},
		{
			name:       "別のエージェント",
			key:        "mei",
			shouldFind: true,
			wantName:   "mei",
			wantID:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cfg.GetAgentByInstanceKey(tt.key)
			if tt.shouldFind {
				if got == nil {
					t.Errorf("GetAgentByInstanceKey(%q) returned nil", tt.key)
					return
				}
				if got.Name != tt.wantName {
					t.Errorf("Name = %q, want %q", got.Name, tt.wantName)
				}
				if got.InstanceID != tt.wantID {
					t.Errorf("InstanceID = %q, want %q", got.InstanceID, tt.wantID)
				}
			} else {
				if got != nil {
					t.Errorf("GetAgentByInstanceKey(%q) = %v, want nil", tt.key, got)
				}
			}
		})
	}
}

func TestGetAgentInstances(t *testing.T) {
	cfg := &Config{
		Agents: []AgentConfig{
			{Name: "yuki", FullName: "Yuki Tanaka"},
			{Name: "yuki", FullName: "Yuki Tanaka", InstanceID: "1"},
			{Name: "yuki", FullName: "Yuki Tanaka", InstanceID: "2"},
			{Name: "mei", FullName: "Mei Chen"},
		},
	}

	tests := []struct {
		name          string
		agentName     string
		expectedCount int
	}{
		{
			name:          "yukiは3インスタンス",
			agentName:     "yuki",
			expectedCount: 3,
		},
		{
			name:          "meiは1インスタンス",
			agentName:     "mei",
			expectedCount: 1,
		},
		{
			name:          "存在しないエージェントは0",
			agentName:     "unknown",
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cfg.GetAgentInstances(tt.agentName)
			if len(got) != tt.expectedCount {
				t.Errorf("GetAgentInstances(%q) returned %d, want %d", tt.agentName, len(got), tt.expectedCount)
			}
		})
	}
}

func TestGetAllInstanceKeys(t *testing.T) {
	cfg := &Config{
		Agents: []AgentConfig{
			{Name: "yuki"},
			{Name: "yuki", InstanceID: "1"},
			{Name: "mei"},
		},
	}

	keys := cfg.GetAllInstanceKeys()
	if len(keys) != 3 {
		t.Errorf("GetAllInstanceKeys() returned %d keys, want 3", len(keys))
	}

	expected := map[string]bool{"yuki": true, "yuki-1": true, "mei": true}
	for _, key := range keys {
		if !expected[key] {
			t.Errorf("Unexpected key: %q", key)
		}
	}
}

func TestGetUniqueAgentNames(t *testing.T) {
	cfg := &Config{
		Agents: []AgentConfig{
			{Name: "yuki"},
			{Name: "yuki", InstanceID: "1"},
			{Name: "yuki", InstanceID: "2"},
			{Name: "mei"},
		},
	}

	names := cfg.GetUniqueAgentNames()
	if len(names) != 2 {
		t.Errorf("GetUniqueAgentNames() returned %d names, want 2", len(names))
	}

	expected := map[string]bool{"yuki": true, "mei": true}
	for _, name := range names {
		if !expected[name] {
			t.Errorf("Unexpected name: %q", name)
		}
	}
}

func TestDefaultMaxInstances(t *testing.T) {
	if DefaultMaxInstances != 3 {
		t.Errorf("DefaultMaxInstances = %d, want 3", DefaultMaxInstances)
	}
}
