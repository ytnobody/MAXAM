package config

import "testing"

func TestBuiltinDefaultTeam(t *testing.T) {
	team := BuiltinDefaultTeam()

	if team == nil {
		t.Fatal("BuiltinDefaultTeam returned nil")
	}

	if team.Version != "1" {
		t.Errorf("expected version '1', got '%s'", team.Version)
	}

	if team.TeamName != "MAXAM" {
		t.Errorf("expected team name 'MAXAM', got '%s'", team.TeamName)
	}

	// 6 agents expected
	expectedAgents := []struct {
		name     string
		fullName string
		role     string
	}{
		{"mei", "Mei Chen", "PM + Architect"},
		{"yuki", "Yuki Tanaka", "Backend + Infrastructure"},
		{"rin", "Rin Sato", "Frontend + Backend"},
		{"shiori", "Shiori Tanaka", "Test + Documentation"},
		{"priya", "Priya Sharma", "Review + Security + QA"},
		{"amara", "Amara Okonkwo", "Analysis + Legal"},
	}

	if len(team.Agents) != len(expectedAgents) {
		t.Fatalf("expected %d agents, got %d", len(expectedAgents), len(team.Agents))
	}

	for i, expected := range expectedAgents {
		agent := team.Agents[i]
		if agent.Name != expected.name {
			t.Errorf("agent[%d].Name: expected '%s', got '%s'", i, expected.name, agent.Name)
		}
		if agent.FullName != expected.fullName {
			t.Errorf("agent[%d].FullName: expected '%s', got '%s'", i, expected.fullName, agent.FullName)
		}
		if agent.Role != expected.role {
			t.Errorf("agent[%d].Role: expected '%s', got '%s'", i, expected.role, agent.Role)
		}
	}
}

func TestBuiltinDefaultTeamHasAgents(t *testing.T) {
	team := BuiltinDefaultTeam()

	if !team.HasAgents() {
		t.Error("BuiltinDefaultTeam should have agents")
	}
}
