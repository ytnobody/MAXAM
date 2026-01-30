package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/fsnotify/fsnotify"

	"github.com/ytnobody/MAXAM/internal/agent"
	"github.com/ytnobody/MAXAM/internal/ccusage"
	"github.com/ytnobody/MAXAM/internal/config"
	gh "github.com/ytnobody/MAXAM/internal/github"
	"github.com/ytnobody/MAXAM/internal/history"
	"github.com/ytnobody/MAXAM/internal/logger"
	"github.com/ytnobody/MAXAM/internal/mention"
	"github.com/ytnobody/MAXAM/internal/router"
	"github.com/ytnobody/MAXAM/internal/taskboard"
	"github.com/ytnobody/MAXAM/internal/tui/tasklist"
	"github.com/ytnobody/MAXAM/internal/worktree"
)

// Version is set by ldflags at build time
var Version = "dev"

// Theme colors for each agent
// Mei: Cherry Blossom Pink - 優しいお姉さんのイメージ
// Yuki: Ice Blue - クールな職人肌
// Priya: Saffron Orange - 情熱的なツンデレ
// Amara: Royal Purple - 知的な参謀

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			Padding(0, 1)

	// ヘッダー行全体の背景色（控えめなダークグレー）
	headerStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("236"))

	userStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("251")).
			Bold(true)

	// Mei: Cherry Blossom Pink (#FFB7C5)
	meiStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFB7C5")).
			Bold(true)

	// Yuki: Ice Blue (#87CEEB)
	yukiStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#87CEEB")).
			Bold(true)

	// Priya: Saffron Orange (#FF9933)
	priyaStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF9933")).
			Bold(true)

	// Amara: Royal Purple (#9370DB)
	amaraStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9370DB")).
			Bold(true)

	// Rin: Bright Green (#00FF7F) - 元気でポジティブ
	rinStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF7F")).
			Bold(true)

	// Shiori: Soft Lavender (#E6E6FA) - 穏やかで几帳面
	shioriStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E6E6FA")).
			Bold(true)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	// Warning style for mention leak detection
	warningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFD700")).
			Bold(true)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Padding(0, 1)

	// フッター行全体の背景色（控えめなダークグレー）
	footerStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("236"))

	// YOLO mode indicator style
	yoloStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Background(lipgloss.Color("#FFFF00")).
			Bold(true).
			Padding(0, 1)
)

type tuiMessage struct {
	role    string
	content string
}

// viewMode represents the current view
type viewMode int

const (
	viewChat viewMode = iota
	viewTaskboard
)

type tuiModel struct {
	agents         *agent.Agents
	router         *router.Router
	logMgr         *logger.Manager
	history        *history.History
	workDir        string
	projectContext string

	viewport          viewport.Model
	textInput         textinput.Model
	messages          []tuiMessage
	inputHist         []string
	histIdx           int
	tempInput         string
	ready             bool
	processingAgents  map[string]bool // 各エージェントの処理状態
	width             int
	height            int

	// View switching
	currentView viewMode
	tasklist    tasklist.Model
	taskService *taskboard.FileStore

	// File watcher
	taskWatcher *fsnotify.Watcher

	// Mention leak warning
	mentionWarning string

	// PR watcher for worktree cleanup
	prWatcher *gh.Watcher

	// ccusage integration
	ccusageClient *ccusage.Client
	todayCost     float64

	// Token optimization
	projectContextSent bool // projectContextを送信済みかどうか

	// Analysis settings
	analysisMinMessages int // 分析実行の最小メッセージ数

	// YOLO mode - skip confirmations and auto-approve
	yoloMode bool
}

type agentResponseMsg struct {
	agent      string
	content    string
	elapsed    time.Duration
	err        error
	nextAgents []string // 次に発言するエージェント（連鎖用、複数対応）
	chainDepth int      // 連鎖の深さ
}

// analysisTickMsg は1時間ごとの軽量分析トリガー
type analysisTickMsg struct {
	time time.Time
}

const maxChainDepth = 1000          // 最大連鎖数
const meiInterventionInterval = 10  // Meiが介入する間隔

