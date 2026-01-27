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

	"github.com/ytnobody/MAXAM/internal/agent"
	"github.com/ytnobody/MAXAM/internal/history"
	"github.com/ytnobody/MAXAM/internal/logger"
)

// Theme colors for each agent
// Mei: Cherry Blossom Pink - 優しいお姉さんのイメージ
// Yuki: Ice Blue - クールな職人肌
// Priya: Saffron Orange - 情熱的なツンデレ
// Amara: Royal Purple - 知的な参謀

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			Background(lipgloss.Color("236")).
			Padding(0, 1)

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

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Background(lipgloss.Color("236")).
			Padding(0, 1)
)

type tuiMessage struct {
	role    string
	content string
}

type tuiModel struct {
	agents         *agent.Agents
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
}

type agentResponseMsg struct {
	agent      string
	content    string
	elapsed    time.Duration
	err        error
	nextAgent  string // 次に発言するエージェント（連鎖用）
	chainDepth int    // 連鎖の深さ
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

	return tuiModel{
		agents:           agent.NewAgents(workDir),
		logMgr:           logger.NewManager(logger.GetDefaultLogDir(), workDir),
		history:          hist,
		workDir:          workDir,
		textInput:        ti,
		messages:         messages,
		inputHist:        make([]string, 0),
		processingAgents: make(map[string]bool),
		histIdx:          -1,
	}
}

