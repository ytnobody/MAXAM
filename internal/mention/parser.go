// Package mention provides mention leak detection and member parsing.
package mention

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Member represents a team member parsed from CLAUDE.md
type Member struct {
	Name string
	Role string
}

// ParseMemberTable parses team member table from CLAUDE.md
// Looks for markdown tables with Name/名前 and Role/役割 columns
func ParseMemberTable(workDir string) ([]Member, error) {
	claudePath := filepath.Join(workDir, "CLAUDE.md")
	return parseTableFromFile(claudePath)
}

func parseTableFromFile(path string) ([]Member, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var members []Member
	scanner := bufio.NewScanner(file)

	inTable := false
	nameIdx := -1
	roleIdx := -1
	headerParsed := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines
		if line == "" {
			continue
		}

		// Check if this is a table row
		if !strings.HasPrefix(line, "|") {
			if inTable {
				// Table ended
				break
			}
			continue
		}

		// Parse table cells
		cells := parseTableRow(line)
		if len(cells) < 2 {
			continue
		}

		// Look for header row with Name/名前 column
		if !inTable {
			nameIdx, roleIdx = findColumns(cells)
			if nameIdx >= 0 {
				inTable = true
				headerParsed = false
				continue
			}
		}

		// Skip separator row (e.g., |---|---|)
		if !headerParsed {
			if isSeparatorRow(line) {
				headerParsed = true
				continue
			}
		}

		// Parse data row
		if inTable && headerParsed && nameIdx >= 0 && nameIdx < len(cells) {
			name := strings.TrimSpace(cells[nameIdx])
			role := ""
			if roleIdx >= 0 && roleIdx < len(cells) {
				role = strings.TrimSpace(cells[roleIdx])
			}

			if name != "" {
				members = append(members, Member{
					Name: name,
					Role: role,
				})
			}
		}
	}

	return members, scanner.Err()
}

// parseTableRow splits a markdown table row into cells
func parseTableRow(line string) []string {
	// Remove leading and trailing |
	line = strings.Trim(line, "|")
	parts := strings.Split(line, "|")

	var cells []string
	for _, p := range parts {
		cells = append(cells, strings.TrimSpace(p))
	}
	return cells
}

// findColumns finds the indices of Name and Role columns
func findColumns(cells []string) (nameIdx, roleIdx int) {
	nameIdx = -1
	roleIdx = -1

	namePatterns := []string{"name", "名前", "member", "メンバー"}
	rolePatterns := []string{"role", "役割", "担当"}

	for i, cell := range cells {
		lower := strings.ToLower(cell)
		for _, pattern := range namePatterns {
			if strings.Contains(lower, pattern) {
				nameIdx = i
				break
			}
		}
		for _, pattern := range rolePatterns {
			if strings.Contains(lower, pattern) {
				roleIdx = i
				break
			}
		}
	}

	return nameIdx, roleIdx
}

// isSeparatorRow checks if a line is a table separator (e.g., |---|---|)
func isSeparatorRow(line string) bool {
	// Match pattern like |---|---| or | --- | --- |
	match, _ := regexp.MatchString(`^\|[\s\-:]+\|[\s\-:|]+$`, line)
	return match
}

// ExtractShortName extracts the short name (lowercase, first word) from full name
// e.g., "Mei Chen" -> "mei", "Yuki Tanaka" -> "yuki"
func ExtractShortName(fullName string) string {
	parts := strings.Fields(fullName)
	if len(parts) == 0 {
		return ""
	}
	return strings.ToLower(parts[0])
}
