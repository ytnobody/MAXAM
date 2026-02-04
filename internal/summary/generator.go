// Package summary provides automatic generation of CLAUDE.summary.md
package summary

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// SourceFile is the full CLAUDE.md filename
	SourceFile = "CLAUDE.md"
	// SummaryFile is the generated summary filename
	SummaryFile = "CLAUDE.summary.md"
)

// SectionsToExtract defines which sections to include in summary
// These are H2 headers (## Section Name)
var SectionsToExtract = []string{
	"プロジェクト概要",
	"チームメンバー",
	"コーディング規約",
	"通信規約",
	"エスカレーションルール",
	"Worktree運用",
}

// MaxLinesPerSection limits lines per section to keep summary concise
const MaxLinesPerSection = 30

// NeedsRegeneration checks if CLAUDE.summary.md needs to be regenerated
// Returns true if:
// - CLAUDE.md exists and CLAUDE.summary.md doesn't exist
// - CLAUDE.md is newer than CLAUDE.summary.md
func NeedsRegeneration(workDir string) bool {
	sourcePath := filepath.Join(workDir, SourceFile)
	summaryPath := filepath.Join(workDir, SummaryFile)

	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		// Source doesn't exist, nothing to generate
		return false
	}

	summaryInfo, err := os.Stat(summaryPath)
	if err != nil {
		// Summary doesn't exist but source does
		return true
	}

	// Compare modification times
	return sourceInfo.ModTime().After(summaryInfo.ModTime())
}

// EnsureUpToDate regenerates summary if needed
func EnsureUpToDate(workDir string) error {
	if !NeedsRegeneration(workDir) {
		return nil
	}
	return Generate(workDir)
}

// Generate creates CLAUDE.summary.md from CLAUDE.md
func Generate(workDir string) error {
	sourcePath := filepath.Join(workDir, SourceFile)
	summaryPath := filepath.Join(workDir, SummaryFile)

	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("read source: %w", err)
	}

	summary := ExtractSections(string(content), SectionsToExtract)

	// Add header and footer
	output := "# MAXAM 共通規約（要約版）\n\n" + summary + "\n---\n\n*詳細はCLAUDE.md（フル版）を参照。チーム固有情報は.maxam/CLAUDE.mdを参照*\n"

	if err := os.WriteFile(summaryPath, []byte(output), 0644); err != nil {
		return fmt.Errorf("write summary: %w", err)
	}

	return nil
}

// ExtractSections extracts specified H2 sections from markdown content
func ExtractSections(content string, sections []string) string {
	// Build a set for quick lookup
	sectionSet := make(map[string]bool)
	for _, s := range sections {
		sectionSet[s] = true
	}

	var result strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(content))

	var currentSection string
	var inTargetSection bool
	var lineCount int

	for scanner.Scan() {
		line := scanner.Text()

		// Check for H2 header
		if strings.HasPrefix(line, "## ") {
			sectionName := strings.TrimPrefix(line, "## ")

			// Finish previous section
			if inTargetSection && currentSection != "" {
				result.WriteString("\n")
			}

			// Check if this section should be extracted
			if sectionSet[sectionName] {
				inTargetSection = true
				currentSection = sectionName
				lineCount = 0
				result.WriteString(line)
				result.WriteString("\n")
			} else {
				inTargetSection = false
				currentSection = ""
			}
			continue
		}

		// Check for H1 header (skip title line)
		if strings.HasPrefix(line, "# ") {
			continue
		}

		// If in target section, add line
		if inTargetSection {
			// Stop at H3 within section after certain content (keep first subsections)
			if strings.HasPrefix(line, "### ") {
				// Include subsection header
				lineCount++
				if lineCount <= MaxLinesPerSection {
					result.WriteString("\n")
					result.WriteString(line)
					result.WriteString("\n")
				}
				continue
			}

			lineCount++
			if lineCount <= MaxLinesPerSection {
				result.WriteString(line)
				result.WriteString("\n")
			}
		}
	}

	return result.String()
}
