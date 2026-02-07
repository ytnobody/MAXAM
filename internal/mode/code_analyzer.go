// Package mode manages MAXAM operating modes
package mode

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// CodeAnalyzer performs static analysis on project code
type CodeAnalyzer struct {
	workDir string
}

// NewCodeAnalyzer creates a new CodeAnalyzer
func NewCodeAnalyzer(workDir string) *CodeAnalyzer {
	return &CodeAnalyzer{workDir: workDir}
}

// TodoItem represents a TODO comment found in code
type TodoItem struct {
	File    string
	Line    int
	Content string
	Type    string // TODO, FIXME, HACK, XXX
}

// CodeAnalysisResult contains the results of code analysis
type CodeAnalysisResult struct {
	Todos         []TodoItem
	UnusedImports []FileIssue
	LongFiles     []FileIssue
	Summary       string
}

// FileIssue represents an issue found in a specific file
type FileIssue struct {
	File    string
	Message string
}

// DefaultIgnoreDirs are directories to skip during analysis
var DefaultIgnoreDirs = []string{
	".git", "node_modules", "vendor", ".venv", "__pycache__",
	"dist", "build", "out", "target", ".next", ".nuxt",
}

// DefaultIgnoreFiles are file patterns to skip
var DefaultIgnoreFiles = []string{
	"*.min.js", "*.min.css", "*.map", "*.lock",
}

// Analyze performs code analysis on the project
func (ca *CodeAnalyzer) Analyze() (*CodeAnalysisResult, error) {
	result := &CodeAnalysisResult{
		Todos:         make([]TodoItem, 0),
		UnusedImports: make([]FileIssue, 0),
		LongFiles:     make([]FileIssue, 0),
	}

	// Find TODOs
	todos, err := ca.findTodos()
	if err != nil {
		return nil, err
	}
	result.Todos = todos

	// Find long files
	longFiles, err := ca.findLongFiles(500) // threshold: 500 lines
	if err != nil {
		return nil, err
	}
	result.LongFiles = longFiles

	// Generate summary
	result.Summary = ca.generateSummary(result)

	return result, nil
}

// findTodos scans for TODO, FIXME, HACK, XXX comments
func (ca *CodeAnalyzer) findTodos() ([]TodoItem, error) {
	var todos []TodoItem

	// Regex pattern for TODO-style comments
	pattern := regexp.MustCompile(`(?i)(TODO|FIXME|HACK|XXX)[\s:]*(.*)`)

	err := filepath.Walk(ca.workDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors, continue walking
		}

		// Skip directories
		if info.IsDir() {
			baseName := filepath.Base(path)
			for _, ignore := range DefaultIgnoreDirs {
				if baseName == ignore {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// Skip binary and non-code files
		if !isCodeFile(path) {
			return nil
		}

		// Skip ignored file patterns
		for _, pattern := range DefaultIgnoreFiles {
			if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
				return nil
			}
		}

		// Scan file for TODOs
		fileTodos, err := ca.scanFileForTodos(path, pattern)
		if err != nil {
			return nil // skip file errors
		}
		todos = append(todos, fileTodos...)

		return nil
	})

	if err != nil {
		return nil, err
	}

	return todos, nil
}

// scanFileForTodos scans a single file for TODO comments
func (ca *CodeAnalyzer) scanFileForTodos(path string, pattern *regexp.Regexp) ([]TodoItem, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var todos []TodoItem
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		matches := pattern.FindStringSubmatch(line)
		if len(matches) >= 3 {
			relPath, _ := filepath.Rel(ca.workDir, path)
			todos = append(todos, TodoItem{
				File:    relPath,
				Line:    lineNum,
				Content: strings.TrimSpace(matches[2]),
				Type:    strings.ToUpper(matches[1]),
			})
		}
	}

	return todos, scanner.Err()
}

