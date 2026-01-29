package logger

import "testing"

func TestProjectNameFromPath(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/home/user/my-project", "home_user_my-project"},
		{"/home/user/MAXAM", "home_user_MAXAM"},
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
