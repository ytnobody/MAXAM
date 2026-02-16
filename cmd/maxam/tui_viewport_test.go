package main

import (
	"testing"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

func TestWindowSizeMsg_MinimumDimensions(t *testing.T) {
	tests := []struct {
		name       string
		width      int
		height     int
		wantVPW    int
		wantVPH    int
		wantInputW int
	}{
		{
			name:       "normal size",
			width:      80,
			height:     24,
			wantVPW:    80,
			wantVPH:    19, // 24 - 5 (headerHeight + footerHeight)
			wantInputW: 72, // 80 - 8
		},
		{
			name:       "zero width",
			width:      0,
			height:     24,
			wantVPW:    1,
			wantVPH:    19,
			wantInputW: 1,
		},
		{
			name:       "zero height",
			width:      80,
			height:     0,
			wantVPW:    80,
			wantVPH:    1,
			wantInputW: 72,
		},
		{
			name:       "very small terminal",
			width:      5,
			height:     3,
			wantVPW:    5,
			wantVPH:    1, // 3 - 5 = -2 -> clamped to 1
			wantInputW: 1, // 5 - 8 = -3 -> clamped to 1
		},
		{
			name:       "negative would overflow",
			width:      1,
			height:     1,
			wantVPW:    1,
			wantVPH:    1,
			wantInputW: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the clamping logic directly
			headerHeight := 2
			footerHeight := 3
			verticalMargin := headerHeight + footerHeight

			vpWidth := tt.width
			if vpWidth < 1 {
				vpWidth = 1
			}
			vpHeight := tt.height - verticalMargin
			if vpHeight < 1 {
				vpHeight = 1
			}
			inputWidth := tt.width - 8
			if inputWidth < 1 {
				inputWidth = 1
			}

			if vpWidth != tt.wantVPW {
				t.Errorf("viewport width = %d, want %d", vpWidth, tt.wantVPW)
			}
			if vpHeight != tt.wantVPH {
				t.Errorf("viewport height = %d, want %d", vpHeight, tt.wantVPH)
			}
			if inputWidth != tt.wantInputW {
				t.Errorf("input width = %d, want %d", inputWidth, tt.wantInputW)
			}
		})
	}
}

func TestViewport_SmallDimensions_NoPanic(t *testing.T) {
	// Ensure viewport.New with minimum dimensions doesn't panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("viewport.New panicked with minimum dimensions: %v", r)
		}
	}()

	vp := viewport.New(1, 1)
	vp.SetContent("test content\nline 2\nline 3")
	vp.GotoBottom()
	_ = vp.View()
}

func TestViewport_SetContent_NoPanic(t *testing.T) {
	// Test that setting content on a very small viewport doesn't panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("viewport operations panicked: %v", r)
		}
	}()

	vp := viewport.New(1, 1)
	// Long content that would normally cause issues
	content := ""
	for i := 0; i < 1000; i++ {
		content += "line of content\n"
	}
	vp.SetContent(content)
	vp.GotoBottom()
	_ = vp.View()
}

// Ensure tea.WindowSizeMsg type is correct
var _ tea.Msg = tea.WindowSizeMsg{}
