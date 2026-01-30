package ccusage

import (
	"context"
	"testing"
)

func TestFormatCost(t *testing.T) {
	tests := []struct {
		cost     float64
		expected string
	}{
		{0.0, "$0.00"},
		{0.005, "$0.00"},
		{0.01, "$0.01"},
		{0.50, "$0.50"},
		{1.0, "$1.00"},
		{1.23, "$1.23"},
		{12.34, "$12.34"},
		{123.45, "$123.45"},
	}

	for _, tt := range tests {
		result := FormatCost(tt.cost)
		if result != tt.expected {
			t.Errorf("FormatCost(%v) = %q, want %q", tt.cost, result, tt.expected)
		}
	}
}

func TestNewClient(t *testing.T) {
	client := NewClient()
	if client == nil {
		t.Error("NewClient() returned nil")
	}
	if client.timeout == 0 {
		t.Error("Client timeout should not be zero")
	}
}

func TestGetTodayUsage_NoNpx(t *testing.T) {
	// This test checks behavior when ccusage is not available
	// In CI or environments without npx, should return Available=false
	client := NewClient()
	ctx := context.Background()
	usage := client.GetTodayUsage(ctx)

	// We don't know if npx is installed, but the function should not panic
	// If npx is not installed, Available should be false
	// If npx is installed but ccusage fails, Available should be false
	// The important thing is no panic and graceful degradation
	_ = usage // Just ensure no panic
}