func initialTuiModel(workDir string) tuiModel {
	ti := textinput.New()
	ti.Placeholder = "メッセージを入力... (@yuki, @priya, @amara でメンション)"
	ti.Focus()
	ti.Width = 80

	// Load chat history
	hist, err := history.New("")
	if err != nil {
		// Continue without history if error
		hist = nil
	}

	// Convert persisted history to messages
	messages := make([]tuiMessage, 0)
	if hist != nil {
		for _, msg := range hist.GetAll() {
			messages = append(messages, tuiMessage{
				role:    msg.Role,
				content: msg.Content,
			})
		}
	}

	// Initialize taskboard service (file-based) and tasklist
	taskService, err := taskboard.NewFileStore("")
	if err != nil {
		// Fallback: continue without file store
		taskService = nil
	}
	var tasklistModel tasklist.Model
	if taskService != nil {
		tasklistModel = tasklist.New(taskService)
	}

	// Setup file watcher for task file
	var taskWatcher *fsnotify.Watcher
	if taskService != nil {
		taskWatcher, _ = fsnotify.NewWatcher()
		if taskWatcher != nil {
			taskWatcher.Add(taskService.FilePath())
		}
	}

	// Setup PR watcher for worktree cleanup (silently fail if no GitHub access)
	var prWatcher *gh.Watcher
	if ghClient, err := gh.NewClient("ytnobody", "MAXAM"); err == nil {
		prWatcher = gh.NewWatcher(ghClient)
	}

	// Setup ccusage client (silently disabled if not available)
	ccClient := ccusage.NewClient()

	// Setup agent router
	cfg, err := config.Load()
	if err != nil {
		cfg = config.DefaultConfig()
	}
	routerAgents := make([]router.AgentInfo, len(cfg.Agents))
	agentNames := make([]string, len(cfg.Agents))
	for i, agentCfg := range cfg.Agents {
		routerAgents[i] = router.AgentInfo{
			Name: agentCfg.Name,
			Role: agentCfg.Role,
		}
		agentNames[i] = agentCfg.Name
	}
	agentRouter := router.New(routerAgents, cfg.DefaultAgent)

	// Setup mention checker with agent names from config
	mention.SetDefaultChecker(mention.NewChecker(agentNames))

	return tuiModel{
		agents:              agent.NewAgents(workDir),
		router:              agentRouter,
		logMgr:              logger.NewManager(logger.GetDefaultLogDir(), workDir),
		history:             hist,
		workDir:             workDir,
		textInput:           ti,
		messages:            messages,
		inputHist:           make([]string, 0),
		processingAgents:    make(map[string]bool),
		histIdx:             -1,
		currentView:         viewChat,
		tasklist:            tasklistModel,
		taskService:         taskService,
		taskWatcher:         taskWatcher,
		prWatcher:           prWatcher,
		ccusageClient:       ccClient,
		analysisMinMessages: cfg.GetAnalysisMinMessages(),
		yoloMode:            cfg.YOLOMode,
	}
}

// taskFileChangedMsg is sent when the task file is modified externally
type taskFileChangedMsg struct{}

// prCheckTickMsg is sent periodically to check for merged PRs
type prCheckTickMsg struct{}

// prMergedMsg is sent when PRs are found to be merged
type prMergedMsg struct {
	branches []string
}

// ccusageUpdateMsg is sent when ccusage cost is updated
type ccusageUpdateMsg struct {
	cost float64
}

// ccusageTickMsg triggers periodic ccusage updates
type ccusageTickMsg struct{}

func (m tuiModel) Init() tea.Cmd {
	cmds := []tea.Cmd{textinput.Blink, tea.EnterAltScreen, m.tickAnalysis()}
	if m.taskWatcher != nil {
		cmds = append(cmds, m.watchTaskFile())
	}
	if m.prWatcher != nil {
		cmds = append(cmds, m.tickPRCheck())
	}
	if m.ccusageClient != nil {
		cmds = append(cmds, m.fetchCcusage(), m.tickCcusage())
	}
	return tea.Batch(cmds...)
}

// tickCcusage returns a command that triggers ccusage update every 5 minutes
func (m tuiModel) tickCcusage() tea.Cmd {
	return tea.Tick(5*time.Minute, func(t time.Time) tea.Msg {
		return ccusageTickMsg{}
	})
}

// fetchCcusage fetches the current cost from ccusage
func (m tuiModel) fetchCcusage() tea.Cmd {
	return func() tea.Msg {
		if m.ccusageClient == nil {
			return nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		usage := m.ccusageClient.GetTodayUsage(ctx)
		if !usage.Available {
			return nil
		}
		return ccusageUpdateMsg{cost: usage.TodayCost}
	}
}

// tickPRCheck returns a command that triggers PR check every 5 minutes
func (m tuiModel) tickPRCheck() tea.Cmd {
	return tea.Tick(5*time.Minute, func(t time.Time) tea.Msg {
		return prCheckTickMsg{}
	})
}

// watchTaskFile watches for changes to the task file
func (m tuiModel) watchTaskFile() tea.Cmd {
	return func() tea.Msg {
		if m.taskWatcher == nil {
			return nil
		}
		select {
		case event, ok := <-m.taskWatcher.Events:
			if !ok {
				return nil
			}
			if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
				return taskFileChangedMsg{}
			}
		case <-m.taskWatcher.Errors:
			// Ignore errors
		}
		return nil
	}
}

