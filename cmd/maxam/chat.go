package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/ytnobody/MAXAM/internal/agent"
	"github.com/ytnobody/MAXAM/internal/config"
	"github.com/ytnobody/MAXAM/internal/logger"
	"github.com/ytnobody/MAXAM/internal/member"
	"github.com/ytnobody/MAXAM/internal/mention"
)

// ChatSession manages an interactive conversation with an agent
type ChatSession struct {
	agents  *agent.Agents
	members *member.Members
	logMgr  *logger.Manager
	workDir string
	history []chatMessage
}

type chatMessage struct {
	role    string // "user" or agent name
	content string
}

func runChat() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: maxam chat <agent>")
		fmt.Fprintln(os.Stderr, "Agents: (run 'maxam team list' to see configured agents)")
		os.Exit(1)
	}

	agentName := strings.ToLower(os.Args[2])
	workDir := getWorkDir()

	// Ensure project .maxam/ directory is initialized
	if !config.IsProjectInitialized(workDir) {
		if err := config.EnsureProjectInitialized(workDir); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to initialize .maxam/: %v\n", err)
		} else {
			fmt.Println("Created .maxam/ with default configuration files.")
			fmt.Println("- .maxam/config.yaml")
			fmt.Println("- .maxam/CLAUDE.md")
			fmt.Println()
		}
	}

	// Check if agents are configured
	cfg, err := config.LoadWithProject(workDir)
	if err != nil {
		cfg = config.DefaultConfig()
	}
	if !cfg.HasAgents() {
		fmt.Fprintln(os.Stderr, "No team members configured.")
		fmt.Fprintln(os.Stderr, "Please run 'maxam team init' first to set up your team.")
		os.Exit(1)
	}

	session := &ChatSession{
		agents:  agent.NewAgents(workDir),
		members: member.NewMembers(workDir),
		logMgr:  logger.NewManager(logger.GetDefaultLogDir(), workDir),
		workDir: workDir,
		history: make([]chatMessage, 0),
	}
	defer session.logMgr.Close()

	if agentName == "team" {
		session.runTeamChat()
	} else {
		session.runAgentChat(agentName)
	}
}

func (s *ChatSession) runAgentChat(agentName string) {
	runner, ok := s.agents.Get(agentName)
	if !ok {
		fmt.Fprintf(os.Stderr, "Unknown agent: %s\n", agentName)
		os.Exit(1)
	}

	fullName := s.members.GetFullName(agentName)
	interactive := term.IsTerminal(int(os.Stdin.Fd()))

	if interactive {
		fmt.Printf("Chat with %s\n", fullName)
		fmt.Println("Type 'exit' to quit, 'clear' to reset conversation")
		fmt.Println(strings.Repeat("-", 50))
	}

	scanner := bufio.NewScanner(os.Stdin)

	for {
		if interactive {
			fmt.Print("\nYou: ")
		}
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if interactive && (input == "exit" || input == "quit") {
			fmt.Println("Bye!")
			break
		}
		if interactive && input == "clear" {
			s.history = make([]chatMessage, 0)
			fmt.Println("(conversation cleared)")
			continue
		}

		s.history = append(s.history, chatMessage{role: "user", content: input})

		// Build prompt with history
		prompt := s.buildPrompt(agentName, input)

		if interactive {
			fmt.Printf("\n%s: ", fullName)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		result, err := runner.Run(ctx, prompt)
		cancel()

		if err != nil {
			fmt.Printf("(error: %v)\n", err)
			continue
		}

		result = strings.TrimSpace(result)
		fmt.Println(result)

		// Check for mention leaks in agent response
		if checkResult := mention.Check(result); checkResult.NeedsWarning {
			fmt.Printf("⚠️  %s\n", mention.FormatWarning())
		}

		s.history = append(s.history, chatMessage{role: agentName, content: result})

		// Log
		if log, err := s.logMgr.Get(agentName); err == nil {
			log.LogSimple(input, result, 0)
		}
	}
}

func (s *ChatSession) runTeamChat() {
	interactive := term.IsTerminal(int(os.Stdin.Fd()))

	if interactive {
		fmt.Println("Chat with MAXAM Team")
		fmt.Println("Mention members with @name. Example: @yuki, @priya")
		fmt.Println("Default: Mei will respond if no mention.")
		fmt.Println("Type 'exit' to quit")
		fmt.Println(strings.Repeat("-", 50))
	}

	scanner := bufio.NewScanner(os.Stdin)

	for {
		if interactive {
			fmt.Print("\nYou: ")
		}
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if interactive && (input == "exit" || input == "quit") {
			fmt.Println("Bye!")
			break
		}

		s.history = append(s.history, chatMessage{role: "user", content: input})

		// Detect all mentioned agents
		mentioned := s.members.DetectMentions(input)
		if len(mentioned) == 0 {
			// Default to Mei if no one is mentioned
			mentioned = []string{"mei"}
		}

		// Call each mentioned agent in order
		for _, agentName := range mentioned {
			runner, ok := s.agents.Get(agentName)
			if !ok {
				fmt.Printf("\n(%s is not available)\n", agentName)
				continue
			}

			fullName := s.members.GetFullName(agentName)
			prompt := s.buildPrompt(agentName, input)

			if interactive {
				fmt.Printf("\n%s: ", fullName)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			result, err := runner.Run(ctx, prompt)
			cancel()

			if err != nil {
				fmt.Printf("(error: %v)\n", err)
				continue
			}

			result = strings.TrimSpace(result)
			fmt.Println(result)

			// Check for mention leaks in agent response
			if checkResult := mention.Check(result); checkResult.NeedsWarning {
				fmt.Printf("⚠️  %s\n", mention.FormatWarning())
			}

			s.history = append(s.history, chatMessage{role: agentName, content: result})
		}
	}
}

func (s *ChatSession) buildPrompt(agentName, currentInput string) string {
	var sb strings.Builder

	sb.WriteString("これは対話セッションです。ユーザーと自然に会話してください。\n\n")
	sb.WriteString("重要なルール:\n")
	sb.WriteString("- 情報が不足している場合は、必ず質問してください\n")
	sb.WriteString("- 曖昧な指示には確認を取ってください\n")
	sb.WriteString("- 実行前に計画を説明し、OKをもらってから進めてください\n")
	sb.WriteString("- 短く自然な会話調で返答してください\n\n")

	if len(s.history) > 1 {
		sb.WriteString("## これまでの会話\n\n")
		// Last 10 messages for context
		start := 0
		if len(s.history) > 10 {
			start = len(s.history) - 10
		}
		for i := start; i < len(s.history)-1; i++ {
			msg := s.history[i]
			if msg.role == "user" {
				sb.WriteString(fmt.Sprintf("ユーザー: %s\n\n", msg.content))
			} else {
				sb.WriteString(fmt.Sprintf("%s: %s\n\n", s.members.GetFullName(msg.role), msg.content))
			}
		}
	}

	sb.WriteString("## 今回のメッセージ\n\n")
	sb.WriteString(fmt.Sprintf("ユーザー: %s\n\n", currentInput))
	sb.WriteString("あなたの返答:")

	return sb.String()
}
