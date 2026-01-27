package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ytnobody/MAXAM/internal/agent"
	"github.com/ytnobody/MAXAM/internal/logger"
)

// AnalysisCycle represents Amara's weekly analysis workflow
type AnalysisCycle struct {
	agents  *agent.Agents
	logMgr  *logger.Manager
	workDir string
}

// NewAnalysisCycle creates a new analysis cycle workflow
func NewAnalysisCycle(agents *agent.Agents, logMgr *logger.Manager, workDir string) *AnalysisCycle {
	return &AnalysisCycle{
		agents:  agents,
		logMgr:  logMgr,
		workDir: workDir,
	}
}

// AnalysisResult contains the outcome of the analysis
type AnalysisResult struct {
	Report          string
	Recommendations []string
	CLAUDEMDUpdates map[string]string // agent -> update content
}

// Run executes Amara's analysis
func (ac *AnalysisCycle) Run(ctx context.Context) (*AnalysisResult, error) {
	fmt.Println("[Amara] Starting weekly analysis...")

	// Collect logs from all agents
	logsContent, err := ac.collectLogs()
	if err != nil {
		return nil, fmt.Errorf("collect logs: %w", err)
	}

	// Collect current CLAUDE.md files
	claudeMDs, err := ac.collectCLAUDEMDs()
	if err != nil {
		return nil, fmt.Errorf("collect CLAUDE.md: %w", err)
	}

	// Build analysis prompt
	prompt := ac.buildAnalysisPrompt(logsContent, claudeMDs)

	// Run Amara
	amara := ac.agents.Amara()
	start := time.Now()
	result, err := amara.Run(ctx, prompt)
	elapsed := time.Since(start)

	if err != nil {
		return nil, fmt.Errorf("amara analysis: %w", err)
	}

	// Log Amara's work
	if log, err := ac.logMgr.Get("amara"); err == nil {
		log.LogSimple(prompt, result, elapsed)
	}

	// Parse result
	analysisResult := ac.parseAnalysisResult(result)

	fmt.Printf("[Amara] Analysis complete (%v)\n", elapsed.Round(time.Millisecond))
	return analysisResult, nil
}

func (ac *AnalysisCycle) collectLogs() (string, error) {
	projectName := logger.ProjectNameFromPath(ac.workDir)
	logsDir := filepath.Join(logger.GetDefaultLogDir(), projectName)
	var content strings.Builder

	agents := []string{"mei", "yuki", "priya", "amara"}
	for _, agentName := range agents {
		agentDir := filepath.Join(logsDir, agentName)
		files, err := os.ReadDir(agentDir)
		if err != nil {
			continue
		}

		// Get recent log files (last 7 days)
		cutoff := time.Now().AddDate(0, 0, -7)
		for _, f := range files {
			if !strings.HasSuffix(f.Name(), ".md") {
				continue
			}

			info, err := f.Info()
			if err != nil || info.ModTime().Before(cutoff) {
				continue
			}

			data, err := os.ReadFile(filepath.Join(agentDir, f.Name()))
			if err != nil {
				continue
			}

			content.WriteString(fmt.Sprintf("\n## %s - %s\n\n", agentName, f.Name()))
			// Truncate if too long
			logContent := string(data)
			if len(logContent) > 10000 {
				logContent = logContent[:10000] + "\n...(truncated)"
			}
			content.WriteString(logContent)
		}
	}

	return content.String(), nil
}

func (ac *AnalysisCycle) collectCLAUDEMDs() (map[string]string, error) {
	result := make(map[string]string)

	// Shared CLAUDE.md
	sharedPath := filepath.Join(ac.workDir, "CLAUDE.md")
	if data, err := os.ReadFile(sharedPath); err == nil {
		result["shared"] = string(data)
	}

	// Agent-specific CLAUDE.md
	agents := []string{"mei", "yuki", "priya", "amara"}
	for _, agentName := range agents {
		path := filepath.Join(ac.workDir, "agents", agentName, "CLAUDE.md")
		if data, err := os.ReadFile(path); err == nil {
			result[agentName] = string(data)
		}
	}

	return result, nil
}

func (ac *AnalysisCycle) buildAnalysisPrompt(logs string, claudeMDs map[string]string) string {
	var prompt strings.Builder

	prompt.WriteString(`あなたはAmaraです。MAXAMチームの分析担当として、今週のログを分析してください。

## 分析観点
1. レビュー差し戻し頻出パターン
2. 要件ヒアリングで聞き漏れがちな項目
3. 実装で時間がかかる箇所
4. 顧客からの追加質問が多い領域

## 今週のログ
`)
	prompt.WriteString(logs)

	prompt.WriteString("\n\n## 現在のCLAUDE.md\n")
	for name, content := range claudeMDs {
		prompt.WriteString(fmt.Sprintf("\n### %s\n%s\n", name, content))
	}

	prompt.WriteString(`

## 出力形式
以下の形式で出力してください:

### レポート
（分析結果の概要）

### 発見されたパターン
- パターン1
- パターン2
...

### 推奨事項
- 推奨1
- 推奨2
...

### CLAUDE.md更新提案
（共通または各エージェントのCLAUDE.mdへの追記提案）
`)

	return prompt.String()
}

func (ac *AnalysisCycle) parseAnalysisResult(result string) *AnalysisResult {
	ar := &AnalysisResult{
		Report:          result,
		Recommendations: make([]string, 0),
		CLAUDEMDUpdates: make(map[string]string),
	}

	// Simple parsing for recommendations
	lines := strings.Split(result, "\n")
	inRecommendations := false

	for _, line := range lines {
		if strings.Contains(line, "### 推奨") || strings.Contains(line, "### Recommend") {
			inRecommendations = true
			continue
		}
		if strings.HasPrefix(line, "###") {
			inRecommendations = false
			continue
		}
		if inRecommendations && strings.HasPrefix(strings.TrimSpace(line), "-") {
			rec := strings.TrimPrefix(strings.TrimSpace(line), "- ")
			if rec != "" {
				ar.Recommendations = append(ar.Recommendations, rec)
			}
		}
	}

	return ar
}