// findLongFiles finds files exceeding the line threshold
func (ca *CodeAnalyzer) findLongFiles(threshold int) ([]FileIssue, error) {
	var longFiles []FileIssue

	err := filepath.Walk(ca.workDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			baseName := filepath.Base(path)
			for _, ignore := range DefaultIgnoreDirs {
				if baseName == ignore {
					return filepath.SkipDir
				}
			}
			return nil
		}

		if !isCodeFile(path) {
			return nil
		}

		lineCount, err := countLines(path)
		if err != nil {
			return nil
		}

		if lineCount > threshold {
			relPath, _ := filepath.Rel(ca.workDir, path)
			longFiles = append(longFiles, FileIssue{
				File:    relPath,
				Message: formatLineCount(lineCount),
			})
		}

		return nil
	})

	return longFiles, err
}

// generateSummary creates a human-readable summary
func (ca *CodeAnalyzer) generateSummary(result *CodeAnalysisResult) string {
	var sb strings.Builder

	sb.WriteString("## コード分析結果\n\n")

	// TODO summary
	if len(result.Todos) > 0 {
		sb.WriteString("### TODO/FIXME コメント\n\n")
		todoCount := countByType(result.Todos)
		for typ, count := range todoCount {
			sb.WriteString("- " + typ + ": " + formatCount(count) + "\n")
		}
		sb.WriteString("\n")

		// Show first few TODOs as examples
		sb.WriteString("**例:**\n")
		limit := 5
		if len(result.Todos) < limit {
			limit = len(result.Todos)
		}
		for i := 0; i < limit; i++ {
			todo := result.Todos[i]
			sb.WriteString("- `" + todo.File + ":" + formatLineNum(todo.Line) + "` - " + todo.Content + "\n")
		}
		if len(result.Todos) > 5 {
			sb.WriteString("- ... 他 " + formatCount(len(result.Todos)-5) + " 件\n")
		}
		sb.WriteString("\n")
	} else {
		sb.WriteString("### TODO/FIXME コメント\n\nなし\n\n")
	}

	// Long files
	if len(result.LongFiles) > 0 {
		sb.WriteString("### 長いファイル (500行超)\n\n")
		for _, f := range result.LongFiles {
			sb.WriteString("- `" + f.File + "` - " + f.Message + "\n")
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// isCodeFile checks if a file is a code file based on extension
func isCodeFile(path string) bool {
	codeExtensions := []string{
		".go", ".js", ".ts", ".jsx", ".tsx", ".py", ".rb", ".java",
		".c", ".cpp", ".h", ".hpp", ".cs", ".php", ".rs", ".swift",
		".kt", ".scala", ".sh", ".bash", ".zsh", ".yaml", ".yml",
		".json", ".toml", ".md", ".html", ".css", ".scss", ".sass",
	}

	ext := strings.ToLower(filepath.Ext(path))
	for _, codeExt := range codeExtensions {
		if ext == codeExt {
			return true
		}
	}
	return false
}

// countLines counts the number of lines in a file
func countLines(path string) (int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	count := 0
	for scanner.Scan() {
		count++
	}
	return count, scanner.Err()
}

// countByType counts TODOs by type
func countByType(todos []TodoItem) map[string]int {
	counts := make(map[string]int)
	for _, t := range todos {
		counts[t.Type]++
	}
	return counts
}

// formatCount formats a count as a string
func formatCount(n int) string {
	return strings.TrimSpace(strings.Replace(formatLineNum(n), " ", "", -1))
}

// formatLineNum formats a line number
func formatLineNum(n int) string {
	return strings.TrimLeft(strings.Replace(formatIntSimple(n), " ", "", -1), " ")
}

// formatIntSimple is a simple integer formatter without fmt
func formatIntSimple(n int) string {
	if n == 0 {
		return "0"
	}
	var result []byte
	for n > 0 {
		result = append([]byte{byte('0' + n%10)}, result...)
		n /= 10
	}
	return string(result)
}

// formatLineCount formats line count for display
func formatLineCount(n int) string {
	return formatIntSimple(n) + "行"
}
