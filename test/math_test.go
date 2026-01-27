package test

import "testing"

func TestAdd(t *testing.T) {
	tests := []struct {
		a, b     int
		expected int
	}{
		{1, 2, 3},
		{0, 0, 0},
		{-1, 1, 0},
		{100, -50, 50},
	}

	for _, tt := range tests {
		got := Add(tt.a, tt.b)
		if got != tt.expected {
			t.Errorf("Add(%d, %d) = %d; want %d", tt.a, tt.b, got, tt.expected)
		}
	}
}