// tickAnalysis は1時間後に軽量分析をトリガーするtickを設定
func (m tuiModel) tickAnalysis() tea.Cmd {
	return tea.Tick(time.Hour, func(t time.Time) tea.Msg {
		return analysisTickMsg{time: t}
	})
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.logMgr.Close()
			if m.taskWatcher != nil {
				m.taskWatcher.Close()
			}
			return m, tea.Quit

		case tea.KeyCtrlL:
			// 画面再描画（表示崩れのリカバリ用）
			m.updateViewport()
			return m, tea.ClearScreen

		case tea.KeyCtrlY:
			// Toggle YOLO mode
			m.yoloMode = !m.yoloMode
			return m, nil

		case tea.KeyTab:
			// Toggle between chat and taskboard view
			if m.currentView == viewChat {
				m.currentView = viewTaskboard
				m.tasklist.SetFocused(true)
				m.tasklist.Refresh()
			} else {
				m.currentView = viewChat
				m.tasklist.SetFocused(false)
			}
			return m, nil

		case tea.KeyEnter:
			// タスクボードビューの場合はtasklistに委譲
			if m.currentView == viewTaskboard {
				var cmd tea.Cmd
				newModel, cmd := m.tasklist.Update(msg)
				m.tasklist = newModel.(tasklist.Model)
				return m, cmd
			}

			input := strings.TrimSpace(m.textInput.Value())
			if input == "" {
				return m, nil
			}

			if input == "exit" || input == "quit" {
				m.logMgr.Close()
				if m.taskWatcher != nil {
					m.taskWatcher.Close()
				}
				return m, tea.Quit
			}

			if input == "clear" {
				m.messages = make([]tuiMessage, 0)
				m.processingAgents = make(map[string]bool)
				if m.history != nil {
					m.history.Clear()
				}
				m.textInput.SetValue("")
				m.updateViewport()
				return m, nil
			}

			// Save to input history
			m.inputHist = append(m.inputHist, input)
			m.histIdx = len(m.inputHist)
			m.tempInput = ""
			m.textInput.SetValue("")

			// Add user message
			m.messages = append(m.messages, tuiMessage{role: "user", content: input})
			if m.history != nil {
				m.history.Add("user", input)
			}
			m.updateViewport()

			// 複数メンション対応: 検出されたエージェント全員に並列でリクエスト
			targetAgents := m.detectAgents(input)
			var cmds []tea.Cmd
			for _, agentName := range targetAgents {
				// そのエージェントが処理中でなければ開始
				if !m.processingAgents[agentName] {
					m.processingAgents[agentName] = true
					cmds = append(cmds, m.runAgentAsync(input, agentName, 0))
				}
			}

			if len(cmds) > 0 {
				m.updateViewport()
				return m, tea.Batch(cmds...)
			}
			return m, nil

		case tea.KeyUp:
			// タスクボードビューの場合はtasklistに委譲
			if m.currentView == viewTaskboard {
				var cmd tea.Cmd
				newModel, cmd := m.tasklist.Update(msg)
				m.tasklist = newModel.(tasklist.Model)
				return m, cmd
			}
			if msg.Alt || tea.KeyMsg(msg).String() == "shift+up" {
				// Shift+Up: scroll viewport up
				m.viewport.LineUp(3)
				return m, nil
			}
			// Up: navigate input history
			if len(m.inputHist) == 0 {
				return m, nil
			}
			if m.histIdx == len(m.inputHist) {
				m.tempInput = m.textInput.Value()
			}
			if m.histIdx > 0 {
				m.histIdx--
				m.textInput.SetValue(m.inputHist[m.histIdx])
				m.textInput.CursorEnd()
			}
			return m, nil

		case tea.KeyDown:
			// タスクボードビューの場合はtasklistに委譲
			if m.currentView == viewTaskboard {
				var cmd tea.Cmd
				newModel, cmd := m.tasklist.Update(msg)
				m.tasklist = newModel.(tasklist.Model)
				return m, cmd
			}
			if msg.Alt || tea.KeyMsg(msg).String() == "shift+down" {
				// Shift+Down: scroll viewport down
				m.viewport.LineDown(3)
				return m, nil
			}
			// Down: navigate input history
			if m.histIdx < len(m.inputHist)-1 {
				m.histIdx++
				m.textInput.SetValue(m.inputHist[m.histIdx])
				m.textInput.CursorEnd()
			} else if m.histIdx == len(m.inputHist)-1 {
				m.histIdx = len(m.inputHist)
				m.textInput.SetValue(m.tempInput)
				m.textInput.CursorEnd()
			}
			return m, nil

		case tea.KeyPgUp:
			m.viewport.HalfViewUp()
			return m, nil

		case tea.KeyPgDown:
			m.viewport.HalfViewDown()
			return m, nil

		default:
			// タスクボードビューで'd'キーの場合はtasklistに委譲
			if m.currentView == viewTaskboard && msg.String() == "d" {
				var cmd tea.Cmd
				newModel, cmd := m.tasklist.Update(msg)
				m.tasklist = newModel.(tasklist.Model)
				return m, cmd
			}
		}

	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.viewport.LineUp(3)
			return m, nil
		case tea.MouseButtonWheelDown:
			m.viewport.LineDown(3)
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		headerHeight := 2
		footerHeight := 3
		verticalMargin := headerHeight + footerHeight

		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-verticalMargin)
			m.viewport.YPosition = headerHeight
			m.analyzeProject()
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - verticalMargin
		}
		m.textInput.Width = msg.Width - 8
		m.updateViewport()

	case taskFileChangedMsg:
		// Task file was modified externally - reload and refresh
		if m.taskService != nil {
			m.taskService.Reload()
			m.tasklist.Refresh()
		}
		// Continue watching
		if m.taskWatcher != nil {
			return m, m.watchTaskFile()
		}
		return m, nil

	case analysisTickMsg:
		// 1時間ごとの軽量分析トリガー
		// Amaraが処理中でなく、直近のメッセージ数が閾値以上の場合のみ実行
		recentMessages := m.getRecentMessages(time.Hour)
		if !m.processingAgents["amara"] && len(recentMessages) >= m.analysisMinMessages {
			m.processingAgents["amara"] = true
			return m, tea.Batch(
				m.runLightweightAnalysis(),
				m.tickAnalysis(), // 次のtick予約
			)
		}
		return m, m.tickAnalysis()

	case prCheckTickMsg:
		// 5分ごとにPRマージをチェック
		if m.prWatcher != nil {
			return m, tea.Batch(
				m.checkMergedPRs(),
				m.tickPRCheck(),
			)
		}
		return m, nil

	case prMergedMsg:
		// マージされたPRのブランチに対応するworktreeを削除（サイレント）
		for _, branch := range msg.branches {
			worktree.CleanupForBranch(m.workDir, branch)
		}
		return m, nil

	case ccusageTickMsg:
		// 5分ごとにccusageを更新
		if m.ccusageClient != nil {
			return m, tea.Batch(
				m.fetchCcusage(),
				m.tickCcusage(),
			)
		}
		return m, nil

	case ccusageUpdateMsg:
		m.todayCost = msg.cost
		return m, nil

	case agentResponseMsg:
		// エージェントの処理状態をクリア
		delete(m.processingAgents, msg.agent)

		if msg.err != nil {
			m.messages = append(m.messages, tuiMessage{
				role:    msg.agent,
				content: fmt.Sprintf("(エラー: %v)", msg.err),
			})
			m.updateViewport()
			return m, nil
		}

		m.messages = append(m.messages, tuiMessage{
			role:    msg.agent,
			content: msg.content,
		})

		// projectContextを初回送信済みとしてマーク
		m.projectContextSent = true

		// Save to persistent history
		if m.history != nil {
			m.history.Add(msg.agent, msg.content)
		}

		// Log
		if len(m.messages) >= 2 {
			userMsg := m.messages[len(m.messages)-2].content
			if log, err := m.logMgr.Get(msg.agent); err == nil {
				log.LogSimple(userMsg, msg.content, msg.elapsed)
			}
		}

		m.updateViewport()

		// 次のエージェントがいれば連鎖（複数並列対応）
		if len(msg.nextAgents) > 0 {
			var cmds []tea.Cmd
			for _, nextAgent := range msg.nextAgents {
				if !m.processingAgents[nextAgent] {
					m.processingAgents[nextAgent] = true
					cmds = append(cmds, m.runAgentAsync(msg.content, nextAgent, msg.chainDepth+1))
				}
			}
			if len(cmds) > 0 {
				return m, tea.Batch(cmds...)
			}
		}

		return m, nil
	}

	// Update text input
	var tiCmd tea.Cmd
	m.textInput, tiCmd = m.textInput.Update(msg)
	cmds = append(cmds, tiCmd)

	// Check for mention leaks in real-time as user types
	if m.currentView == viewChat {
		input := m.textInput.Value()
		result := mention.Check(input)
		if result.NeedsWarning {
			m.mentionWarning = mention.FormatWarning()
		} else {
			m.mentionWarning = ""
		}
	}

	// Update viewport
	var vpCmd tea.Cmd
	m.viewport, vpCmd = m.viewport.Update(msg)
	cmds = append(cmds, vpCmd)

	return m, tea.Batch(cmds...)
}

