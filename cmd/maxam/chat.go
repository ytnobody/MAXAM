package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/ytnobody/MAXAM/internal/agent"
	"github.com/ytnobody/MAXAM/internal/config"
	"github.com/ytnobody/MAXAM/internal/logger"
	"github.com/ytnobody/MAXAM/internal/member"
	"github.com/ytnobody/MAXAM/internal/mention"
	"github.com/ytnobody/MAXAM/internal/ws"
)

// chatFlags holds parsed command line flags for chat command
type chatFlags struct {
	agentName    string
	daemon       bool
	mentionCheck bool
	websocket    bool
	wsPort       int
}

// ChatSession manages an interactive conversation with an agent
type ChatSession struct {
	agents       *agent.Agents
	members      *member.Members
	logMgr       *logger.Manager
	workDir      string
	history      []chatMessage
	mentionCheck bool
	daemon       bool
	wsServer     *ws.Server
	defaultAgent string
}

type chatMessage struct {
	role    string // "user" or agent name
	content string
}

// parseChatFlags parses command line arguments for chat command
func parseChatFlags(args []string) chatFlags {
	flags := chatFlags{
		mentionCheck: true, // default: on
	}

	if len(args) < 3 {
		return flags
	}

	flags.agentName = strings.ToLower(args[2])

	// Parse remaining flags
	for i := 3; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--daemon":
			flags.daemon = true
			// daemon mode: default mention check to false
			flags.mentionCheck = false
		case arg == "--mention-check":
			flags.mentionCheck = true
		case arg == "--mention-check=true":
			flags.mentionCheck = true
		case arg == "--mention-check=false":
			flags.mentionCheck = false
		case arg == "--no-mention-check":
			flags.mentionCheck = false
		case arg == "--websocket", arg == "--ws":
			flags.websocket = true
		case strings.HasPrefix(arg, "--ws-port="):
			if port, err := strconv.Atoi(strings.TrimPrefix(arg, "--ws-port=")); err == nil {
				flags.wsPort = port
				flags.websocket = true
			}
		}
	}

	return flags
}

func runChat() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: maxam chat <agent> [flags]")
		fmt.Fprintln(os.Stderr, "Agents: (run 'maxam team list' to see configured agents)")
		fmt.Fprintln(os.Stderr, "Flags:")
		fmt.Fprintln(os.Stderr, "  --daemon            Run in daemon mode (wait for input continuously)")
		fmt.Fprintln(os.Stderr, "  --mention-check     Enable mention checker (default: on, off in daemon mode)")
		fmt.Fprintln(os.Stderr, "  --no-mention-check  Disable mention checker")
		fmt.Fprintln(os.Stderr, "  --websocket, --ws   Enable WebSocket broadcast (daemon mode only)")
		fmt.Fprintln(os.Stderr, "  --ws-port=PORT      WebSocket server port (default: 8080)")
		os.Exit(1)
	}

	flags := parseChatFlags(os.Args)
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

	members := member.NewMembers(workDir)

	// Determine default agent: role-based (フェイシング) > config > fallback
	defaultAgent := members.FindByRoleContains("フェイシング")
	if defaultAgent == "" {
		defaultAgent = cfg.DefaultAgent
	}
	if defaultAgent == "" {
		defaultAgent = "mei" // final fallback
	}

	session := &ChatSession{
		agents:       agent.NewAgents(workDir),
		members:      members,
		logMgr:       logger.NewManagerWithLevel(logger.GetDefaultLogDir(), workDir, logger.ParseLogLevel(cfg.GetLogLevel())),
		workDir:      workDir,
		history:      make([]chatMessage, 0),
		mentionCheck: flags.mentionCheck,
		daemon:       flags.daemon,
		defaultAgent: defaultAgent,
	}
	defer session.logMgr.Close()

	// Start WebSocket server if enabled (daemon mode only)
	if flags.websocket && flags.daemon {
		port := flags.wsPort
		if port == 0 {
			port = 8080
		}
		session.wsServer = ws.NewServer(port)
		if err := session.wsServer.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to start WebSocket server: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "WebSocket server started on port %d\n", port)
			defer func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				session.wsServer.Stop(ctx)
			}()
		}
	}

	if flags.agentName == "team" {
		if flags.daemon {
			session.runTeamChatDaemon()
		} else {
			session.runTeamChat()
		}
	} else {
		session.runAgentChat(flags.agentName)
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
		if s.mentionCheck {
			if checkResult := mention.Check(result); checkResult.NeedsWarning {
				fmt.Printf("⚠️  %s\n", mention.FormatWarning())
			}
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
		fmt.Printf("Default: %s will respond if no mention.\n", s.members.GetFullName(s.defaultAgent))
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
			// Default to configured agent if no one is mentioned
			mentioned = []string{s.defaultAgent}
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
			if s.mentionCheck {
				if checkResult := mention.Check(result); checkResult.NeedsWarning {
					fmt.Printf("⚠️  %s\n", mention.FormatWarning())
				}
			}

			s.history = append(s.history, chatMessage{role: agentName, content: result})
		}
	}
}

