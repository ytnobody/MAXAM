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
	"github.com/ytnobody/MAXAM/internal/agent/control"
	"github.com/ytnobody/MAXAM/internal/agent/worker"
	"github.com/ytnobody/MAXAM/internal/approval"
	"github.com/ytnobody/MAXAM/internal/branch"
	"github.com/ytnobody/MAXAM/internal/ccusage"
	"github.com/ytnobody/MAXAM/internal/config"
	"github.com/ytnobody/MAXAM/internal/contextmon"
	"github.com/ytnobody/MAXAM/internal/errorwatch"
	gh "github.com/ytnobody/MAXAM/internal/github"
	"github.com/ytnobody/MAXAM/internal/heartbeat"
	"github.com/ytnobody/MAXAM/internal/history"
	"github.com/ytnobody/MAXAM/internal/logger"
	"github.com/ytnobody/MAXAM/internal/member"
	"github.com/ytnobody/MAXAM/internal/mention"
	"github.com/ytnobody/MAXAM/internal/mode"
	"github.com/ytnobody/MAXAM/internal/recovery"
	"github.com/ytnobody/MAXAM/internal/router"
	"github.com/ytnobody/MAXAM/internal/taskboard"
	"github.com/ytnobody/MAXAM/internal/taskstatus"
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

	// フッター行全体の背景色（控えめなダークグレー）- Interactiveモード用
	footerStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("236"))

	// Planモード用フッター背景色（青系）
	footerPlanStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#1E3A5F"))

	// Autoモード用フッター背景色（緑系）
	footerAutoStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#1E4D2B"))

	// System message style for mention warnings
	systemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFD700")).
			Bold(true)

	// Mode indicator styles
	modeInteractiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#888888")).
				Bold(false)

	modePlanStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00BFFF")).
			Bold(true)

	modeAutoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6347")).
			Bold(true)
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
	members        *member.Members
	router         *router.Router
	logMgr         *logger.Manager
	history        *history.History
	workDir        string
	projectContext string
	config         *config.Config // config for agent colors

	viewport         viewport.Model
	textInput        textinput.Model
	messages         []tuiMessage
	inputHist        []string
	histIdx          int
	tempInput        string
	ready            bool
	processingAgents map[string]bool // 各エージェントの処理状態
	width            int
	height           int

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

	// Error detection and automatic follow-up
	errorDetector *errorwatch.Detector

	// Worker pool for goroutine separation
	workerPool *worker.Pool

	// Control command handler for stop/resume
	controlHandler *control.Handler

	// Mention warning chain prevention (consecutive warning count per agent)
	mentionWarningCount map[string]int

	// Issue list on idle
	lastIssueListTime time.Time  // 最後にIssue一覧を投稿した時刻
	ghClient          *gh.Client // GitHub client for issue list

	// Mode manager for operating mode control
	modeManager *mode.Manager

	// Plan mode components
	planExecutor        *mode.PlanExecutor
	approvalWatcher     *approval.Watcher
	currentPlanIssue    int // Current plan issue number being watched
	currentPlanIssueURL string

	// Context size monitoring
	contextMonitor *contextmon.Monitor
	contextWarning string // Current context size warning message

	// Heartbeat monitoring and recovery
	heartbeatMonitor  *heartbeat.Monitor
	recoveryHandler   *recovery.Handler
	heartbeatMsgQueue chan heartbeatEventMsg // Queue for heartbeat events to process in Update

	// Task status from GitHub PRs
	taskStatusFetcher *taskstatus.Fetcher
	taskStatusLine    string // Cached formatted status line
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

