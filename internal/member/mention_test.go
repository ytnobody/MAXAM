package member

import (
	"sort"
	"testing"
)

func TestDetectMentions(t *testing.T) {
	// Create members with test data
	m := &Members{
		members: map[string]*Member{
			"mei":   {Name: "mei", FullName: "Mei Chen", Role: "PM"},
			"yuki":  {Name: "yuki", FullName: "Yuki Tanaka", Role: "Backend"},
			"priya": {Name: "priya", FullName: "Priya Sharma", Role: "QA"},
			"alex":  {Name: "alex", FullName: "Alex Johnson", Role: "Frontend"},
			"hana":  {Name: "hana", FullName: "Hana Kim", Role: "Designer"},
		},
		aliases: map[string]string{},
	}

	tests := []struct {
		name     string
		text     string
		expected []string
	}{
		{
			name:     "single mention",
			text:     "Hey @yuki, can you check this?",
			expected: []string{"yuki"},
		},
		{
			name:     "multiple mentions",
			text:     "@alex @hana please collaborate on this",
			expected: []string{"alex", "hana"},
		},
		{
			name:     "no mentions",
			text:     "Just a regular message",
			expected: nil,
		},
		{
			name:     "case insensitive",
			text:     "@YUKI @Priya hello",
			expected: []string{"yuki", "priya"},
		},
		{
			name:     "duplicate mentions",
			text:     "@yuki please @yuki can you help",
			expected: []string{"yuki"},
		},
		{
			name:     "mention in sentence",
			text:     "I think @mei should review this",
			expected: []string{"mei"},
		},
		{
			name:     "three mentions",
			text:     "@yuki @priya @alex all hands meeting",
			expected: []string{"alex", "priya", "yuki"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.DetectMentions(tt.text)

			// Sort for comparison
			sort.Strings(result)
			sort.Strings(tt.expected)

			if len(result) != len(tt.expected) {
				t.Errorf("got %v, want %v", result, tt.expected)
				return
			}

			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("got %v, want %v", result, tt.expected)
					return
				}
			}
		})
	}
}

func TestDetectMentionsWithAliases(t *testing.T) {
	// Create members with aliases (hiragana names)
	m := &Members{
		members: map[string]*Member{
			"mei":   {Name: "mei", FullName: "Mei Chen", Role: "PM", Aliases: []string{"めい"}},
			"yuki":  {Name: "yuki", FullName: "Yuki Tanaka", Role: "Backend", Aliases: []string{"ゆき"}},
			"rin":   {Name: "rin", FullName: "Rin Sato", Role: "Frontend", Aliases: []string{"りん"}},
			"priya": {Name: "priya", FullName: "Priya Sharma", Role: "QA", Aliases: []string{"ぷりや"}},
		},
		aliases: map[string]string{
			"めい":  "mei",
			"ゆき":  "yuki",
			"りん":  "rin",
			"ぷりや": "priya",
		},
	}

	tests := []struct {
		name     string
		text     string
		expected []string
	}{
		{
			name:     "hiragana mention",
			text:     "@ゆき お願いします",
			expected: []string{"yuki"},
		},
		{
			name:     "mixed mentions",
			text:     "@yuki @りん 一緒にやろう",
			expected: []string{"rin", "yuki"},
		},
		{
			name:     "multiple hiragana mentions",
			text:     "@めい @ぷりや レビューお願いします",
			expected: []string{"mei", "priya"},
		},
		{
			name:     "alias and canonical same person",
			text:     "@yuki と @ゆき は同じ人",
			expected: []string{"yuki"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.DetectMentions(tt.text)

			// Sort for comparison
			sort.Strings(result)
			sort.Strings(tt.expected)

			if len(result) != len(tt.expected) {
				t.Errorf("got %v, want %v", result, tt.expected)
				return
			}

			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("got %v, want %v", result, tt.expected)
					return
				}
			}
		})
	}
}

func TestDetectMention(t *testing.T) {
	m := &Members{
		members: map[string]*Member{
			"mei":  {Name: "mei", FullName: "Mei Chen", Role: "PM"},
			"yuki": {Name: "yuki", FullName: "Yuki Tanaka", Role: "Backend"},
		},
		aliases: map[string]string{},
	}

	tests := []struct {
		name string
		text string
		def  string
		want string
	}{
		{
			name: "with mention",
			text: "@yuki check this",
			def:  "mei",
			want: "yuki",
		},
		{
			name: "no mention uses default",
			text: "hello everyone",
			def:  "mei",
			want: "mei",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.DetectMention(tt.text, tt.def)
			if got != tt.want {
				t.Errorf("DetectMention() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetWithAlias(t *testing.T) {
	m := &Members{
		members: map[string]*Member{
			"yuki": {Name: "yuki", FullName: "Yuki Tanaka", Role: "Backend", Aliases: []string{"ゆき"}},
		},
		aliases: map[string]string{
			"ゆき": "yuki",
		},
	}

	tests := []struct {
		name      string
		query     string
		wantFound bool
		wantName  string
	}{
		{"canonical name", "yuki", true, "yuki"},
		{"alias hiragana", "ゆき", true, "yuki"},
		{"unknown", "unknown", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			member, found := m.Get(tt.query)
			if found != tt.wantFound {
				t.Errorf("Get(%q) found = %v, want %v", tt.query, found, tt.wantFound)
				return
			}
			if found && member.Name != tt.wantName {
				t.Errorf("Get(%q).Name = %q, want %q", tt.query, member.Name, tt.wantName)
			}
		})
	}
}

func TestGetFullName(t *testing.T) {
	m := &Members{
		members: map[string]*Member{
			"yuki": {Name: "yuki", FullName: "Yuki Tanaka", Role: "Backend"},
		},
		aliases: map[string]string{},
	}

	tests := []struct {
		shortName string
		want      string
	}{
		{"yuki", "Yuki Tanaka"},
		{"unknown", "Unknown"}, // fallback to capitalized
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.shortName, func(t *testing.T) {
			got := m.GetFullName(tt.shortName)
			if got != tt.want {
				t.Errorf("GetFullName(%q) = %q, want %q", tt.shortName, got, tt.want)
			}
		})
	}
}