func (s *ChatSession) runTeamChatDaemon() {
	fmt.Fprintln(os.Stderr, "MAXAM Team Chat (daemon mode)")
	fmt.Fprintln(os.Stderr, "Waiting for input... (empty line to send, Ctrl+C to quit)")

	scanner := bufio.NewScanner(os.Stdin)
	var messageLines []string

	for {
		if !scanner.Scan() {
			// EOF or error - process remaining message if any
			if len(messageLines) > 0 {
				s.processTeamMessage(strings.Join(messageLines, "\n"))
			}
			break
		}

		line := scanner.Text()

		// Empty line = end of message
		if line == "" {
			if len(messageLines) > 0 {
				s.processTeamMessage(strings.Join(messageLines, "\n"))
				messageLines = nil
			}
			continue
		}

		messageLines = append(messageLines, line)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
	}
}

func (s *ChatSession) processTeamMessage(input string) {
	input = strings.TrimSpace(input)
	if input == "" {
		return
	}

	s.history = append(s.history, chatMessage{role: "user", content: input})

	// Broadcast user input via WebSocket if server is running
	if s.wsServer != nil && s.wsServer.IsRunning() {
		userMsg := ws.NewChatMessage("owner", "", input)
		s.wsServer.Broadcast(userMsg)
	}

	// Detect all mentioned agents
	mentioned := s.members.DetectMentions(input)
	if len(mentioned) == 0 {
		mentioned = []string{s.defaultAgent}
	}

	// Call each mentioned agent in order
	for _, agentName := range mentioned {
		runner, ok := s.agents.Get(agentName)
		if !ok {
			fmt.Printf("(%s is not available)\n", agentName)
			continue
		}

		fullName := s.members.GetFullName(agentName)
		prompt := s.buildPrompt(agentName, input)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		result, err := runner.Run(ctx, prompt)
		cancel()

		if err != nil {
			fmt.Printf("(error: %v)\n", err)
			continue
		}

		result = strings.TrimSpace(result)
		fmt.Println(result)

		// Broadcast agent response via WebSocket if server is running
		if s.wsServer != nil && s.wsServer.IsRunning() {
			agentMsg := ws.NewChatMessage(fullName, "", result)
			s.wsServer.Broadcast(agentMsg)
		}

		// Check for mention leaks in agent response
		if s.mentionCheck {
			if checkResult := mention.Check(result); checkResult.NeedsWarning {
				fmt.Printf("⚠️  %s\n", mention.FormatWarning())
			}
		}

		s.history = append(s.history, chatMessage{role: agentName, content: result})
	}

	// Flush stdout to ensure response is sent immediately
	os.Stdout.Sync()
}

func (s *ChatSession) buildPrompt(agentName, currentInput string) string {
	var sb strings.Builder

	sb.WriteString("これは対話セッションです。ユーザーと自然に会話してください。\n\n")
	sb.WriteString("重要なルール:\n")
	sb.WriteString("- 情報が不足している場合は、必ず質問してください\n")
	sb.WriteString("- 曖昧な指示には確認を取ってください\n")
	sb.WriteString("- 実行前に計画を説明し、OKをもらってから進めてください\n")
	sb.WriteString("- 短く自然な会話調で返答してください\n\n")

	// daemon モード時は gh コマンドの使い方を追記
	if s.daemon {
		sb.WriteString("## GitHub連携\n\n")
		sb.WriteString("Issue や PR にコメントを残したい場合は `gh` コマンドを使用してください。\n\n")
		sb.WriteString("- Issue へのコメント: `gh issue comment <番号> --body \"コメント内容\"`\n")
		sb.WriteString("- PR へのコメント: `gh pr comment <番号> --body \"コメント内容\"`\n\n")
		sb.WriteString("例:\n")
		sb.WriteString("- `gh issue comment 123 --body \"実装完了しました\"`\n")
		sb.WriteString("- `gh pr comment 45 --body \"LGTM!\"`\n\n")
	}

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
