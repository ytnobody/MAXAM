package config

import (
	"embed"
)

//go:embed agents/*/CLAUDE.md
var embeddedAgentsFS embed.FS

// GetEmbeddedAgents returns a map of agent name to CLAUDE.md content
func GetEmbeddedAgents() (map[string]string, error) {
	agents := make(map[string]string)

	for _, name := range DefaultAgents {
		path := "agents/" + name + "/CLAUDE.md"
		data, err := embeddedAgentsFS.ReadFile(path)
		if err != nil {
			continue // Skip if not found
		}
		agents[name] = string(data)
	}

	return agents, nil
}
