package main

import (
	"testing"
)

func TestIsNoIssueResponse(t *testing.T) {
	tests := []struct {
		name     string
		response string
		want     bool
	}{
		{
			name:     "特になし",
			response: "特になし",
			want:     true,
		},
		{
			name:     "特に問題なし",
			response: "特に問題なし。",
			want:     true,
		},
		{
			name:     "特に気になる点はない",
			response: "確認したけど、特に気になる点はないね。",
			want:     true,
		},
		{
			name:     "問題なし",
			response: "問題なし",
			want:     true,
		},
		{
			name:     "英語 nothing to report",
			response: "Nothing to report.",
			want:     true,
		},
		{
			name:     "英語 no issues",
			response: "No issues found.",
			want:     true,
		},
		{
			name:     "気づきあり",
			response: "差し戻しが2回あったね。Issueにしておく？",
			want:     false,
		},
		{
			name:     "コメントあり",
			response: "要件の確認が少し曖昧だったかも。",
			want:     false,
		},
		{
			name:     "空文字",
			response: "",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isNoIssueResponse(tt.response)
			if got != tt.want {
				t.Errorf("isNoIssueResponse(%q) = %v, want %v", tt.response, got, tt.want)
			}
		})
	}
}