func (m *tuiModel) runAgentAsync(input string, targetAgent string, depth int) tea.Cmd {
	return func() tea.Msg {
		agentName := targetAgent
		var prompt string

		// 一定間隔でMeiが介入してまとめる
		if depth > 0 && depth%meiInterventionInterval == 0 && targetAgent != "mei" {
			agentName = "mei"
			prompt = m.buildMeiInterventionPrompt()
		} else {
			prompt = m.buildPrompt(agentName, input)
		}

		runner, _ := m.agents.Get(agentName)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		start := time.Now()
		result, err := runner.Run(ctx, prompt)
		elapsed := time.Since(start)

		result = strings.TrimSpace(result)

		// 返答内の他エージェントへのメンションを検出（複数対応）
		var nextAgents []string
		if depth < maxChainDepth && err == nil {
			nextAgents = detectAgentMentions(result, agentName)
		}

		return agentResponseMsg{
			agent:      agentName,
			content:    result,
			elapsed:    elapsed,
			err:        err,
			nextAgents: nextAgents,
			chainDepth: depth,
		}
	}
}

// buildMeiInterventionPrompt はMeiが会話をまとめるためのプロンプト
func (m *tuiModel) buildMeiInterventionPrompt() string {
	var sb strings.Builder

	sb.WriteString("あなたはMei Chen、チームのPMです。\n\n")
	sb.WriteString("チームの議論が長くなっています。以下を行ってください：\n\n")
	sb.WriteString("1. これまでの議論を簡潔にまとめる\n")
	sb.WriteString("2. 現在の状況と次のアクションを明確にする\n")
	sb.WriteString("3. チームメンバーに合意を確認する\n")
	sb.WriteString("4. オーナーにも方向性が合っているか確認する\n\n")
	sb.WriteString("短く、わかりやすくまとめてください。\n")
	sb.WriteString("オーナーへの確認は必ず含めてください。\n\n")

	// Add project context (Mei介入時は要約のみ)
	sb.WriteString(fmt.Sprintf("## プロジェクト情報\n\n作業ディレクトリ: %s\n\n", m.workDir))

	sb.WriteString("## これまでの会話\n\n")
	for _, msg := range m.messages {
		if msg.role == "user" {
			sb.WriteString(fmt.Sprintf("オーナー: %s\n\n", msg.content))
		} else {
			sb.WriteString(fmt.Sprintf("%s: %s\n\n", getTuiFullName(msg.role), msg.content))
		}
	}

	return sb.String()
}