func (m tuiModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, tea.EnterAltScreen, m.tickAnalysis())
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
			return m, tea.Quit

		case tea.KeyEnter:
			input := strings.TrimSpace(m.textInput.Value())
			if input == "" {
				return m, nil
			}

			if input == "exit" || input == "quit" {
				m.logMgr.Close()
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

		headerHeight := 3
		footerHeight := 4
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

	case analysisTickMsg:
		// 1時間ごとの軽量分析トリガー
		// Amaraが処理中でなく、会話がある場合のみ実行
		if !m.processingAgents["amara"] && len(m.messages) > 0 {
			m.processingAgents["amara"] = true
			return m, tea.Batch(
				m.runLightweightAnalysis(),
				m.tickAnalysis(), // 次のtick予約
			)
		}
		return m, m.tickAnalysis()

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

		// 次のエージェントがいれば連鎖（並列対応）
		if msg.nextAgent != "" && !m.processingAgents[msg.nextAgent] {
			m.processingAgents[msg.nextAgent] = true
			return m, m.runAgentAsync(msg.content, msg.nextAgent, msg.chainDepth+1)
		}

		return m, nil
	}

	// Update text input
	var tiCmd tea.Cmd
	m.textInput, tiCmd = m.textInput.Update(msg)
	cmds = append(cmds, tiCmd)

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

		// 返答内の他エージェントへのメンションを検出
		nextAgent := ""
		if depth < maxChainDepth && err == nil {
			nextAgent = detectAgentMention(result, agentName)
		}

		return agentResponseMsg{
			agent:      agentName,
			content:    result,
			elapsed:    elapsed,
			err:        err,
			nextAgent:  nextAgent,
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

	// Add project context
	if m.projectContext != "" {
		sb.WriteString(m.projectContext)
		sb.WriteString("\n")
	}

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

// detectAgentMention は返答内の他エージェントへのメンションを検出
func detectAgentMention(text string, currentAgent string) string {
	lower := strings.ToLower(text)

	// 他のエージェントへの呼びかけパターンを検出
	agents := []struct {
		name     string
		patterns []string
	}{
		{"yuki", []string{"@yuki", "yuki、", "yuki,", "ゆき、", "ゆき,", "yukiさん", "ゆきさん", "yukiに", "ゆきに", "yukiお願い", "ゆきお願い"}},
		{"priya", []string{"@priya", "priya、", "priya,", "プリヤ、", "プリヤ,", "priyaさん", "プリヤさん", "priyaに", "プリヤに", "priyaお願い", "プリヤお願い", "レビューお願い", "チェックお願い"}},
		{"amara", []string{"@amara", "amara、", "amara,", "アマラ、", "アマラ,", "amaraさん", "アマラさん", "amaraに", "アマラに", "amaraお願い", "アマラお願い", "分析お願い"}},
		{"mei", []string{"@mei", "mei、", "mei,", "メイ、", "メイ,", "meiさん", "メイさん", "meiに", "メイに", "meiお願い", "メイお願い"}},
	}

	for _, a := range agents {
		if a.name == currentAgent {
			continue // 自分自身へのメンションは無視
		}
		for _, pattern := range a.patterns {
			if strings.Contains(lower, pattern) {
				return a.name
			}
		}
	}

	return ""
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

	// Header
	header := titleStyle.Render("MAXAM Team Chat") + "  " +
		helpStyle.Render("Mei(default) @yuki @priya @amara | exit:終了 clear:リセット")

	// Footer with input
	var status string
	if len(m.processingAgents) > 0 {
		var names []string
		for name := range m.processingAgents {
			names = append(names, getTuiFullName(name))
		}
		status = statusStyle.Render(fmt.Sprintf(" %s 処理中... ", strings.Join(names, ", ")))
	} else {
		status = statusStyle.Render(fmt.Sprintf(" 履歴:%d ↑↓:履歴 PgUp/Dn:スクロール ", len(m.inputHist)))
	}

	inputLine := "You: " + m.textInput.View()

	footer := status + "\n" + inputLine

	// Combine
	return fmt.Sprintf("%s\n%s\n%s",
		header,
		m.viewport.View(),
		footer,
	)
}

// detectAgents は入力から対象エージェントを複数検出（並列呼び出し用）
func (m *tuiModel) detectAgents(text string) []string {
	lower := strings.ToLower(text)
	var agents []string
	seen := make(map[string]bool)

	// 明示的なメンションを優先的にチェック
	if strings.Contains(lower, "@yuki") || strings.Contains(lower, "ゆきちゃん") {
		agents = append(agents, "yuki")
		seen["yuki"] = true
	}
	if strings.Contains(lower, "@priya") || strings.Contains(lower, "プリヤちゃん") {
		agents = append(agents, "priya")
		seen["priya"] = true
	}
	if strings.Contains(lower, "@amara") || strings.Contains(lower, "アマラちゃん") {
		agents = append(agents, "amara")
		seen["amara"] = true
	}
	if strings.Contains(lower, "@mei") || strings.Contains(lower, "メイちゃん") {
		agents = append(agents, "mei")
		seen["mei"] = true
	}
	if strings.Contains(lower, "@rin") || strings.Contains(lower, "りんちゃん") {
		agents = append(agents, "rin")
		seen["rin"] = true
	}
	if strings.Contains(lower, "@shiori") || strings.Contains(lower, "しおりちゃん") {
		agents = append(agents, "shiori")
		seen["shiori"] = true
	}

	// 明示的なメンションがなければキーワードで判定（1人だけ）
	if len(agents) == 0 {
		if strings.Contains(lower, "ゆき") || strings.Contains(lower, "実装") || strings.Contains(lower, "コード書") {
			return []string{"yuki"}
		}
		if strings.Contains(lower, "プリヤ") || strings.Contains(lower, "レビュー") || strings.Contains(lower, "チェック") {
			return []string{"priya"}
		}
		if strings.Contains(lower, "アマラ") || strings.Contains(lower, "分析") || strings.Contains(lower, "傾向") {
			return []string{"amara"}
		}
		if strings.Contains(lower, "りん") || strings.Contains(lower, "フロントエンド") || strings.Contains(lower, "ui") {
			return []string{"rin"}
		}
		if strings.Contains(lower, "しおり") || strings.Contains(lower, "テスト") || strings.Contains(lower, "ドキュメント") {
			return []string{"shiori"}
		}
		// デフォルトはMei
		return []string{"mei"}
	}

	return agents
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
	sb.WriteString("- 情報が不足していたら質問してください\n")
	sb.WriteString("- 曖昧な指示には確認を取ってください\n")
	sb.WriteString("- 作業前に計画を説明し、OKをもらってから進めてください\n")
	sb.WriteString("- 短く自然な会話で返答してください\n")
	sb.WriteString("- 他のメンバーに作業を依頼するときは「@名前」で呼びかけてください\n")
	sb.WriteString("- 呼びかけられたら、その依頼に応答してください\n\n")

	// Add project context
	if m.projectContext != "" {
		sb.WriteString(m.projectContext)
		sb.WriteString("\n")
	}

	if len(m.messages) > 0 {
		sb.WriteString("## 会話履歴\n\n")
		start := 0
		if len(m.messages) > 15 {
			start = len(m.messages) - 15
		}
		for i := start; i < len(m.messages); i++ {
			msg := m.messages[i]
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
	default:
		return lipgloss.NewStyle()
	}
}

// runLightweightAnalysis は1時間ごとの軽量分析を実行
func (m *tuiModel) runLightweightAnalysis() tea.Cmd {
	return func() tea.Msg {
		// 直近1時間のメッセージを取得
		recentMessages := m.getRecentMessages(time.Hour)
		if len(recentMessages) == 0 {
			// 分析対象がなければ何もしない
			return agentResponseMsg{
				agent:   "amara",
				content: "", // 空なら表示しない
				err:     nil,
			}
		}

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
			nextAgent:  "",
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
