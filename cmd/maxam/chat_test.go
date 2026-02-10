package main

import "testing"

func TestParseChatFlags(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantAgent   string
		wantDaemon  bool
		wantMention bool
	}{
		{
			name:        "agent only",
			args:        []string{"maxam", "chat", "yuki"},
			wantAgent:   "yuki",
			wantDaemon:  false,
			wantMention: true,
		},
		{
			name:        "agent with daemon",
			args:        []string{"maxam", "chat", "team", "--daemon"},
			wantAgent:   "team",
			wantDaemon:  true,
			wantMention: false, // daemon mode: default off
		},
		{
			name:        "daemon with mention-check explicitly enabled",
			args:        []string{"maxam", "chat", "team", "--daemon", "--mention-check"},
			wantAgent:   "team",
			wantDaemon:  true,
			wantMention: true,
		},
		{
			name:        "daemon with mention-check=true",
			args:        []string{"maxam", "chat", "team", "--daemon", "--mention-check=true"},
			wantAgent:   "team",
			wantDaemon:  true,
			wantMention: true,
		},
		{
			name:        "daemon with mention-check=false",
			args:        []string{"maxam", "chat", "team", "--daemon", "--mention-check=false"},
			wantAgent:   "team",
			wantDaemon:  true,
			wantMention: false,
		},
		{
			name:        "non-daemon with no-mention-check",
			args:        []string{"maxam", "chat", "yuki", "--no-mention-check"},
			wantAgent:   "yuki",
			wantDaemon:  false,
			wantMention: false,
		},
		{
			name:        "uppercase agent name normalized",
			args:        []string{"maxam", "chat", "YUKI"},
			wantAgent:   "yuki",
			wantDaemon:  false,
			wantMention: true,
		},
		{
			name:        "empty args",
			args:        []string{"maxam", "chat"},
			wantAgent:   "",
			wantDaemon:  false,
			wantMention: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags := parseChatFlags(tt.args)

			if flags.agentName != tt.wantAgent {
				t.Errorf("agentName = %q, want %q", flags.agentName, tt.wantAgent)
			}
			if flags.daemon != tt.wantDaemon {
				t.Errorf("daemon = %v, want %v", flags.daemon, tt.wantDaemon)
			}
			if flags.mentionCheck != tt.wantMention {
				t.Errorf("mentionCheck = %v, want %v", flags.mentionCheck, tt.wantMention)
			}
		})
	}
}