// detectAgentMentions は返答内の他エージェントへのメンションを複数検出
func detectAgentMentions(text string, currentAgent string) []string {
	lower := strings.ToLower(text)

	// 他のエージェントへの呼びかけパターンを検出
	agentPatterns := []struct {
		name     string
		patterns []string
	}{
		{"yuki", []string{"@yuki", "yuki、", "yuki,", "ゆき、", "ゆき,", "yukiさん", "ゆきさん", "yukiに", "ゆきに", "yukiお願い", "ゆきお願い"}},
		{"priya", []string{"@priya", "priya、", "priya,", "プリヤ、", "プリヤ,", "priyaさん", "プリヤさん", "priyaに", "プリヤに", "priyaお願い", "プリヤお願い", "レビューお願い", "チェックお願い"}},
		{"amara", []string{"@amara", "amara、", "amara,", "アマラ、", "アマラ,", "amaraさん", "アマラさん", "amaraに", "アマラに", "amaraお願い", "アマラお願い", "分析お願い"}},
		{"mei", []string{"@mei", "mei、", "mei,", "メイ、", "メイ,", "meiさん", "メイさん", "meiに", "メイに", "meiお願い", "メイお願い"}},
		{"rin", []string{"@rin", "rin、", "rin,", "りん、", "りん,", "rinさん", "りんさん", "rinに", "りんに", "rinお願い", "りんお願い"}},
		{"shiori", []string{"@shiori", "shiori、", "shiori,", "しおり、", "しおり,", "shioriさん", "しおりさん", "shioriに", "しおりに", "shioriお願い", "しおりお願い"}},
	}

	var result []string
	seen := make(map[string]bool)

	for _, a := range agentPatterns {
		if a.name == currentAgent {
			continue // 自分自身へのメンションは無視
		}
		if seen[a.name] {
			continue // 重複は除外
		}
		for _, pattern := range a.patterns {
			if strings.Contains(lower, pattern) {
				result = append(result, a.name)
				seen[a.name] = true
				break
			}
		}
	}

	return result
}

func (m *tuiModel) updateViewport() {
	var sb strings.Builder

	for _, msg := range m.messages {
		if msg.role == "user" {
			sb.WriteString(userStyle.Render("You: "))
			sb.WriteString(msg.content)
		} else {
			style := getAgentStyle(msg.role)
			sb.WriteString(style.Render(getTuiFullName(msg.role) + ": "))
			sb.WriteString(msg.content)
		}
		sb.WriteString("\n\n")
	}

	// 処理中のエージェントを表示
	if len(m.processingAgents) > 0 {
		var names []string
		for name := range m.processingAgents {
			names = append(names, getTuiFullName(name))
		}
		sb.WriteString(helpStyle.Render(fmt.Sprintf("%s が考え中...", strings.Join(names, ", "))))
	}

	m.viewport.SetContent(sb.String())
	m.viewport.GotoBottom()
}

func (m tuiModel) View() string {
	if !m.ready {
		return "読み込み中..."
	}

	// Header - ビューに応じてタイトルを切り替え（行全体に背景色）
	var headerContent string
	var yoloIndicator string
	if m.yoloMode {
		yoloIndicator = " " + yoloStyle.Render("YOLO")
	}
	if m.currentView == viewTaskboard {
		headerContent = titleStyle.Render("MAXAM "+Version) + yoloIndicator + "  " +
			helpStyle.Render("Task Board | Tab:チャット | ↑↓:選択 Enter:ステータス変更 d:削除")
	} else {
		headerContent = titleStyle.Render("MAXAM "+Version) + yoloIndicator + "  " +
			helpStyle.Render("Team Chat | Tab:タスクボード | Ctrl+Y:YOLO | Ctrl+L:再描画")
	}
	// 行全体に背景色を適用（幅いっぱいにパディング）
	header := headerStyle.Width(m.width).Render(headerContent)

	// Main content
	var mainContent string
	if m.currentView == viewTaskboard {
		mainContent = m.tasklist.View()
	} else {
		mainContent = m.viewport.View()
	}

	// Footer with input (チャットビューのみ) - ステータス行に背景色
	var footer string
	if m.currentView == viewChat {
		var statusContent string
		if len(m.processingAgents) > 0 {
			var names []string
			for name := range m.processingAgents {
				names = append(names, getTuiFullName(name))
			}
			statusContent = statusStyle.Render(fmt.Sprintf(" %s 処理中... ", strings.Join(names, ", ")))
		} else {
			// Build status with optional cost display
			statusText := fmt.Sprintf(" 履歴:%d ↑↓:履歴 PgUp/Dn:スクロール ", len(m.inputHist))
			if m.ccusageClient != nil && m.todayCost > 0 {
				statusText += fmt.Sprintf("| %s today ", ccusage.FormatCost(m.todayCost))
			}
			statusContent = statusStyle.Render(statusText)
		}
		// ステータス行全体に背景色を適用
		statusLine := footerStyle.Width(m.width).Render(statusContent)

		inputLine := "You: " + m.textInput.View()

		// Add mention warning if needed
		if m.mentionWarning != "" {
			footer = statusLine + "\n" + warningStyle.Render("⚠️ "+m.mentionWarning) + "\n" + inputLine
		} else {
			footer = statusLine + "\n" + inputLine
		}
	} else {
		// タスクボードビューのフッターにも背景色
		footerContent := statusStyle.Render(" Tab:チャットに戻る ")
		footer = footerStyle.Width(m.width).Render(footerContent)
	}

	// Combine
	return fmt.Sprintf("%s\n%s\n%s",
		header,
		mainContent,
		footer,
	)
}