const maxChainDepth = 1000                // 最大連鎖数
const meiInterventionInterval = 10        // Meiが介入する間隔
const issueListInterval = 5 * time.Minute // Issue一覧投稿の最小間隔
const issueListLimit = 20                 // Issue一覧の最大表示件数
const taskPrefix = "/task "               // 実装指示用プレフィックス

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

	// Setup ccusage client (silently disabled if not available)
	ccClient := ccusage.NewClient()

	// Load config first (needed for GitHub settings)
	cfg, err := config.LoadWithProject(workDir)
	if err != nil {
		cfg = config.DefaultConfig()
	}

	// Setup PR watcher for worktree cleanup (silently fail if no GitHub access)
	var prWatcher *gh.Watcher
	var ghClient *gh.Client
	var planExecutor *mode.PlanExecutor
	var approvalWatcher *approval.Watcher
	modeManager := mode.NewManager(config.ModeInteractive)

	var taskStatusFetcher *taskstatus.Fetcher
	if ghCfg := cfg.GetGitHubConfig(); ghCfg != nil && ghCfg.Owner != "" && ghCfg.Repo != "" {
		if client, err := gh.NewClient(ghCfg.Owner, ghCfg.Repo); err == nil {
			ghClient = client
			prWatcher = gh.NewWatcher(ghClient)

			// Setup plan mode components
			planExecutor = mode.NewPlanExecutor(ghClient, nil)
			approvalWatcher = approval.NewWatcher(ghClient)

			// Setup task status fetcher (uses underlying github client)
			taskStatusFetcher = taskstatus.NewFetcher(client.GetUnderlyingClient(), ghCfg.Owner, ghCfg.Repo)
		}
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

	// Setup mention checker with agent names from config + owner
	mentionTargets := append(agentNames, "オーナー")
	mention.SetDefaultChecker(mention.NewChecker(mentionTargets))

	members := member.NewMembers(workDir)

	// Setup agents and worker pool
	agents := agent.NewAgents(workDir)
	workerPool := worker.NewPool()
	for _, name := range agents.All() {
		if runner, ok := agents.Get(name); ok {
			w := worker.NewWorker(name, runner)
			workerPool.Add(w)
		}
	}

	// Setup control command handler
	controlHandler := control.NewHandler(workerPool)

	// Setup context monitor
	ctxMonitor := contextmon.NewMonitor(contextmon.DefaultConfig())

	// Setup heartbeat monitoring and recovery
	// Create message queue for heartbeat events (buffered to avoid blocking)
	heartbeatMsgQueue := make(chan heartbeatEventMsg, 10)

	// PingFunc: Check worker health without LLM calls
	// A worker is considered unresponsive if it's been working on a task for too long
	pingFn := func(ctx context.Context, workerName string) error {
		w, ok := workerPool.Get(workerName)
		if !ok {
			return fmt.Errorf("worker not found: %s", workerName)
		}
		// Stopped workers are intentionally stopped, not unresponsive
		if w.IsStopped() {
			return nil
		}
		// Check if task has been running too long (10 minutes threshold)
		if w.State().IsWorking() {
			duration := w.State().GetTaskDuration()
			if duration > 10*time.Minute {
				return fmt.Errorf("task running too long: %v", duration)
			}
		}
		return nil
	}

	// Heartbeat config with shorter intervals for responsiveness
	hbConfig := heartbeat.Config{
		Interval:   30 * time.Second, // Check every 30 seconds
		Timeout:    5 * time.Second,  // Ping timeout (not used for state checks)
		MaxRetries: 3,                // 3 failures before marked as dead
	}
	hbMonitor := heartbeat.NewMonitor(hbConfig, pingFn)

	// Register all workers to monitor
	for _, name := range workerPool.All() {
		hbMonitor.RegisterWorker(name)
	}

	// RestartFunc: Resume a stopped worker
	restartFn := func(ctx context.Context, workerName string) error {
		w, ok := workerPool.Get(workerName)
		if !ok {
			return fmt.Errorf("worker not found: %s", workerName)
		}
		w.Resume()
		return nil
	}

	// NotifyFunc: Queue a message to be processed in the Update loop
	notifyFn := func(ctx context.Context, target, message string) error {
		select {
		case heartbeatMsgQueue <- heartbeatEventMsg{
			workerName: target,
			message:    message,
		}:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Queue full, drop message (best effort)
			return nil
		}
	}

	// Recovery config
	recConfig := recovery.Config{
		MaxRetries:    3,
		RetryDelay:    5 * time.Second,
		NotifyTimeout: 10 * time.Second,
		PMAgent:       cfg.DefaultAgent,
		OwnerChannel:  "owner",
	}
	if recConfig.PMAgent == "" {
		recConfig.PMAgent = "mei" // fallback
	}
	recHandler := recovery.NewHandler(recConfig, restartFn, notifyFn)

	// Set unresponsive callback to trigger recovery
	hbMonitor.SetUnresponsiveCallback(func(workerName string, failCount int) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Try recovery
		recovered := recHandler.HandleUnresponsive(ctx, workerName, failCount)

		// Queue event for TUI update
		select {
		case heartbeatMsgQueue <- heartbeatEventMsg{
			workerName: workerName,
			failCount:  failCount,
			recovered:  recovered,
			message:    fmt.Sprintf("エージェント %s が無応答 (失敗回数: %d)", workerName, failCount),
		}:
		default:
			// Queue full, drop
		}
	})

	return tuiModel{
		agents:              agents,
		members:             members,
		router:              agentRouter,
		logMgr:              logger.NewManager(logger.GetDefaultLogDir(), workDir),
		history:             hist,
		workDir:             workDir,
		config:              cfg,
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
		errorDetector:       errorwatch.DefaultDetector(),
		workerPool:          workerPool,
		controlHandler:      controlHandler,
		ghClient:            ghClient,
		mentionWarningCount: make(map[string]int),
		modeManager:         modeManager,
		planExecutor:        planExecutor,
		approvalWatcher:     approvalWatcher,
		contextMonitor:      ctxMonitor,
		heartbeatMonitor:    hbMonitor,
		recoveryHandler:     recHandler,
		heartbeatMsgQueue:   heartbeatMsgQueue,
		taskStatusFetcher:   taskStatusFetcher,
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

// taskStatusTickMsg triggers periodic task status updates
type taskStatusTickMsg struct{}

// taskStatusUpdateMsg is sent when task status is updated
type taskStatusUpdateMsg struct {
	statusLine string
}

// planWorkflowMsg is sent when plan workflow completes or fails
type planWorkflowMsg struct {
	issueNumber   int
	issueURL      string
	branchName    string // milestone branch name if created
	branchCreated bool   // true if branch was newly created
	milestoneName string // human-readable milestone name
	err           error
}

// planApprovalTickMsg is sent periodically to check for plan approval
type planApprovalTickMsg struct{}

// planApprovedMsg is sent when a plan is approved
type planApprovedMsg struct {
	issueNumber int
	approvedBy  string
}

// issueListMsg is sent when issue list is fetched
type issueListMsg struct {
	issues []string
	err    error
}

// heartbeatEventMsg is sent when a worker becomes unresponsive or recovers
type heartbeatEventMsg struct {
	workerName string
	failCount  int
	recovered  bool
	message    string
}

// heartbeatTickMsg triggers periodic heartbeat check processing
type heartbeatTickMsg struct{}

func (m tuiModel) Init() tea.Cmd {
	// Start worker pool
	if m.workerPool != nil {
		m.workerPool.StartAll()
	}

	// Start heartbeat monitoring
	if m.heartbeatMonitor != nil {
		m.heartbeatMonitor.Start()
	}

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
	// Start heartbeat event processing
	if m.heartbeatMsgQueue != nil {
		cmds = append(cmds, m.processHeartbeatEvents())
	}
	// Start task status fetching
	if m.taskStatusFetcher != nil {
		cmds = append(cmds, m.fetchTaskStatus(), m.tickTaskStatus())
	}
	return tea.Batch(cmds...)
}

// tickCcusage returns a command that triggers ccusage update every 5 minutes
func (m tuiModel) tickCcusage() tea.Cmd {
	return tea.Tick(5*time.Minute, func(t time.Time) tea.Msg {
		return ccusageTickMsg{}
	})
}

// processHeartbeatEvents processes queued heartbeat events
func (m tuiModel) processHeartbeatEvents() tea.Cmd {
	return func() tea.Msg {
		if m.heartbeatMsgQueue == nil {
			return nil
		}
		// Non-blocking read from queue
		select {
		case event := <-m.heartbeatMsgQueue:
			return event
		default:
			// No events, wait a bit and check again
			time.Sleep(100 * time.Millisecond)
			select {
			case event := <-m.heartbeatMsgQueue:
				return event
			default:
				return heartbeatTickMsg{}
			}
		}
	}
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

// tickTaskStatus returns a command that triggers task status update every 2 minutes
func (m tuiModel) tickTaskStatus() tea.Cmd {
	return tea.Tick(2*time.Minute, func(t time.Time) tea.Msg {
		return taskStatusTickMsg{}
	})
}

// fetchTaskStatus fetches task status from GitHub PRs
func (m tuiModel) fetchTaskStatus() tea.Cmd {
	return func() tea.Msg {
		if m.taskStatusFetcher == nil {
			return nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		statuses, err := m.taskStatusFetcher.Fetch(ctx)
		if err != nil {
			return nil // Silently fail
		}

		// Build member name mapping from config
		memberNames := make(map[string]string)
		if m.config != nil {
			for _, agent := range m.config.Agents {
				// Map lowercase name to display name
				memberNames[agent.Name] = m.members.GetFullName(agent.Name)
			}
		}

		statusLine := taskstatus.FormatStatusLine(statuses, memberNames)
		return taskStatusUpdateMsg{statusLine: statusLine}
	}
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
			if m.workerPool != nil {
				m.workerPool.StopAll()
			}
			if m.heartbeatMonitor != nil {
				m.heartbeatMonitor.Stop()
			}
			return m, tea.Quit

		case tea.KeyCtrlL:
			// 画面再描画（表示崩れのリカバリ用）
			m.updateViewport()
			return m, tea.ClearScreen

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
				if m.workerPool != nil {
					m.workerPool.StopAll()
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

			// Handle mode commands (/plan, /stop, /status)
			if result := mode.Parse(input); result.IsCommand {
				m.textInput.SetValue("")
				return m.handleModeCommand(result)
			}

			// Handle control commands (@agent stop, @agent resume, stop all, resume all)
			if m.controlHandler != nil {
				if result := m.controlHandler.Handle(input); result != nil {
					m.textInput.SetValue("")
					icon := "✅"
					if !result.Success {
						icon = "❌"
					}
					m.messages = append(m.messages, tuiMessage{
						role:    "system",
						content: fmt.Sprintf("%s %s", icon, result.Message),
					})
					m.updateViewport()
					return m, nil
				}
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

			// /task プレフィックスで分岐: SendTask or SendChat
			// 処理中でもWorkerに送信し、作業中ならWorkerが即座にステータスを返す
			if strings.HasPrefix(input, taskPrefix) {
				// /task 〇〇 → 実装指示として SendTask を使う
				taskContent := strings.TrimPrefix(input, taskPrefix)
				targetAgents := m.detectAgents(taskContent)
				var cmds []tea.Cmd
				for _, agentName := range targetAgents {
					m.processingAgents[agentName] = true
					cmds = append(cmds, m.runTaskAsync(taskContent, agentName))
				}
				if len(cmds) > 0 {
					m.updateViewport()
					return m, tea.Batch(cmds...)
				}
				return m, nil
			}

			// 通常の会話として SendChat を使う
			// 処理中でもWorkerに送信し、作業中ならWorkerが即座にステータスを返す
			targetAgents := m.detectAgents(input)
			var cmds []tea.Cmd
			for _, agentName := range targetAgents {
				m.processingAgents[agentName] = true
				cmds = append(cmds, m.runAgentAsync(input, agentName, 0))
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

	case taskStatusTickMsg:
		// 2分ごとにタスク状況を更新
		if m.taskStatusFetcher != nil {
			return m, tea.Batch(
				m.fetchTaskStatus(),
				m.tickTaskStatus(),
			)
		}
		return m, nil

	case taskStatusUpdateMsg:
		m.taskStatusLine = msg.statusLine
		return m, nil

	case issueListMsg:
		if msg.err == nil && len(msg.issues) > 0 {
			// Issue一覧をシステムメッセージとして投稿
			var sb strings.Builder
			sb.WriteString("📋 オープンなIssue一覧:\n")
			for _, issue := range msg.issues {
				sb.WriteString(fmt.Sprintf("  %s\n", issue))
			}
			if len(msg.issues) >= issueListLimit {
				sb.WriteString(fmt.Sprintf("  ...（%d件まで表示）\n", issueListLimit))
			}
			content := sb.String()
			m.messages = append(m.messages, tuiMessage{role: "system", content: content})
			if m.history != nil {
				m.history.Add("system", content)
			}
			m.updateViewport()
		}
		return m, nil

	case planWorkflowMsg:
		if msg.err != nil {
			// Plan workflow failed
			m.modeManager.Stop() // Rollback to interactive
			m.messages = append(m.messages, tuiMessage{
				role:    "system",
				content: fmt.Sprintf("❌ Planモード開始に失敗: %s", msg.err.Error()),
			})
			m.updateViewport()
			return m, nil
		}

		// Build success message with branch info
		var statusMsg strings.Builder
		statusMsg.WriteString(fmt.Sprintf("✅ 議論用Issueを作成しました: %s\n", msg.issueURL))

		// Add branch creation status
		if msg.branchName != "" {
			if msg.branchCreated {
				statusMsg.WriteString(fmt.Sprintf("🌿 マイルストーンブランチを作成しました: `%s`\n", msg.branchName))
			} else {
				statusMsg.WriteString(fmt.Sprintf("🌿 既存のマイルストーンブランチを使用: `%s`\n", msg.branchName))
			}
		}

		statusMsg.WriteString("承認待ち中... (IssueにOK/LGTMとコメントしてください)")

		m.messages = append(m.messages, tuiMessage{
			role:    "system",
			content: statusMsg.String(),
		})
		m.updateViewport()

		// Start approval polling
		if m.approvalWatcher != nil {
			m.currentPlanIssue = msg.issueNumber
			m.currentPlanIssueURL = msg.issueURL
			m.approvalWatcher.Watch(msg.issueNumber)
		}

		return m, m.tickPlanApproval()

	case planApprovalTickMsg:
		// Check for plan approval if in plan mode
		if m.modeManager.Current() == config.ModePlan && m.approvalWatcher != nil && m.currentPlanIssue > 0 {
			return m, tea.Batch(
				m.checkPlanApproval(),
				m.tickPlanApproval(),
			)
		}
		return m, nil

	case planApprovedMsg:
		m.messages = append(m.messages, tuiMessage{
			role:    "system",
			content: fmt.Sprintf("🎉 Issue #%d が %s に承認されました！Autoモードに移行します。", msg.issueNumber, msg.approvedBy),
		})
		m.updateViewport()
		return m, nil

	case heartbeatEventMsg:
		// Handle heartbeat events (worker unresponsive/recovered)
		var icon string
		if msg.recovered {
			icon = "✅"
			m.messages = append(m.messages, tuiMessage{
				role:    "system",
				content: fmt.Sprintf("%s エージェント %s が復帰しました", icon, msg.workerName),
			})
		} else if msg.failCount > 0 {
			icon = "⚠️"
			m.messages = append(m.messages, tuiMessage{
				role:    "system",
				content: fmt.Sprintf("%s %s", icon, msg.message),
			})
		} else if msg.message != "" {
			// Notification message (PM/owner notification)
			m.messages = append(m.messages, tuiMessage{
				role:    "system",
				content: fmt.Sprintf("📢 %s", msg.message),
			})
		}
		m.updateViewport()
		// Continue processing events
		return m, m.processHeartbeatEvents()

	case heartbeatTickMsg:
		// Periodic tick - just keep polling for events
		return m, m.processHeartbeatEvents()

	case agentResponseMsg:
		// エージェントの処理状態をクリア
		delete(m.processingAgents, msg.agent)

		if msg.err != nil {
			// Log error event
			if log, err := m.logMgr.Get(msg.agent); err == nil {
				log.Error("Request failed: %v", msg.err)
			}

			m.messages = append(m.messages, tuiMessage{
				role:    msg.agent,
				content: fmt.Sprintf("(エラー: %v)", msg.err),
			})

			// Check if this is a recoverable error and send auto follow-up
			errMsg := msg.err.Error()
			followUpResult := m.errorDetector.CheckAndGenerateFollowUp(msg.agent, errMsg)
			if followUpResult.ShouldSend {
				// Get PM (default agent) to send follow-up
				pmAgent := m.config.DefaultAgent
				if pmAgent == "" {
					pmAgent = "mei" // fallback
				}

				// Add system message about auto follow-up
				m.messages = append(m.messages, tuiMessage{
					role:    "system",
					content: fmt.Sprintf("[自動声かけ] %s", followUpResult.Message),
				})

				// Add system message about auto mode switch
				if followUpResult.SwitchedToAuto {
					m.messages = append(m.messages, tuiMessage{
						role:    "system",
						content: fmt.Sprintf("[autoモード] %s をautoモードに切り替えました（手動で戻す必要があります）", msg.agent),
					})
				}

				// Trigger PM to send follow-up message
				m.processingAgents[pmAgent] = true
				m.updateViewport()
				return m, m.runAgentAsync(followUpResult.Message, pmAgent, 0)
			}

			m.updateViewport()
			return m, nil
		}

		m.messages = append(m.messages, tuiMessage{
			role:    msg.agent,
			content: msg.content,
		})

		// Check for mention leaks in agent response and add to history
		// Only enforce in Plan/Auto modes, not in Interactive mode
		m.mentionWarning = ""
		var triggerRetry bool
		if m.modeManager.ShouldEnforceMention() {
			mentionResult := mention.Check(msg.content)
			if mentionResult.NeedsWarning {
				m.mentionWarning = mention.FormatWarning()
				// Add warning to messages and history for agent responses
				warningMsg := mention.FormatSystemWarning(m.getFullName(msg.agent))
				m.messages = append(m.messages, tuiMessage{role: "system", content: warningMsg})
				if m.history != nil {
					m.history.Add("system", warningMsg)
				}

				// Increment consecutive warning count for this agent
				m.mentionWarningCount[msg.agent]++
				// Trigger retry if this is the first or second warning (allow 2 attempts)
				if m.mentionWarningCount[msg.agent] <= 2 {
					triggerRetry = true
				}
			} else {
				// Reset warning count on successful mention
				m.mentionWarningCount[msg.agent] = 0
			}
		}

		// projectContextを初回送信済みとしてマーク
		m.projectContextSent = true

		// Save to persistent history
		if m.history != nil {
			m.history.Add(msg.agent, msg.content)
		}

		// Log
		if log, err := m.logMgr.Get(msg.agent); err == nil {
			// Event log (info level)
			log.Info("Request completed in %v", msg.elapsed)

			// Detailed log (debug level)
			if len(m.messages) >= 2 {
				userMsg := m.messages[len(m.messages)-2].content
				log.LogSimple(userMsg, msg.content, msg.elapsed)
			}
		}

		m.updateViewport()

		// Mention warning retry: give agent a chance to fix the missing mention
		if triggerRetry {
			m.processingAgents[msg.agent] = true
			// Re-trigger the same agent with a hint to add mention
			retryPrompt := "[System] メンションが漏れています。宛先を @名前 で明示して、もう一度返答してください。"
			return m, m.runAgentAsync(retryPrompt, msg.agent, msg.chainDepth+1)
		}

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

		// 全エージェントがアイドルになったらIssue一覧を投稿
		if len(m.processingAgents) == 0 {
			if cmd := m.maybePostIssueList(); cmd != nil {
				return m, cmd
			}
		}

		return m, nil
	}

	// Update text input
	var tiCmd tea.Cmd
	m.textInput, tiCmd = m.textInput.Update(msg)
	cmds = append(cmds, tiCmd)

	// Check for mention leaks in real-time as user types
	// Only enforce in Plan/Auto modes, not in Interactive mode
	if m.currentView == viewChat && m.modeManager.ShouldEnforceMention() {
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

		// Log chat start event
		if log, err := m.logMgr.Get(agentName); err == nil {
			log.Info("Chat started")
		}

		// Try to use worker pool first
		if m.workerPool != nil {
			if w, ok := m.workerPool.Get(agentName); ok {
				responseChan := make(chan worker.ChatResponse, 1)
				w.SendChat(prompt, responseChan)

				// Wait for response
				resp := <-responseChan
				result := strings.TrimSpace(resp.Content)

				// 返答内の他エージェントへのメンションを検出（複数対応）
				var nextAgents []string
				if depth < maxChainDepth && resp.Err == nil {
					nextAgents = m.detectAgentMentions(result, agentName)
				}

				return agentResponseMsg{
					agent:      agentName,
					content:    result,
					elapsed:    resp.Elapsed,
					err:        resp.Err,
					nextAgents: nextAgents,
					chainDepth: depth,
				}
			}
		}

		// Fallback to direct runner execution
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
			nextAgents = m.detectAgentMentions(result, agentName)
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

// runTaskAsync は /task コマンドで呼び出される実装指示用
// SendTask を使い、エージェントに作業中フラグを立てる
func (m *tuiModel) runTaskAsync(taskContent string, targetAgent string) tea.Cmd {
	return func() tea.Msg {
		prompt := m.buildTaskPrompt(targetAgent, taskContent)

		// Log task start event
		if log, err := m.logMgr.Get(targetAgent); err == nil {
			log.Info("Task started: %s", taskContent)
		}

		// Worker pool経由でSendTask
		if m.workerPool != nil {
			if w, ok := m.workerPool.Get(targetAgent); ok {
				responseChan := make(chan worker.TaskResponse, 1)
				w.SendTask(taskContent, prompt, responseChan)

				// Wait for response
				resp := <-responseChan
				result := strings.TrimSpace(resp.Content)

				// タスク完了後、他エージェントへのメンションを検出
				var nextAgents []string
				if resp.Err == nil {
					nextAgents = m.detectAgentMentions(result, targetAgent)
				}

				return agentResponseMsg{
					agent:      targetAgent,
					content:    result,
					elapsed:    resp.Elapsed,
					err:        resp.Err,
					nextAgents: nextAgents,
					chainDepth: 0,
				}
			}
		}

		// Fallback to direct runner execution
		runner, _ := m.agents.Get(targetAgent)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		start := time.Now()
		result, err := runner.Run(ctx, prompt)
		elapsed := time.Since(start)

		result = strings.TrimSpace(result)

		var nextAgents []string
		if err == nil {
			nextAgents = m.detectAgentMentions(result, targetAgent)
		}

		return agentResponseMsg{
			agent:      targetAgent,
			content:    result,
			elapsed:    elapsed,
			err:        err,
			nextAgents: nextAgents,
			chainDepth: 0,
		}
	}
}

// buildTaskPrompt は実装指示用のプロンプトを構築
func (m *tuiModel) buildTaskPrompt(agentName, taskContent string) string {
	var sb strings.Builder

	sb.WriteString("これは実装タスクです。指示に従って作業を実行してください。\n\n")
	sb.WriteString("チームメンバー:\n")
	for i, agentCfg := range m.config.Agents {
		marker := ""
		if m.config.DefaultAgent == agentCfg.Name || (m.config.DefaultAgent == "" && i == 0) {
			marker = "（デフォルト応答者）"
		}
		sb.WriteString(fmt.Sprintf("- %s: %s%s\n", m.members.GetFullName(agentCfg.Name), agentCfg.Role, marker))
	}
	sb.WriteString("\n")

	sb.WriteString("重要:\n")
	sb.WriteString("- これは実装タスクです。会話ではなく作業を行ってください\n")
	sb.WriteString("- 作業完了後は結果を報告してください\n")
	sb.WriteString("- 他のメンバーに依頼が必要な場合は「@名前」で呼びかけてください\n\n")

	// プロジェクトコンテキスト
	if m.projectContext != "" {
		if !m.projectContextSent {
			sb.WriteString(m.projectContext)
			sb.WriteString("\n")
		} else {
			sb.WriteString(fmt.Sprintf("## プロジェクト情報\n\n作業ディレクトリ: %s\n\n", m.workDir))
		}
	}

	// 直近の会話履歴（参考用）
	if len(m.messages) > 0 {
		sb.WriteString("## 直近の会話（参考）\n\n")
		historyMessages := m.selectHistoryMessages()
		for _, msg := range historyMessages {
			if msg.role == "user" {
				sb.WriteString(fmt.Sprintf("オーナー: %s\n\n", msg.content))
			} else {
				sb.WriteString(fmt.Sprintf("%s: %s\n\n", m.getFullName(msg.role), msg.content))
			}
		}
	}

	sb.WriteString("## 実装タスク\n\n")
	sb.WriteString(fmt.Sprintf("オーナーからの指示: %s\n", taskContent))

	prompt := sb.String()

	// Check context size and update warning
	if m.contextMonitor != nil {
		status := m.contextMonitor.Check(len(prompt))
		if status.Level >= contextmon.LevelWarning {
			m.contextWarning = status.FormatWarning()
		} else {
			m.contextWarning = ""
		}
	}

	return prompt
}

// buildMeiInterventionPrompt はMeiが会話をまとめるためのプロンプト
func (m *tuiModel) buildMeiInterventionPrompt() string {
	var sb strings.Builder

	// Get PM name from config
	pmAgent := m.config.DefaultAgent
	if pmAgent == "" {
		pmAgent = "mei" // fallback
	}
	pmName := pmAgent
	if agentConfig := m.config.GetAgentByInstanceKey(pmAgent); agentConfig != nil && agentConfig.FullName != "" {
		pmName = agentConfig.FullName
	}

	sb.WriteString(fmt.Sprintf("あなたは%s、チームのPMです。\n\n", pmName))
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
			sb.WriteString(fmt.Sprintf("%s: %s\n\n", m.getFullName(msg.role), msg.content))
		}
	}

	return sb.String()
}

// detectAgentMentions は返答内の他エージェントへのメンションを複数検出
// 設定から動的にエージェント名を取得
func (m *tuiModel) detectAgentMentions(text string, currentAgent string) []string {
	lower := strings.ToLower(text)

	var result []string
	seen := make(map[string]bool)

	// 設定からエージェント名を取得して動的にパターン生成
	for _, agentCfg := range m.config.Agents {
		name := strings.ToLower(agentCfg.Name)
		if name == currentAgent {
			continue // 自分自身へのメンションは無視
		}
		if seen[name] {
			continue // 重複は除外
		}

		// @name、name、、name,などのパターンをチェック
		patterns := []string{
			"@" + name,
			name + "、",
			name + ",",
			name + "さん",
			name + "に",
			name + "お願い",
		}

		for _, pattern := range patterns {
			if strings.Contains(lower, pattern) {
				result = append(result, name)
				seen[name] = true
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
		} else if msg.role == "system" {
			sb.WriteString(systemStyle.Render(msg.content))
		} else {
			style := m.getAgentStyle(msg.role)
			sb.WriteString(style.Render(m.getFullName(msg.role) + ": "))
			sb.WriteString(msg.content)
		}
		sb.WriteString("\n\n")
	}

	// 処理中のエージェントを表示
	if len(m.processingAgents) > 0 {
		var names []string
		for name := range m.processingAgents {
			names = append(names, m.getFullName(name))
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

	// Header（行全体に背景色）
	headerContent := titleStyle.Render("MAXAM "+Version) + "  " +
		helpStyle.Render("Team Chat | Ctrl+L:再描画")
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
				names = append(names, m.members.GetFullName(name))
			}
			statusContent = statusStyle.Render(fmt.Sprintf(" %s 処理中... ", strings.Join(names, ", ")))
		} else {
			// Build status with mode indicator and optional cost display
			modeIndicator := m.renderModeIndicator()
			statusText := fmt.Sprintf(" %s 履歴:%d ↑↓:履歴 PgUp/Dn:スクロール ", modeIndicator, len(m.inputHist))
			if m.ccusageClient != nil && m.todayCost > 0 {
				statusText += fmt.Sprintf("| %s today ", ccusage.FormatCost(m.todayCost))
			}
			// Add task status if available
			if m.taskStatusLine != "" {
				statusText += fmt.Sprintf("| %s ", m.taskStatusLine)
			}
			statusContent = statusStyle.Render(statusText)
		}
		// ステータス行全体に背景色を適用（モードに応じて色を変更）
		statusLine := m.getFooterStyle().Width(m.width).Render(statusContent)

		inputLine := "You: " + m.textInput.View()

		// Build warning lines (combine mention and context warnings)
		var warningLines []string
		if m.mentionWarning != "" {
			warningLines = append(warningLines, warningStyle.Render("⚠️ "+m.mentionWarning))
		}
		if m.contextWarning != "" {
			warningLines = append(warningLines, warningStyle.Render(m.contextWarning))
		}

		if len(warningLines) > 0 {
			footer = statusLine + "\n" + strings.Join(warningLines, "\n") + "\n" + inputLine
		} else {
			footer = statusLine + "\n" + inputLine
		}
	} else {
		// タスクボードビューのフッターにも背景色（モードに応じて色を変更）
		footerContent := statusStyle.Render(" Tab:チャットに戻る ")
		footer = m.getFooterStyle().Width(m.width).Render(footerContent)
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
	// 設定からチームメンバーを動的に生成
	for i, agentCfg := range m.config.Agents {
		marker := ""
		if m.config.DefaultAgent == agentCfg.Name || (m.config.DefaultAgent == "" && i == 0) {
			marker = "（デフォルト応答者）"
		}
		sb.WriteString(fmt.Sprintf("- %s: %s%s\n", m.members.GetFullName(agentCfg.Name), agentCfg.Role, marker))
	}
	sb.WriteString("\n")
	sb.WriteString("重要:\n")
	sb.WriteString("- 情報が不足していたら質問してください\n")
	sb.WriteString("- 曖昧な指示には確認を取ってください\n")
	sb.WriteString("- 作業前に計画を説明し、OKをもらってから進めてください\n")
	sb.WriteString("- 短く自然な会話で返答してください\n")
	sb.WriteString("- 他のメンバーに作業を依頼するときは「@名前」で呼びかけてください\n")
	sb.WriteString("- 呼びかけられたら、その依頼に応答してください\n\n")

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
				sb.WriteString(fmt.Sprintf("%s: %s\n\n", m.getFullName(msg.role), msg.content))
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
		sb.WriteString(fmt.Sprintf("%s からあなたへ: %s\n", m.getFullName(lastRole), input))
	} else {
		sb.WriteString(fmt.Sprintf("オーナー: %s\n", input))
	}

	prompt := sb.String()

	// Check context size and update warning
	if m.contextMonitor != nil {
		status := m.contextMonitor.Check(len(prompt))
		if status.Level >= contextmon.LevelWarning {
			m.contextWarning = status.FormatWarning()
		} else {
			m.contextWarning = ""
		}
	}

	return prompt
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

// getFullName は設定からエージェントのフルネームを取得
func (m *tuiModel) getFullName(name string) string {
	return m.members.GetFullName(name)
}

// getAgentStyle returns the style for an agent based on config color
func (m *tuiModel) getAgentStyle(name string) lipgloss.Style {
	// Try to get color from config
	if m.config != nil {
		if color := m.config.GetAgentColor(name); color != "" {
			return lipgloss.NewStyle().
				Foreground(lipgloss.Color(color)).
				Bold(true)
		}
	}

	// Fallback to hardcoded colors for backwards compatibility
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
			sb.WriteString(fmt.Sprintf("%s: %s\n", m.getFullName(msg.role), msg.content))
		}
	}

	return sb.String()
}

// maybePostIssueList checks if we should post issue list and returns a command if so
func (m *tuiModel) maybePostIssueList() tea.Cmd {
	// GitHub clientがなければスキップ
	if m.ghClient == nil {
		return nil
	}

	// 重複防止: 前回投稿から一定時間経過していなければスキップ
	if time.Since(m.lastIssueListTime) < issueListInterval {
		return nil
	}

	// 時刻を更新（先に更新して重複を防ぐ）
	m.lastIssueListTime = time.Now()

	return m.fetchIssueList()
}

// fetchIssueList fetches open issues from GitHub
func (m *tuiModel) fetchIssueList() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		issues, err := m.ghClient.ListIssues(ctx, nil)
		if err != nil {
			return issueListMsg{err: err}
		}

		// Issue文字列に変換（上限まで）
		var result []string
		for i, issue := range issues {
			if i >= issueListLimit {
				break
			}
			// #番号 タイトル の形式
			result = append(result, fmt.Sprintf("#%d %s", issue.GetNumber(), issue.GetTitle()))
		}

		return issueListMsg{issues: result}
	}
}

// handleModeCommand はモードコマンドを処理する
func (m *tuiModel) handleModeCommand(result mode.ParseResult) (tea.Model, tea.Cmd) {
	switch result.Command {
	case mode.CommandPlan:
		// Check if plan mode components are available
		if m.planExecutor == nil || m.ghClient == nil {
			m.messages = append(m.messages, tuiMessage{
				role:    "system",
				content: "⚠️ GitHub連携が設定されていないため、Planモードを使用できません。",
			})
			m.updateViewport()
			return m, nil
		}

		// Enter plan mode first
		if err := m.modeManager.EnterPlanMode(); err != nil {
			m.messages = append(m.messages, tuiMessage{
				role:    "system",
				content: fmt.Sprintf("⚠️ %s", err.Error()),
			})
			m.updateViewport()
			return m, nil
		}

		m.messages = append(m.messages, tuiMessage{
			role:    "system",
			content: "🎯 Planモードに切り替えました。プロジェクト分析中...",
		})
		m.updateViewport()

		// Start plan workflow asynchronously
		return m, m.startPlanWorkflow()

	case mode.CommandAuto:
		// Enter auto mode directly (without plan)
		if err := m.modeManager.EnterAutoModeWithoutPlan(); err != nil {
			m.messages = append(m.messages, tuiMessage{
				role:    "system",
				content: fmt.Sprintf("⚠️ %s", err.Error()),
			})
			m.updateViewport()
			return m, nil
		}

		m.messages = append(m.messages, tuiMessage{
			role:    "system",
			content: "🚀 Autoモードに切り替えました。プランなしで自動実行モードに入ります。\n/stop でInteractiveモードに戻れます。",
		})

	case mode.CommandStop:
		// Stop approval watching if active
		if m.approvalWatcher != nil && m.currentPlanIssue > 0 {
			m.approvalWatcher.Unwatch(m.currentPlanIssue)
			m.currentPlanIssue = 0
			m.currentPlanIssueURL = ""
		}
		m.modeManager.Stop()
		m.messages = append(m.messages, tuiMessage{
			role:    "system",
			content: "💬 Interactiveモードに戻りました。",
		})

	case mode.CommandStatus:
		status := m.modeManager.StatusString()
		statusContent := fmt.Sprintf("📊 現在のモード: %s", status)

		// Show stopped agents if any
		if m.workerPool != nil {
			stoppedAgents := m.workerPool.GetStoppedWorkers()
			if len(stoppedAgents) > 0 {
				statusContent += fmt.Sprintf("\n⏸️ 停止中のエージェント: %s", strings.Join(stoppedAgents, ", "))
			}
		}

		// Show agents in auto mode
		if m.errorDetector != nil {
			autoAgents := m.errorDetector.ModeManager().AllAutoAgents()
			if len(autoAgents) > 0 {
				statusContent += fmt.Sprintf("\n🔄 autoモードのエージェント: %s", strings.Join(autoAgents, ", "))
			}
		}

		m.messages = append(m.messages, tuiMessage{
			role:    "system",
			content: statusContent,
		})

	case mode.CommandResetAuto:
		// Reset agent auto mode
		if m.errorDetector == nil {
			m.messages = append(m.messages, tuiMessage{
				role:    "system",
				content: "⚠️ エラー検出が初期化されていません。",
			})
		} else if result.Args == "" {
			// Reset all agents
			autoAgents := m.errorDetector.ModeManager().AllAutoAgents()
			if len(autoAgents) == 0 {
				m.messages = append(m.messages, tuiMessage{
					role:    "system",
					content: "ℹ️ autoモードのエージェントはいません。",
				})
			} else {
				m.errorDetector.ModeManager().ResetAll()
				m.messages = append(m.messages, tuiMessage{
					role:    "system",
					content: fmt.Sprintf("✅ 全エージェントのautoモードをリセットしました: %s", strings.Join(autoAgents, ", ")),
				})
			}
		} else {
			// Reset specific agent
			agentName := result.Args
			if m.errorDetector.IsAgentInAutoMode(agentName) {
				m.errorDetector.ResetAgentMode(agentName)
				m.messages = append(m.messages, tuiMessage{
					role:    "system",
					content: fmt.Sprintf("✅ %s のautoモードをリセットしました。", agentName),
				})
			} else {
				m.messages = append(m.messages, tuiMessage{
					role:    "system",
					content: fmt.Sprintf("ℹ️ %s はautoモードではありません。", agentName),
				})
			}
		}
	}

	m.updateViewport()
	return m, nil
}

// startPlanWorkflow starts the plan workflow asynchronously
// Analyzes project → Creates milestone branch → Creates plan issue → Starts approval polling
func (m *tuiModel) startPlanWorkflow() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		// Step 1: Analyze project
		analysis, err := m.planExecutor.AnalyzeProject(ctx)
		if err != nil {
			return planWorkflowMsg{err: fmt.Errorf("プロジェクト分析に失敗: %w", err)}
		}

		// Step 2: Generate milestone name and create branch
		milestoneName := m.generateMilestoneName(analysis)
		branchName, branchCreated, branchErr := branch.CreateMilestoneBranch(m.workDir, milestoneName)
		// Branch creation is best-effort; don't fail the workflow if it fails
		if branchErr != nil {
			// Log the error but continue
			branchName = ""
			branchCreated = false
		}

		// Step 3: Generate proposal based on analysis (include branch info)
		proposal := m.generatePlanProposal(analysis)
		if branchCreated && branchName != "" {
			proposal += fmt.Sprintf("\n\n### マイルストーンブランチ\n- `%s` を作成しました\n", branchName)
		} else if branchName != "" {
			proposal += fmt.Sprintf("\n\n### マイルストーンブランチ\n- `%s` が既に存在します\n", branchName)
		}

		// Step 4: Create plan issue
		planIssue, err := m.planExecutor.CreatePlanIssue(ctx, analysis, proposal)
		if err != nil {
			return planWorkflowMsg{err: fmt.Errorf("Issue作成に失敗: %w", err)}
		}

		return planWorkflowMsg{
			issueNumber:   planIssue.IssueNumber,
			issueURL:      planIssue.URL,
			branchName:    branchName,
			branchCreated: branchCreated,
			milestoneName: milestoneName,
		}
	}
}

