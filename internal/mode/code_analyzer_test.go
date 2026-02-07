package mode

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodeAnalyzer_findTodos(t *testing.T) {
	// Create a temp directory with test files
	tmpDir := t.TempDir()

	// Create a Go file with TODOs
	goFile := filepath.Join(tmpDir, "main.go")
	goContent := `package main

// TODO: implement this function
func main() {
	// FIXME: this is broken
	println("hello")
	// HACK: temporary workaround
	// XXX: need to refactor
}
`
	if err := os.WriteFile(goFile, []byte(goContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a JS file with TODOs
	jsFile := filepath.Join(tmpDir, "app.js")
	jsContent := `// TODO: add error handling
function foo() {
	// fixme: lowercase should work too
	return 42;
}
`
	if err := os.WriteFile(jsFile, []byte(jsContent), 0644); err != nil {
		t.Fatal(err)
	}

	analyzer := NewCodeAnalyzer(tmpDir)
	todos, err := analyzer.findTodos()
	if err != nil {
		t.Fatalf("findTodos() error = %v", err)
	}

	if len(todos) != 6 {
		t.Errorf("Expected 6 TODOs, got %d", len(todos))
		for _, todo := range todos {
			t.Logf("  %s:%d [%s] %s", todo.File, todo.Line, todo.Type, todo.Content)
		}
	}

	// Check types are detected correctly
	typeCount := make(map[string]int)
	for _, todo := range todos {
		typeCount[todo.Type]++
	}

	if typeCount["TODO"] != 2 {
		t.Errorf("Expected 2 TODOs, got %d", typeCount["TODO"])
	}
	if typeCount["FIXME"] != 2 {
		t.Errorf("Expected 2 FIXMEs, got %d", typeCount["FIXME"])
	}
	if typeCount["HACK"] != 1 {
		t.Errorf("Expected 1 HACK, got %d", typeCount["HACK"])
	}
	if typeCount["XXX"] != 1 {
		t.Errorf("Expected 1 XXX, got %d", typeCount["XXX"])
	}
}

func TestCodeAnalyzer_findLongFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a short file
	shortFile := filepath.Join(tmpDir, "short.go")
	shortContent := strings.Repeat("// line\n", 50)
	if err := os.WriteFile(shortFile, []byte(shortContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a long file (over 500 lines)
	longFile := filepath.Join(tmpDir, "long.go")
	longContent := strings.Repeat("// line\n", 600)
	if err := os.WriteFile(longFile, []byte(longContent), 0644); err != nil {
		t.Fatal(err)
	}

	analyzer := NewCodeAnalyzer(tmpDir)
	longFiles, err := analyzer.findLongFiles(500)
	if err != nil {
		t.Fatalf("findLongFiles() error = %v", err)
	}

	if len(longFiles) != 1 {
		t.Errorf("Expected 1 long file, got %d", len(longFiles))
	}

	if len(longFiles) > 0 && longFiles[0].File != "long.go" {
		t.Errorf("Expected long.go, got %s", longFiles[0].File)
	}
}

func TestCodeAnalyzer_Analyze(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file with TODOs
	goFile := filepath.Join(tmpDir, "main.go")
	goContent := `package main

// TODO: implement
func main() {}
`
	if err := os.WriteFile(goFile, []byte(goContent), 0644); err != nil {
		t.Fatal(err)
	}

	analyzer := NewCodeAnalyzer(tmpDir)
	result, err := analyzer.Analyze()
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	if len(result.Todos) != 1 {
		t.Errorf("Expected 1 TODO, got %d", len(result.Todos))
	}

	if result.Summary == "" {
		t.Error("Expected non-empty summary")
	}

	// Check summary contains expected content
	if !strings.Contains(result.Summary, "TODO") {
		t.Error("Summary should contain TODO")
	}
}

func TestCodeAnalyzer_IgnoresDirs(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a normal file
	normalFile := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(normalFile, []byte("// TODO: normal\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create node_modules (should be ignored)
	nodeModules := filepath.Join(tmpDir, "node_modules")
	if err := os.MkdirAll(nodeModules, 0755); err != nil {
		t.Fatal(err)
	}
	nodeFile := filepath.Join(nodeModules, "package.js")
	if err := os.WriteFile(nodeFile, []byte("// TODO: in node_modules\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create vendor (should be ignored)
	vendor := filepath.Join(tmpDir, "vendor")
	if err := os.MkdirAll(vendor, 0755); err != nil {
		t.Fatal(err)
	}
	vendorFile := filepath.Join(vendor, "dep.go")
	if err := os.WriteFile(vendorFile, []byte("// TODO: in vendor\n"), 0644); err != nil {
		t.Fatal(err)
	}

	analyzer := NewCodeAnalyzer(tmpDir)
	todos, err := analyzer.findTodos()
	if err != nil {
		t.Fatalf("findTodos() error = %v", err)
	}

	// Should only find the TODO in normal file
	if len(todos) != 1 {
		t.Errorf("Expected 1 TODO (ignoring node_modules and vendor), got %d", len(todos))
		for _, todo := range todos {
			t.Logf("  Found: %s", todo.File)
		}
	}
}

func TestIsCodeFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"main.go", true},
		{"app.js", true},
		{"style.css", true},
		{"README.md", true},
		{"config.yaml", true},
		{"image.png", false},
		{"binary.exe", false},
		{"data.bin", false},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			result := isCodeFile(tc.path)
			if result != tc.expected {
				t.Errorf("isCodeFile(%q) = %v, want %v", tc.path, result, tc.expected)
			}
		})
	}
}

func TestCountLines(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")

	content := "line1\nline2\nline3\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	count, err := countLines(filePath)
	if err != nil {
		t.Fatalf("countLines() error = %v", err)
	}

	if count != 3 {
		t.Errorf("Expected 3 lines, got %d", count)
	}
}

func TestFormatIntSimple(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{10, "10"},
		{123, "123"},
		{1000, "1000"},
	}

	for _, tc := range tests {
		result := formatIntSimple(tc.input)
		if result != tc.expected {
			t.Errorf("formatIntSimple(%d) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}