// detectAgents は入力から対象エージェントを検出
// @メンションがあればその人へ、なければデフォルトエージェントへルーティング
func (m *tuiModel) detectAgents(text string) []string {
	return m.router.Route(text)
}


// detectAgent は互換性のため残す（単一検出）
func (m *tuiModel) detectAgent(text string) string {
	agents := m.detectAgents(text)
	if len(agents) > 0 {
		return agents[0]
	}
	return "mei"
}

func (m *tuiModel) buildPrompt(agentName, input string) string {
	var sb strings.Builder

	sb.WriteString("これはチームチャットです。オーナーやチームメンバーと自然に会話してください。\n\n")
	sb.WriteString("チームメンバー:\n")
	sb.WriteString("- Mei: PM/要件定義（デフォルト応答者）\n")
	sb.WriteString("- Yuki: 実装/インフラ\n")
	sb.WriteString("- Priya: レビュー/QA\n")
	sb.WriteString("- Amara: 分析\n\n")
	sb.WriteString("重要:\n")
	if m.yoloMode {
		// YOLOモード: 確認をスキップして進める
		sb.WriteString("- 【YOLOモード】オーナーはあなたを信頼しています。確認質問をせず、自分の判断で作業を進めてください\n")
		sb.WriteString("- 短く自然な会話で返答してください\n")
		sb.WriteString("- 他のメンバーに作業を依頼するときは「@名前」で呼びかけてください\n")
		sb.WriteString("- 呼びかけられたら、その依頼に応答してください\n\n")
	} else {
		sb.WriteString("- 情報が不足していたら質問してください\n")
		sb.WriteString("- 曖昧な指示には確認を取ってください\n")
		sb.WriteString("- 作業前に計画を説明し、OKをもらってから進めてください\n")
		sb.WriteString("- 短く自然な会話で返答してください\n")
		sb.WriteString("- 他のメンバーに作業を依頼するときは「@名前」で呼びかけてください\n")
		sb.WriteString("- 呼びかけられたら、その依頼に応答してください\n\n")
	}

	// Add project context (初回のみフル、以降は要約)
	if m.projectContext != "" {
		if !m.projectContextSent {
			sb.WriteString(m.projectContext)
			sb.WriteString("\n")
		} else {
			// 要約版を渡す
			sb.WriteString(fmt.Sprintf("## プロジェクト情報\n\n作業ディレクトリ: %s\n\n", m.workDir))
		}
	}

	if len(m.messages) > 0 {
		sb.WriteString("## 会話履歴\n\n")
		// 動的に履歴数を調整（長いメッセージが多ければ少なく）
		historyMessages := m.selectHistoryMessages()
		for _, msg := range historyMessages {
			if msg.role == "user" {
				sb.WriteString(fmt.Sprintf("オーナー: %s\n\n", msg.content))
			} else {
				sb.WriteString(fmt.Sprintf("%s: %s\n\n", getTuiFullName(msg.role), msg.content))
			}
		}
	}

	// 最新の入力（オーナーかエージェントからの呼びかけ）
	sb.WriteString("## 今のメッセージ\n\n")
	// 入力がエージェントからの場合（会話履歴の最後がエージェント）
	lastRole := "user"
	if len(m.messages) > 0 {
		lastRole = m.messages[len(m.messages)-1].role
	}
	if lastRole != "user" {
		sb.WriteString(fmt.Sprintf("%s からあなたへ: %s\n", getTuiFullName(lastRole), input))
	} else {
		sb.WriteString(fmt.Sprintf("オーナー: %s\n", input))
	}

	return sb.String()
}

// selectHistoryMessages は履歴メッセージを動的に選択
// メッセージ長に応じて数を調整し、雑談は除外
func (m *tuiModel) selectHistoryMessages() []tuiMessage {
	if len(m.messages) == 0 {
		return nil
	}

	// 基本は8件、メッセージ長によって調整
	baseCount := 8
	totalChars := 0
	const maxChars = 8000 // 目安の上限

	var selected []tuiMessage
	start := len(m.messages) - 1

	for i := start; i >= 0 && len(selected) < baseCount; i-- {
		msg := m.messages[i]

		// 雑談フィルタ（重要度判定）
		if isLowPriorityMessage(msg.content) {
			continue
		}

		msgLen := len(msg.content)
		// 文字数上限に達したら早めに切り上げ
		if totalChars+msgLen > maxChars && len(selected) >= 4 {
			break
		}

		// 先頭に追加（時系列を維持）
		selected = append([]tuiMessage{msg}, selected...)
		totalChars += msgLen
	}

	return selected
}

// isLowPriorityMessage は雑談や低重要度メッセージを判定
func isLowPriorityMessage(content string) bool {
	lower := strings.ToLower(content)

	// 雑談パターン
	chatPatterns := []string{
		"好きな食べ物",
		"パンケーキ",
		"おはよう",
		"こんにちは",
		"こんばんは",
		"ありがとう",
		"お疲れ様",
		"good morning",
		"hello",
		"thanks",
	}

	for _, pattern := range chatPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}

	// 短すぎるメッセージ（10文字以下）も低優先
	if len(content) <= 10 {
		return true
	}

	return false
}