// generateMilestoneName generates a milestone name based on project analysis
// Uses the oldest open issue or current date as the milestone identifier
func (m *tuiModel) generateMilestoneName(analysis *mode.ProjectAnalysis) string {
	// If there's a priority issue, use its info
	if analysis.OldestOpenIssue != nil {
		// Use format like "issue-123-fix-bug"
		title := analysis.OldestOpenIssue.Title
		// Truncate long titles
		if len(title) > 30 {
			title = title[:30]
		}
		return fmt.Sprintf("issue-%d-%s", analysis.OldestOpenIssue.Number, title)
	}

	// Fallback to date-based milestone
	return time.Now().Format("2006-01-02")
}

// generatePlanProposal generates a plan proposal based on project analysis
func (m *tuiModel) generatePlanProposal(analysis *mode.ProjectAnalysis) string {
	var sb strings.Builder

	sb.WriteString("## 次のマイルストーン提案\n\n")

	// Based on open issues
	if analysis.OpenIssues > 0 {
		sb.WriteString(fmt.Sprintf("現在 %d件のオープンIssueがあります。\n\n", analysis.OpenIssues))
	}

	if analysis.UnassignedIssues > 0 {
		sb.WriteString(fmt.Sprintf("### 未アサインのIssue: %d件\n", analysis.UnassignedIssues))
		sb.WriteString("これらのIssueへのアサインを検討してください。\n\n")
	}

	if analysis.OldestOpenIssue != nil {
		age := time.Since(analysis.OldestOpenIssue.CreatedAt)
		days := int(age.Hours() / 24)
		sb.WriteString(fmt.Sprintf("### 最優先で対応すべきIssue\n"))
		sb.WriteString(fmt.Sprintf("- #%d: %s (%d日前に作成)\n\n",
			analysis.OldestOpenIssue.Number,
			analysis.OldestOpenIssue.Title,
			days))
	}

	// Add recommendation
	sb.WriteString("### アクション案\n")
	sb.WriteString("1. 上記の優先Issueから着手する\n")
	sb.WriteString("2. 未アサインのIssueを担当者に振り分ける\n")
	sb.WriteString("3. ラベルで優先度を整理する\n")

	return sb.String()
}

