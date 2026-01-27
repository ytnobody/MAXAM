package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ytnobody/MAXAM/internal/agent"
	"github.com/ytnobody/MAXAM/internal/logger"
)

type TeamChat struct {
	agents  *agent.Agents
	logMgr  *logger.Manager
	workDir string
	history []message
}

type message struct {
	role    string
	content string
}

func main() {
	fmt.Println("MAXAM Team Chat")
	fmt.Println("===============")
	fmt.Println()
	fmt.Println("チームメンバー:")
	fmt.Println("  Mei    - PM/要件定義 (デフォルト)")
	fmt.Println("  Yuki   - 実装/インフラ (@yuki)")
	fmt.Println("  Priya  - レビュー/QA (@priya)")
	fmt.Println("  Amara  - 分析 (@amara)")
	fmt.Println()
	fmt.Println("使い方: 普通に話しかけてください。@yukiなどでメンション可。")
	fmt.Println("終了: exit")
	fmt.Println(strings.Repeat("-", 50))

	workDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	chat := &TeamChat{
		agents:  agent.NewAgents(workDir),
		logMgr:  logger.NewManager(filepath.Join(workDir, "logs")),
		workDir: workDir,
		history: make([]message, 0),
	}
	defer chat.logMgr.Close()

	chat.run()
}

func (c *TeamChat) run() {
	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("\nYou: ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if input == "exit" || input == "quit" {
			fmt.Println("\nBye!")
			break
		}
		if input == "clear" {
			c.history = make([]message, 0)
			fmt.Println("(会話をリセットしました)")
			continue
		}
		if input == "status" {
			c.showStatus()
			continue
		}

		c.history = append(c.history, message{role: "user", content: input})

		// Detect agent
		agentName := c.detectAgent(input)
		runner, _ := c.agents.Get(agentName)
		fullName := getFullName(agentName)

		// Build prompt
		prompt := c.buildPrompt(agentName, input)

		fmt.Printf("\n%s: ", fullName)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		start := time.Now()
		result, err := runner.Run(ctx, prompt)
		elapsed := time.Since(start)
		cancel()

		if err != nil {
			fmt.Printf("(エラー: %v)\n", err)
			continue
		}

		result = strings.TrimSpace(result)
		fmt.Println(result)

		c.history = append(c.history, message{role: agentName, content: result})

		// Log
		if log, err := c.logMgr.Get(agentName); err == nil {
			log.LogSimple(input, result, elapsed)
		}
	}
}

func (c *TeamChat) detectAgent(text string) string {
	lower := strings.ToLower(text)

	if strings.Contains(lower, "@yuki") || strings.Contains(lower, "ゆき") ||
		strings.Contains(lower, "実装") || strings.Contains(lower, "コード書") {
		return "yuki"
	}
	if strings.Contains(lower, "@priya") || strings.Contains(lower, "プリヤ") ||
		strings.Contains(lower, "レビュー") || strings.Contains(lower, "チェック") {
		return "priya"
	}
	if strings.Contains(lower, "@amara") || strings.Contains(lower, "アマラ") ||
		strings.Contains(lower, "分析") || strings.Contains(lower, "傾向") {
		return "amara"
	}

	return "mei"
}

func (c *TeamChat) buildPrompt(agentName, input string) string {
	var sb strings.Builder

	sb.WriteString("これはチームチャットです。ユーザーと自然に会話してください。\n\n")
	sb.WriteString("重要:\n")
	sb.WriteString("- 情報が不足していたら質問してください\n")
	sb.WriteString("- 曖昧な指示には確認を取ってください\n")
	sb.WriteString("- 作業前に計画を説明し、OKをもらってから進めてください\n")
	sb.WriteString("- 短く自然な会話で返答してください\n")
	sb.WriteString("- 必要なら他のメンバーに振ることを提案してください\n\n")

	if len(c.history) > 1 {
		sb.WriteString("## 会話履歴\n\n")
		start := 0
		if len(c.history) > 10 {
			start = len(c.history) - 10
		}
		for i := start; i < len(c.history)-1; i++ {
			msg := c.history[i]
			if msg.role == "user" {
				sb.WriteString(fmt.Sprintf("ユーザー: %s\n\n", msg.content))
			} else {
				sb.WriteString(fmt.Sprintf("%s: %s\n\n", getFullName(msg.role), msg.content))
			}
		}
	}

	sb.WriteString("## 今のメッセージ\n\n")
	sb.WriteString(fmt.Sprintf("ユーザー: %s\n", input))

	return sb.String()
}

func (c *TeamChat) showStatus() {
	fmt.Println("\n--- Team Status ---")
	fmt.Printf("会話履歴: %d メッセージ\n", len(c.history))
	fmt.Printf("作業ディレクトリ: %s\n", c.workDir)
	fmt.Println("-------------------")
}

func getFullName(name string) string {
	switch name {
	case "mei":
		return "Mei"
	case "yuki":
		return "Yuki"
	case "priya":
		return "Priya"
	case "amara":
		return "Amara"
	default:
		return name
	}
}