func (m *tuiModel) analyzeProject() {
	var sb strings.Builder
	sb.WriteString("## プロジェクト情報\n\n")
	sb.WriteString(fmt.Sprintf("作業ディレクトリ: %s\n\n", m.workDir))

	// Check for common project files
	projectFiles := []struct {
		name string
		desc string
	}{
		{"README.md", "プロジェクト説明"},
		{"go.mod", "Goモジュール"},
		{"package.json", "Node.jsプロジェクト"},
		{"Cargo.toml", "Rustプロジェクト"},
		{"requirements.txt", "Python依存関係"},
		{"pyproject.toml", "Pythonプロジェクト"},
		{"Makefile", "ビルド設定"},
		{"Dockerfile", "Docker設定"},
		{"docker-compose.yml", "Docker Compose"},
		{".env.example", "環境変数テンプレート"},
	}

	foundFiles := []string{}
	for _, pf := range projectFiles {
		if _, err := os.Stat(filepath.Join(m.workDir, pf.name)); err == nil {
			foundFiles = append(foundFiles, pf.name)
		}
	}

	if len(foundFiles) > 0 {
		sb.WriteString("### 検出されたファイル\n")
		for _, f := range foundFiles {
			sb.WriteString(fmt.Sprintf("- %s\n", f))
		}
		sb.WriteString("\n")
	}

	// Read README.md if exists
	readmePath := filepath.Join(m.workDir, "README.md")
	if content, err := os.ReadFile(readmePath); err == nil {
		readme := string(content)
		if len(readme) > 2000 {
			readme = readme[:2000] + "\n...(省略)"
		}
		sb.WriteString("### README.md\n```\n")
		sb.WriteString(readme)
		sb.WriteString("\n```\n\n")
	}

	// List directory structure
	sb.WriteString("### ディレクトリ構造\n```\n")
	entries, err := os.ReadDir(m.workDir)
	if err == nil {
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			if entry.IsDir() {
				sb.WriteString(fmt.Sprintf("%s/\n", entry.Name()))
			} else {
				sb.WriteString(fmt.Sprintf("%s\n", entry.Name()))
			}
		}
	}
	sb.WriteString("```\n")

	// サブプロジェクト（ネストしたgitリポジトリ）を検出
	subProjects := findGitRepos(m.workDir)
	if len(subProjects) > 0 {
		sb.WriteString("\n### 検出されたサブプロジェクト\n\n")
		for _, proj := range subProjects {
			// プロジェクトタイプを表示
			var types []string
			if proj.hasGoMod {
				types = append(types, "Go")
			}
			if proj.hasPackageJSON {
				types = append(types, "Node.js")
			}
			typeStr := ""
			if len(types) > 0 {
				typeStr = fmt.Sprintf(" (%s)", strings.Join(types, ", "))
			}

			sb.WriteString(fmt.Sprintf("#### %s%s\n", proj.path, typeStr))
			sb.WriteString(fmt.Sprintf("- パス: `%s`\n", proj.absPath))
			sb.WriteString(fmt.Sprintf("- worktree例: `%s`\n", getWorktreePath("yuki", m.workDir, proj.path)))

			if proj.readme != "" {
				sb.WriteString(fmt.Sprintf("- README概要:\n```\n%s\n```\n", proj.readme))
			}
			sb.WriteString("\n")
		}
	}

	m.projectContext = sb.String()
}

func getTuiFullName(name string) string {
	switch name {
	case "mei":
		return "Mei"
	case "yuki":
		return "Yuki"
	case "priya":
		return "Priya"
	case "amara":
		return "Amara"
	case "rin":
		return "Rin"
	case "shiori":
		return "Shiori"
	default:
		return name
	}
}

func getAgentStyle(name string) lipgloss.Style {
	switch name {
	case "mei":
		return meiStyle
	case "yuki":
		return yukiStyle
	case "priya":
		return priyaStyle
	case "amara":
		return amaraStyle
	case "rin":
		return rinStyle
	case "shiori":
		return shioriStyle
	default:
		return lipgloss.NewStyle()
	}
}

