package main

import (
	"reflect"
	"testing"
)

func TestDetectMentions(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "single mention @yuki",
			input:    "@yuki can you help?",
			expected: []string{"yuki"},
		},
		{
			name:     "single mention @mei",
			input:    "@mei please review",
			expected: []string{"mei"},
		},
		{
			name:     "multiple mentions @alex @hana",
			input:    "@alex @hana please introduce yourselves",
			expected: []string{"alex", "hana"},
		},
		{
			name:     "multiple mentions with other text",
			input:    "Hey @yuki and @priya, can you review this?",
			expected: []string{"yuki", "priya"},
		},
		{
			name:     "no mentions defaults to mei",
			input:    "Hello everyone!",
			expected: []string{"mei"},
		},
		{
			name:     "japanese name ゆき",
			input:    "ゆきさん、お願いします",
			expected: []string{"yuki"},
		},
		{
			name:     "japanese name アレックス",
			input:    "アレックスに聞いて",
			expected: []string{"alex"},
		},
		{
			name:     "japanese name ハナ",
			input:    "ハナちゃんどう思う？",
			expected: []string{"hana"},
		},
		{
			name:     "all core agents",
			input:    "@mei @yuki @rin @shiori @priya @amara",
			expected: []string{"mei", "yuki", "rin", "shiori", "priya", "amara"},
		},
		{
			name:     "new agents alex and hana",
			input:    "@alex @hana hello!",
			expected: []string{"alex", "hana"},
		},
		{
			name:     "case insensitive",
			input:    "@YUKI @Priya @alex",
			expected: []string{"yuki", "priya", "alex"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectMentions(tt.input)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("detectMentions(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}
