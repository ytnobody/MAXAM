// Package member extracts team members from CLAUDE.md
package member

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ytnobody/MAXAM/internal/config"
)

// Member represents a team member
type Member struct {
	Name     string // Short name (e.g., "yuki")
	FullName string // Full name (e.g., "Yuki Tanaka")
	Role     string // Role description
}

// Members holds all team members
type Members struct {
	members map[string]*Member // key: lowercase short name
}

// NewMembers creates a Members instance from config and CLAUDE.md
// Priority: project/.maxam/config.yaml > ~/.maxam/config.yaml > default
func NewMembers(workDir string) *Members {
	m := &Members{
		members: make(map[string]*Member),
	}

	// 1. Load from config with project overrides
	cfg, err := config.LoadWithProject(workDir)
	if err != nil {
		cfg = config.DefaultConfig()
	}
	for _, ac := range cfg.Agents {
		m.members[strings.ToLower(ac.Name)] = &Member{
			Name:     ac.Name,
			FullName: ac.FullName,
			Role:     ac.Role,
		}
	}

	// 2. Parse project-local CLAUDE.md (.maxam/CLAUDE.md) first
	projectClaudeMD := config.ProjectClaudeMD(workDir)
	if data, err := os.ReadFile(projectClaudeMD); err == nil {
		parsed := ParseMemberTable(string(data))
		for _, pm := range parsed {
			key := strings.ToLower(pm.Name)
			if existing, exists := m.members[key]; exists {
				// If existing FullName looks like a short name, use FullName from CLAUDE.md
				if existing.FullName == existing.Name || existing.FullName == key {
					existing.FullName = pm.FullName
				}
			} else {
				m.members[key] = pm
			}
		}
	}

	// 3. Parse root CLAUDE.md and merge (CLAUDE.md can add new members)
	claudeMDPath := filepath.Join(workDir, "CLAUDE.md")
	if data, err := os.ReadFile(claudeMDPath); err == nil {
		parsed := ParseMemberTable(string(data))
		for _, pm := range parsed {
			key := strings.ToLower(pm.Name)
			if existing, exists := m.members[key]; exists {
				// If existing FullName looks like a short name, use FullName from CLAUDE.md
				if existing.FullName == existing.Name || existing.FullName == key {
					existing.FullName = pm.FullName
				}
			} else {
				// New member from CLAUDE.md
				m.members[key] = pm
			}
		}
	}

	return m
}

// Get returns a member by name (case-insensitive)
func (m *Members) Get(name string) (*Member, bool) {
	member, ok := m.members[strings.ToLower(name)]
	return member, ok
}

// All returns all members
func (m *Members) All() []*Member {
	result := make([]*Member, 0, len(m.members))
	for _, member := range m.members {
		result = append(result, member)
	}
	return result
}

// Names returns all member names (short names, lowercase)
func (m *Members) Names() []string {
	result := make([]string, 0, len(m.members))
	for name := range m.members {
		result = append(result, name)
	}
	return result
}

// ParseMemberTable extracts members from CLAUDE.md table format
// Expects format:
// | Name | Role |
// |------|------|
// | Mei Chen | PM |
func ParseMemberTable(content string) []*Member {
	var result []*Member

	lines := strings.Split(content, "\n")
	inTable := false
	headerPassed := false

	// Match table rows: | content | content | ...
	tableRowRe := regexp.MustCompile(`^\s*\|(.+)\|\s*$`)
	// Match separator: |---|---|
	separatorRe := regexp.MustCompile(`^\s*\|[-:\s|]+\|\s*$`)

	for _, line := range lines {
		// Check for table row
		if match := tableRowRe.FindStringSubmatch(line); match != nil {
			// Check if it's a separator line
			if separatorRe.MatchString(line) {
				if inTable {
					headerPassed = true
				}
				continue
			}

			// Split by |
			cells := strings.Split(match[1], "|")
			if len(cells) < 2 {
				continue
			}

			name := strings.TrimSpace(cells[0])
			role := strings.TrimSpace(cells[1])

			// Skip header row (usually "Name" or "名前")
			if !inTable {
				if isHeaderCell(name) {
					inTable = true
					continue
				}
			}

			if !headerPassed {
				continue
			}

			// Extract short name from full name
			shortName := extractShortName(name)
			if shortName == "" {
				continue
			}

			result = append(result, &Member{
				Name:     shortName,
				FullName: name,
				Role:     role,
			})
		} else if inTable {
			// Non-table line after we were in table = table ended
			break
		}
	}

	return result
}

// isHeaderCell checks if a cell looks like a header
func isHeaderCell(cell string) bool {
	lower := strings.ToLower(cell)
	return lower == "name" || lower == "名前" || lower == "member" || lower == "メンバー"
}

// extractShortName extracts a short name from full name
// "Mei Chen" -> "mei"
// "Yuki Tanaka" -> "yuki"
func extractShortName(fullName string) string {
	parts := strings.Fields(fullName)
	if len(parts) == 0 {
		return ""
	}
	// Use first name as short name
	return strings.ToLower(parts[0])
}