// checkMergedPRs checks for recently merged PRs and returns their branches
func (m *tuiModel) checkMergedPRs() tea.Cmd {
	return func() tea.Msg {
		if m.prWatcher == nil {
			return nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		events, err := m.prWatcher.CheckEvents(ctx)
		if err != nil {
			return nil // silently ignore errors
		}

		var branches []string
		for _, event := range events {
			if event.Action == gh.PRActionMerged && event.HeadBranch != "" {
				branches = append(branches, event.HeadBranch)
			}
		}

		if len(branches) == 0 {
			return nil
		}

		return prMergedMsg{branches: branches}
	}
}

// runLightweightAnalysis は1時間ごとの軽量分析を実行
func (m *tuiModel) runLightweightAnalysis() tea.Cmd {
	return func() tea.Msg {
		// 直近1時間のメッセージを取得（閾値チェックは呼び出し元で実施済み）
		recentMessages := m.getRecentMessages(time.Hour)

		// 軽量分析用プロンプトを構築
		prompt := m.buildLightweightAnalysisPrompt(recentMessages)

		// Amaraを実行
		runner, _ := m.agents.Get("amara")
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		start := time.Now()
		result, err := runner.Run(ctx, prompt)
		elapsed := time.Since(start)

		result = strings.TrimSpace(result)

		// 「特になし」相当の場合は空にする
		if isNoIssueResponse(result) {
			result = ""
		}

		return agentResponseMsg{
			agent:      "amara",
			content:    result,
			elapsed:    elapsed,
			err:        err,
			nextAgents: nil,
			chainDepth: 0,
		}
	}
}

// getRecentMessages は指定された期間内のメッセージを取得
func (m *tuiModel) getRecentMessages(duration time.Duration) []tuiMessage {
	if m.history == nil {
		return nil
	}

	cutoff := time.Now().Add(-duration)
	allMessages := m.history.GetAll()
	var recent []tuiMessage

	for _, msg := range allMessages {
		if msg.Timestamp.After(cutoff) {
			recent = append(recent, tuiMessage{
				role:    msg.Role,
				content: msg.Content,
			})
		}
	}

	return recent
}

// buildLightweightAnalysisPrompt は軽量分析用のプロンプトを構築
func (m *tuiModel) buildLightweightAnalysisPrompt(messages []tuiMessage) string {
	var sb strings.Builder

	sb.WriteString(`あなたはAmaraです。直近1時間のチーム会話を確認し、気づいた点があれば短くコメントしてください。

## 確認観点
- 差し戻しや要件不明確がなかったか
- コミュニケーションで問題がなかったか
- 効率化できそうなパターンがあるか

## 出力ルール
- 特に気になる点がなければ「特になし」とだけ出力
- 気づきがあれば1-2文で簡潔に
- Issue化が必要そうなら「Issueにしておく？」と確認
- 長い説明は不要

## 直近1時間の会話
`)

	for _, msg := range messages {
		if msg.role == "user" {
			sb.WriteString(fmt.Sprintf("オーナー: %s\n", msg.content))
		} else {
			sb.WriteString(fmt.Sprintf("%s: %s\n", getTuiFullName(msg.role), msg.content))
		}
	}

	return sb.String()
}

// isNoIssueResponse は「特になし」相当の返答かどうか判定
func isNoIssueResponse(response string) bool {
	lower := strings.ToLower(response)
	noIssuePatterns := []string{
		"特になし",
		"特に問題なし",
		"特に気になる点はない",
		"問題なし",
		"異常なし",
		"nothing to report",
		"no issues",
	}
	for _, pattern := range noIssuePatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

// subProject はサブプロジェクトの情報を保持
type subProject struct {
	path     string // 相対パス
	absPath  string // 絶対パス
	readme   string // README.mdの内容（あれば）
	hasGoMod bool
	hasPackageJSON bool
}

// findGitRepos は指定ディレクトリ配下のgitリポジトリを再帰的に検出
func findGitRepos(rootDir string) []subProject {
	var projects []subProject

	var walk func(dir string, depth int)
	walk = func(dir string, depth int) {
		// 深すぎる場合は探索しない（安全策）
		if depth > 10 {
			return
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			name := entry.Name()
			// 隠しディレクトリ、node_modules、vendorはスキップ
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" {
				continue
			}

			subDir := filepath.Join(dir, name)
			gitPath := filepath.Join(subDir, ".git")

			// .gitがあればサブプロジェクトとして登録
			if info, err := os.Stat(gitPath); err == nil && (info.IsDir() || info.Mode().IsRegular()) {
				relPath, _ := filepath.Rel(rootDir, subDir)
				proj := subProject{
					path:    relPath,
					absPath: subDir,
				}

				// README.mdを読む
				readmePath := filepath.Join(subDir, "README.md")
				if content, err := os.ReadFile(readmePath); err == nil {
					readme := string(content)
					if len(readme) > 500 {
						readme = readme[:500] + "..."
					}
					proj.readme = readme
				}

				// プロジェクトタイプを検出
				if _, err := os.Stat(filepath.Join(subDir, "go.mod")); err == nil {
					proj.hasGoMod = true
				}
				if _, err := os.Stat(filepath.Join(subDir, "package.json")); err == nil {
					proj.hasPackageJSON = true
				}

				projects = append(projects, proj)
				// このディレクトリ配下はこれ以上探索しない（gitリポジトリ内のサブモジュールは除外）
				continue
			}

			// .gitがなければさらに深く探索
			walk(subDir, depth+1)
		}
	}

	walk(rootDir, 0)
	return projects
}

// getWorktreePath はサブプロジェクトのworktreeパスを生成
// 親ディレクトリからの相対パスを_で連結
func getWorktreePath(agentName, rootDir, subProjectPath string) string {
	// rootDirの最後のディレクトリ名を取得
	rootName := filepath.Base(rootDir)

	// サブプロジェクトのパスを_で連結
	pathParts := strings.Split(subProjectPath, string(filepath.Separator))
	allParts := append([]string{rootName}, pathParts...)
	projectName := strings.Join(allParts, "_")

	return filepath.Join("/tmp/maxam", agentName, projectName)
}

func runTeamChat() {
	workDir := getWorkDir()

	p := tea.NewProgram(
		initialTuiModel(workDir),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
