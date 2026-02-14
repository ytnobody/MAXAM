package config

// BuiltinDefaultTeam returns the built-in default team configuration.
// This is the only place where team members are hardcoded.
// After maxam init, users customize the generated config.yaml.
func BuiltinDefaultTeam() *Config {
	return &Config{
		Version:  "1",
		TeamName: "MAXAM",
		Agents: []AgentConfig{
			{
				Name:     "mei",
				FullName: "Mei Chen",
				Role:     "PM + Architect",
			},
			{
				Name:     "yuki",
				FullName: "Yuki Tanaka",
				Role:     "Backend + Infrastructure",
			},
			{
				Name:     "rin",
				FullName: "Rin Sato",
				Role:     "Frontend + Backend",
			},
			{
				Name:     "shiori",
				FullName: "Shiori Tanaka",
				Role:     "Test + Documentation",
			},
			{
				Name:     "priya",
				FullName: "Priya Sharma",
				Role:     "Review + Security + QA",
			},
			{
				Name:     "amara",
				FullName: "Amara Okonkwo",
				Role:     "Analysis + Legal",
			},
		},
	}
}
