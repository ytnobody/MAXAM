package logger

import "testing"

func TestProjectNameFromPath(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/home/ubuntu/my-project", "home_ubuntu_my-project"},
		{"/home/ubuntu/MAXAM", "home_ubuntu_MAXAM"},
		{"/var/www/app", "var_www_app"},
		{"/single", "single"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := ProjectNameFromPath(tt.path)
			if got != tt.expected {
				t.Errorf("ProjectNameFromPath(%q) = %q, want %q", tt.path, got, tt.expected)
			}
		})
	}
}