// tickPlanApproval returns a command that triggers plan approval check
func (m *tuiModel) tickPlanApproval() tea.Cmd {
	return tea.Tick(30*time.Second, func(t time.Time) tea.Msg {
		return planApprovalTickMsg{}
	})
}

// checkPlanApproval checks for plan approval
func (m *tuiModel) checkPlanApproval() tea.Cmd {
	return func() tea.Msg {
		if m.approvalWatcher == nil || m.currentPlanIssue == 0 {
			return nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		events, err := m.approvalWatcher.CheckOnce(ctx)
		if err != nil {
			return nil // Silently ignore errors
		}

		for _, event := range events {
			if event.IssueNumber == m.currentPlanIssue && event.Result.Approved {
				return planApprovedMsg{
					issueNumber: event.IssueNumber,
					approvedBy:  event.Result.ApprovedBy,
				}
			}
		}

		return nil
	}
}

// renderModeIndicator はモード表示用の文字列を返す
func (m *tuiModel) renderModeIndicator() string {
	if m.modeManager == nil {
		return modeInteractiveStyle.Render("[Interactive]")
	}

	currentMode := m.modeManager.Current()
	var modeStr string
	switch currentMode {
	case config.ModePlan:
		modeStr = modePlanStyle.Render("[Plan]")
	case config.ModeAuto:
		modeStr = modeAutoStyle.Render("[Auto]")
	default:
		modeStr = modeInteractiveStyle.Render("[Interactive]")
	}

	// Show agents in auto mode (per-agent auto mode)
	if m.errorDetector != nil {
		autoAgents := m.errorDetector.ModeManager().AllAutoAgents()
		if len(autoAgents) > 0 {
			modeStr += " " + warningStyle.Render(fmt.Sprintf("🔄 auto: %s", strings.Join(autoAgents, ", ")))
		}
	}

	return modeStr
}

// getFooterStyle はモードに応じたフッタースタイルを返す
func (m *tuiModel) getFooterStyle() lipgloss.Style {
	if m.modeManager == nil {
		return footerStyle
	}

	switch m.modeManager.Current() {
	case config.ModePlan:
		return footerPlanStyle
	case config.ModeAuto:
		return footerAutoStyle
	default:
		return footerStyle
	}
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
	path           string // 相対パス
	absPath        string // 絶対パス
	readme         string // README.mdの内容（あれば）
	hasGoMod       bool
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
