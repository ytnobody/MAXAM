package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNew(t *testing.T) {
	cfg := Config{
		Name:        "Test Agent",
		Role:        "test",
		WorkDir:     "/tmp/test",
		ClaudeMDDir: "/tmp/test/agents/test",
	}

	a := New(cfg)

	if a.Name != cfg.Name {
		t.Errorf("Name = %q, want %q", a.Name, cfg.Name)
	}
	if a.Role != cfg.Role {
		t.Errorf("Role = %q, want %q", a.Role, cfg.Role)
	}
	if a.WorkDir != cfg.WorkDir {
		t.Errorf("WorkDir = %q, want %q", a.WorkDir, cfg.WorkDir)
	}
	if a.ClaudeMDDir != cfg.ClaudeMDDir {
		t.Errorf("ClaudeMDDir = %q, want %q", a.ClaudeMDDir, cfg.ClaudeMDDir)
	}
}

func TestNewRunner(t *testing.T) {
	r := NewRunner("Test", "/work", "/agents/test")

	if r.Name != "Test" {
		t.Errorf("Name = %q, want %q", r.Name, "Test")
	}
	if r.WorkDir != "/work" {
		t.Errorf("WorkDir = %q, want %q", r.WorkDir, "/work")
	}
	if r.ClaudeMDDir != "/agents/test" {
		t.Errorf("ClaudeMDDir = %q, want %q", r.ClaudeMDDir, "/agents/test")
	}
	if r.Timeout == 0 {
		t.Error("Timeout should have default value")
	}
}

func TestNewAgents(t *testing.T) {
	// Create temp directory with test agent structure
	tmpDir, err := os.MkdirTemp("", "agents_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create .maxam directory structure
	maxamDir := filepath.Join(tmpDir, ".maxam")
	agentsDir := filepath.Join(maxamDir, "agents")

	// Create test agents
	testAgents := []string{"alice", "bob"}
	for _, name := range testAgents {
		agentDir := filepath.Join(agentsDir, name)
		if err := os.MkdirAll(agentDir, 0755); err != nil {
			t.Fatal(err)
		}
		// Create CLAUDE.md for each agent
		claudeMD := filepath.Join(agentDir, "CLAUDE.md")
		if err := os.WriteFile(claudeMD, []byte("# "+name), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Create config.yaml with test agents
	configContent := `version: "1"
default_agent: alice
agents:
  - name: alice
    full_name: Alice Test
    role: tester
  - name: bob
    full_name: Bob Test
    role: reviewer
`
	if err := os.WriteFile(filepath.Join(maxamDir, "config.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Test with project directory containing config
	agents := NewAgents(tmpDir)

	// Check test agents exist
	for _, name := range testAgents {
		if _, ok := agents.Get(name); !ok {
			t.Errorf("agent %q not found", name)
		}
	}

	// Check that non-existent agent returns false
	if _, ok := agents.Get("nonexistent"); ok {
		t.Error("Get(nonexistent) should return false")
	}
}

func TestAgentsGet(t *testing.T) {
	// Create temp directory with test agent structure
	tmpDir, err := os.MkdirTemp("", "agents_get_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create .maxam directory structure
	maxamDir := filepath.Join(tmpDir, ".maxam")
	agentsDir := filepath.Join(maxamDir, "agents")
	aliceDir := filepath.Join(agentsDir, "alice")
	if err := os.MkdirAll(aliceDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create CLAUDE.md for alice
	if err := os.WriteFile(filepath.Join(aliceDir, "CLAUDE.md"), []byte("# Alice"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create config.yaml
	configContent := `version: "1"
default_agent: alice
agents:
  - name: alice
    full_name: Alice Test
    role: tester
`
	if err := os.WriteFile(filepath.Join(maxamDir, "config.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	agents := NewAgents(tmpDir)

	// Existing agent
	r, ok := agents.Get("alice")
	if !ok {
		t.Error("Get(alice) returned false")
	}
	if r == nil {
		t.Error("Get(alice) returned nil runner")
	}

	// Non-existing agent
	_, ok = agents.Get("nonexistent")
	if ok {
		t.Error("Get(nonexistent) should return false")
	}
}

func TestBuildSystemPrompt(t *testing.T) {
	// Create temp directory structure
	tmpDir, err := os.MkdirTemp("", "agent_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create shared CLAUDE.md
	sharedContent := "# Shared Rules"
	if err := os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte(sharedContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create agent-specific CLAUDE.md
	agentDir := filepath.Join(tmpDir, "agents", "test")
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		t.Fatal(err)
	}
	agentContent := "# Agent Rules"
	if err := os.WriteFile(filepath.Join(agentDir, "CLAUDE.md"), []byte(agentContent), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("with both files", func(t *testing.T) {
		r := NewRunner("Test", tmpDir, agentDir)
		prompt, err := r.buildSystemPrompt()
		if err != nil {
			t.Fatal(err)
		}

		if prompt == "" {
			t.Error("prompt is empty")
		}
		// Should contain embedded rules
		if !contains(prompt, "MAXAM") {
			t.Errorf("prompt should contain embedded rules")
		}
		// Should contain both contents
		if !contains(prompt, sharedContent) {
			t.Errorf("prompt should contain shared content")
		}
		if !contains(prompt, agentContent) {
			t.Errorf("prompt should contain agent content")
		}
	})

	t.Run("with only shared file", func(t *testing.T) {
		r := NewRunner("Test", tmpDir, "")
		prompt, err := r.buildSystemPrompt()
		if err != nil {
			t.Fatal(err)
		}

		if !contains(prompt, sharedContent) {
			t.Errorf("prompt should contain shared content")
		}
	})

	t.Run("with no external files", func(t *testing.T) {
		emptyDir, _ := os.MkdirTemp("", "empty")
		defer os.RemoveAll(emptyDir)

		r := NewRunner("Test", emptyDir, "")
		prompt, err := r.buildSystemPrompt()
		// Should succeed with embedded rules only
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !contains(prompt, "MAXAM") {
			t.Errorf("prompt should contain embedded rules")
		}
	})
}

func TestSendTaskWithoutStart(t *testing.T) {
	a := New(Config{Name: "Test"})

	_, err := a.SendTask("test")
	if err == nil {
		t.Error("expected error when agent not started")
	}
}

func TestCallToolWithoutStart(t *testing.T) {
	a := New(Config{Name: "Test"})

	_, err := a.CallTool("test", nil)
	if err == nil {
		t.Error("expected error when agent not started")
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
