package control

import (
	"testing"
)

func TestParser_Parse(t *testing.T) {
	p := NewParser()

	tests := []struct {
		name     string
		input    string
		wantType CommandType
		wantTgt  string
		wantNil  bool
	}{
		// Individual stop commands
		{
			name:     "stop agent with @mention",
			input:    "@yuki stop",
			wantType: CommandStop,
			wantTgt:  "yuki",
		},
		{
			name:     "stop agent with @mention uppercase",
			input:    "@YUKI STOP",
			wantType: CommandStop,
			wantTgt:  "YUKI",
		},
		{
			name:     "stop agent with mixed case",
			input:    "@Yuki Stop",
			wantType: CommandStop,
			wantTgt:  "Yuki",
		},
		{
			name:     "stop agent with trailing space",
			input:    "@priya stop  ",
			wantType: CommandStop,
			wantTgt:  "priya",
		},
		// Individual resume commands
		{
			name:     "resume agent with @mention",
			input:    "@yuki resume",
			wantType: CommandResume,
			wantTgt:  "yuki",
		},
		{
			name:     "resume agent uppercase",
			input:    "@MEI RESUME",
			wantType: CommandResume,
			wantTgt:  "MEI",
		},
		// Global stop all
		{
			name:     "stop all lowercase",
			input:    "stop all",
			wantType: CommandStopAll,
			wantTgt:  "",
		},
		{
			name:     "stop all uppercase",
			input:    "STOP ALL",
			wantType: CommandStopAll,
			wantTgt:  "",
		},
		{
			name:     "stop all with trailing space",
			input:    "stop all  ",
			wantType: CommandStopAll,
			wantTgt:  "",
		},
		// Global resume all
		{
			name:     "resume all lowercase",
			input:    "resume all",
			wantType: CommandResumeAll,
			wantTgt:  "",
		},
		{
			name:     "resume all mixed case",
			input:    "Resume All",
			wantType: CommandResumeAll,
			wantTgt:  "",
		},
		// Non-control commands (should return nil)
		{
			name:    "regular message",
			input:   "hello world",
			wantNil: true,
		},
		{
			name:    "mention without command",
			input:   "@yuki hello",
			wantNil: true,
		},
		{
			name:    "stop without all or agent",
			input:   "stop",
			wantNil: true,
		},
		{
			name:    "resume without all or agent",
			input:   "resume",
			wantNil: true,
		},
		{
			name:    "partial command",
			input:   "@yuki sto",
			wantNil: true,
		},
		{
			name:    "stop some (not all)",
			input:   "stop some",
			wantNil: true,
		},
		{
			name:    "empty input",
			input:   "",
			wantNil: true,
		},
		{
			name:    "whitespace only",
			input:   "   ",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := p.Parse(tt.input)

			if tt.wantNil {
				if cmd != nil {
					t.Errorf("Parse(%q) = %+v, want nil", tt.input, cmd)
				}
				return
			}

			if cmd == nil {
				t.Errorf("Parse(%q) = nil, want command", tt.input)
				return
			}

			if cmd.Type != tt.wantType {
				t.Errorf("Parse(%q).Type = %v, want %v", tt.input, cmd.Type, tt.wantType)
			}

			if cmd.Target != tt.wantTgt {
				t.Errorf("Parse(%q).Target = %q, want %q", tt.input, cmd.Target, tt.wantTgt)
			}
		})
	}
}

func TestParser_IsControlCommand(t *testing.T) {
	p := NewParser()

	tests := []struct {
		input string
		want  bool
	}{
		{"@yuki stop", true},
		{"@yuki resume", true},
		{"stop all", true},
		{"resume all", true},
		{"hello world", false},
		{"@yuki hello", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := p.IsControlCommand(tt.input)
			if got != tt.want {
				t.Errorf("IsControlCommand(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestCommandType_String(t *testing.T) {
	tests := []struct {
		cmdType CommandType
		want    string
	}{
		{CommandStop, "stop"},
		{CommandResume, "resume"},
		{CommandStopAll, "stop_all"},
		{CommandResumeAll, "resume_all"},
		{CommandUnknown, "unknown"},
		{CommandType(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.cmdType.String()
			if got != tt.want {
				t.Errorf("CommandType(%d).String() = %q, want %q", tt.cmdType, got, tt.want)
			}
		})
	}
}
