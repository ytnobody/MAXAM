# MAXAM 仕様書

開発者向けの内部 API リファレンスとアーキテクチャ概要。

## 目次

1. [アーキテクチャ概要](#アーキテクチャ概要)
2. [パッケージ構成](#パッケージ構成)
3. [API リファレンス](#api-リファレンス)
   - [config パッケージ](#config-パッケージ)
   - [agent パッケージ](#agent-パッケージ)
   - [mode パッケージ](#mode-パッケージ)
   - [router パッケージ](#router-パッケージ)
   - [workflow パッケージ](#workflow-パッケージ)
   - [github パッケージ](#github-パッケージ)
   - [pool パッケージ](#pool-パッケージ)
   - [heartbeat パッケージ](#heartbeat-パッケージ)

---

## アーキテクチャ概要

```
┌──────────────────────────────────────────────────────────────┐
│                           MAXAM                               │
│  ┌────────────────────────────────────────────────────────┐  │
│  │                         TUI                             │  │
│  │                    (BubbleTea)                          │  │
│  └─────────────────────────┬──────────────────────────────┘  │
│                            │                                  │
│  ┌─────────────────────────┴──────────────────────────────┐  │
│  │                    Core Layer                           │  │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐   │  │
│  │  │ Router  │  │  Mode   │  │Workflow │  │ Config  │   │  │
│  │  │         │  │ Manager │  │         │  │         │   │  │
│  │  └────┬────┘  └────┬────┘  └────┬────┘  └────┬────┘   │  │
│  │       └────────────┴────────────┴────────────┘        │  │
│  └─────────────────────────┬──────────────────────────────┘  │
│                            │                                  │
│  ┌─────────────────────────┴──────────────────────────────┐  │
│  │                   Agent Layer                           │  │
│  │  ┌─────────────────────────────────────────────────┐   │  │
│  │  │                  Agent Pool                      │   │  │
│  │  │   ┌─────┐ ┌─────┐ ┌─────┐ ┌─────┐ ┌─────┐      │   │  │
│  │  │   │ Mei │ │Yuki │ │ Rin │ │Shiori│ │Priya│ ... │   │  │
│  │  │   └──┬──┘ └──┬──┘ └──┬──┘ └──┬──┘ └──┬──┘      │   │  │
│  │  └──────┴───────┴───────┴───────┴───────┴──────────┘   │  │
│  └─────────────────────────┬──────────────────────────────┘  │
│                            │                                  │
│  ┌─────────────────────────┴──────────────────────────────┐  │
│  │                 Integration Layer                       │  │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐   │  │
│  │  │ GitHub  │  │  Slack  │  │Heartbeat│  │  Logger │   │  │
│  │  │ Client  │  │ Handler │  │ Monitor │  │         │   │  │
│  │  └─────────┘  └─────────┘  └─────────┘  └─────────┘   │  │
│  └────────────────────────────────────────────────────────┘  │
│                            │                                  │
│  ┌─────────────────────────┴──────────────────────────────┐  │
│  │                  Claude Code CLI                        │  │
│  └────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────┘
```

### レイヤー説明

| レイヤー | 役割 |
|----------|------|
| TUI | ユーザーインターフェース（BubbleTeaベース） |
| Core | ルーティング、モード管理、ワークフロー、設定 |
| Agent | エージェントプール、ワーカー管理 |
| Integration | 外部サービス連携（GitHub, Slack, ログ） |
| Claude Code CLI | AIバックエンド |

---

## パッケージ構成

```
internal/
├── agent/              # エージェント実行
│   ├── control/        # 制御コマンド（stop/resume/kill）
│   └── worker/         # ワーカー管理
├── approval/           # 承認フロー
├── auto/               # 自動実行モード
├── branch/             # ブランチ管理
├── config/             # 設定管理
├── contextmon/         # コンテキスト監視
├── errorwatch/         # エラー検出
├── github/             # GitHub API連携
├── heartbeat/          # ヘルスチェック
├── history/            # 会話履歴
├── logger/             # ログ管理
├── member/             # チームメンバー
├── mention/            # メンション解析
├── mode/               # モード管理
├── plan/               # プランニング
├── pool/               # ワーカープール
├── recovery/           # リカバリ処理
├── retry/              # リトライ機構
├── router/             # メッセージルーティング
├── slack/              # Slack連携
├── task/               # タスク管理
├── taskstatus/         # タスクステータス
├── workflow/           # ワークフロー
├── worktree/           # Git worktree
└── ws/                 # WebSocket
```

---

## API リファレンス

### config パッケージ

設定の読み込みとマージを担当。

#### 型定義

```go
// Config はMAXAMの設定を表す
type Config struct {
    Version               string           `yaml:"version"`
    TeamName              string           `yaml:"team_name,omitempty"`
    DefaultAgent          string           `yaml:"default_agent,omitempty"`
    DefaultMode           Mode             `yaml:"default_mode,omitempty"`
    Agents                []AgentConfig    `yaml:"agents"`
    AnalysisMinMessages   int              `yaml:"analysis_min_messages,omitempty"`
    ContextMode           ContextMode      `yaml:"context_mode,omitempty"`
    YOLOMode              bool             `yaml:"yolo_mode,omitempty"`
    WorkersPerAgent       int              `yaml:"workers_per_agent,omitempty"`
    MentionCheckerEnabled *bool            `yaml:"mention_checker_enabled,omitempty"`
    LogLevel              string           `yaml:"log_level,omitempty"`
    Heartbeat             *HeartbeatConfig `yaml:"heartbeat,omitempty"`
    GitHub                *GitHubConfig    `yaml:"github,omitempty"`
    WebSocket             *WebSocketConfig `yaml:"websocket,omitempty"`
}

// AgentConfig はエージェント設定を表す
type AgentConfig struct {
    Name       string `yaml:"name"`
    FullName   string `yaml:"full_name"`
    Role       string `yaml:"role"`
    Model      string `yaml:"model,omitempty"`       // opus, sonnet, haiku
    Color      string `yaml:"color,omitempty"`       // hex color code
    InstanceID string `yaml:"instance_id,omitempty"` // multi-instance用
}

// Mode は動作モードを表す
type Mode string

const (
    ModeInteractive Mode = "interactive"
    ModePlan        Mode = "plan"
    ModeAuto        Mode = "auto"
)

// ContextMode はシステムプロンプトのコンテキストモード
type ContextMode string

const (
    ContextModeFull    ContextMode = "full"
    ContextModeSummary ContextMode = "summary"
)
```

#### 主要関数

| 関数 | シグネチャ | 説明 |
|------|-----------|------|
| `Load` | `func Load() (*Config, error)` | グローバル設定を読み込む（~/.maxam/config.yaml） |
| `LoadWithProject` | `func LoadWithProject(projectDir string) (*Config, error)` | プロジェクト設定とマージして読み込む |
| `Save` | `func Save(cfg *Config) error` | グローバル設定を保存 |
| `SaveToProject` | `func SaveToProject(projectDir string, cfg *Config) error` | プロジェクト設定を保存 |
| `DefaultConfig` | `func DefaultConfig() *Config` | デフォルト設定を返す |

#### 設定マージ優先順位

```
1. デフォルト値
2. ~/.maxam/config.yaml
3. .maxam/config.yaml  ← 最優先
```

---

### agent パッケージ

エージェントの実行を管理。

#### 型定義

```go
// Model はClaude Codeのモデル指定
type Model string

const (
    ModelDefault Model = ""       // 設定に従う
    ModelHaiku   Model = "haiku"
    ModelSonnet  Model = "sonnet"
    ModelOpus    Model = "opus"
)

// Runner はClaude Code CLIを実行するラッパー
type Runner struct {
    Name        string
    WorkDir     string
    ClaudeMDDir string
    Timeout     time.Duration
    ContextMode config.ContextMode
    Model       Model
}

// Agents は複数のRunnerを管理
type Agents struct {
    runners map[string]*Runner
}
```

#### 主要メソッド

| メソッド | シグネチャ | 説明 |
|----------|-----------|------|
| `Run` | `func (r *Runner) Run(ctx context.Context, prompt string) (string, error)` | プロンプトを実行 |
| `RunWithModel` | `func (r *Runner) RunWithModel(ctx context.Context, prompt string, model Model) (string, error)` | モデル指定で実行 |
| `Get` | `func (a *Agents) Get(name string) (*Runner, bool)` | 名前でRunnerを取得 |
| `GetByRole` | `func (a *Agents) GetByRole(role string) (*Runner, bool)` | ロールでRunnerを取得 |

---

### mode パッケージ

動作モードの状態マシン。

#### 状態遷移図

```
                    EnterPlanMode()
┌─────────────┐ ──────────────────► ┌───────────┐
│ Interactive │                     │   Plan    │
│    Mode     │ ◄────────────────── │   Mode    │
└─────────────┘       Stop()        └─────┬─────┘
       ▲                                  │
       │              EnterAutoMode()     │
       │              (with approval)     ▼
       │                            ┌───────────┐
       └──────────── Stop() ─────── │   Auto    │
                                    │   Mode    │
                                    └───────────┘
```

#### 型定義

```go
// Manager はモード状態を管理
type Manager struct {
    currentMode config.Mode
    planContext *PlanContext
}

// PlanContext はプランモードのコンテキスト
type PlanContext struct {
    DiscussionURL string
    IssueNumbers  []int
    Approved      bool
}
```

#### 主要メソッド

| メソッド | シグネチャ | 説明 |
|----------|-----------|------|
| `Current` | `func (m *Manager) Current() config.Mode` | 現在のモードを返す |
| `EnterPlanMode` | `func (m *Manager) EnterPlanMode() error` | プランモードに遷移 |
| `EnterAutoMode` | `func (m *Manager) EnterAutoMode(ctx *PlanContext) error` | オートモードに遷移（承認必要） |
| `EnterAutoModeWithoutPlan` | `func (m *Manager) EnterAutoModeWithoutPlan() error` | プランなしでオートモードに遷移 |
| `Stop` | `func (m *Manager) Stop()` | インタラクティブモードに戻る |
| `IsInteractive` | `func (m *Manager) IsInteractive() bool` | インタラクティブモードか判定 |
| `IsPlan` | `func (m *Manager) IsPlan() bool` | プランモードか判定 |
| `IsAuto` | `func (m *Manager) IsAuto() bool` | オートモードか判定 |

---

### router パッケージ

メッセージのルーティング。

#### 型定義

```go
// Router はメッセージを適切なエージェントに振り分ける
type Router struct {
    agents       []AgentInfo
    defaultAgent string
    mentionRegex *regexp.Regexp
}

// AgentInfo はルーティング用のエージェント情報
type AgentInfo struct {
    Name string
    Role string
}
```

#### 主要メソッド

| メソッド | シグネチャ | 説明 |
|----------|-----------|------|
| `New` | `func New(agents []AgentInfo, defaultAgent string) *Router` | Routerを生成 |
| `Route` | `func (r *Router) Route(message string) []string` | メッセージをルーティング |

#### ルーティングロジック

1. メッセージから `@name` 形式のメンションを抽出
2. 有効なエージェント名にマッチするか確認
3. マッチしたエージェントのリストを返す
4. メンションがなければデフォルトエージェントを返す

---

### workflow パッケージ

自動ワークフローの実行。

#### ReviewCycle

```go
// ReviewCycle は開発者→レビュアーのサイクル
type ReviewCycle struct {
    agents        *agent.Agents
    logMgr        *logger.Manager
    MaxIterations int  // デフォルト: 3
}

// ReviewResult はレビューの結果
type ReviewResult struct {
    Approved    bool
    Iterations  int
    Escalated   bool
    FinalOutput string
    History     []ReviewRound
}
```

#### 主要メソッド

| メソッド | シグネチャ | 説明 |
|----------|-----------|------|
| `NewReviewCycle` | `func NewReviewCycle(agents *agent.Agents, logMgr *logger.Manager) *ReviewCycle` | レビューサイクルを生成 |
| `Run` | `func (rc *ReviewCycle) Run(ctx context.Context, task string) (*ReviewResult, error)` | レビューサイクルを実行 |

#### レビューフロー

```
Round 1: Developer実装 → Reviewerレビュー
    │
    ├── APPROVED → 完了
    ├── [MINOR] → Round 2へ
    └── [DESIGN]/[SEC-CRITICAL] → エスカレーション

Round 2: 修正 → 再レビュー（Haikuモデル使用可）
    ...

Round 3: 最終ラウンド
    └── まだAPPROVEDでない → エスカレーション
```

---

### github パッケージ

GitHub API連携。

#### 型定義

```go
// Client はGitHub APIクライアント
type Client struct {
    client *github.Client
    owner  string
    repo   string
}

// PREvent はPRイベント
type PREvent struct {
    Number     int
    Title      string
    Author     string
    Action     PRAction
    MergedAt   *time.Time
    ClosedAt   *time.Time
    URL        string
    MergedBy   string
    CreatedAt  time.Time
    HeadBranch string
}

// PRAction はPRイベントの種類
type PRAction string

const (
    PRActionMerged PRAction = "merged"
    PRActionClosed PRAction = "closed"
)

// PRReviewStatus はPRのレビュー状態
type PRReviewStatus string

const (
    PRReviewStatusPending          PRReviewStatus = "PENDING"
    PRReviewStatusApproved         PRReviewStatus = "APPROVED"
    PRReviewStatusChangesRequested PRReviewStatus = "CHANGES_REQUESTED"
)
```

#### Issue操作

| メソッド | シグネチャ | 説明 |
|----------|-----------|------|
| `CreateIssue` | `func (c *Client) CreateIssue(ctx context.Context, title, body string, labels []string) (*github.Issue, error)` | Issue作成 |
| `GetIssue` | `func (c *Client) GetIssue(ctx context.Context, number int) (*github.Issue, error)` | Issue取得 |
| `ListIssues` | `func (c *Client) ListIssues(ctx context.Context, labels []string) ([]*github.Issue, error)` | Issue一覧 |
| `CommentIssue` | `func (c *Client) CommentIssue(ctx context.Context, number int, body string) error` | コメント追加 |
| `CloseIssue` | `func (c *Client) CloseIssue(ctx context.Context, number int) error` | Issueクローズ |

#### PR操作

| メソッド | シグネチャ | 説明 |
|----------|-----------|------|
| `CreatePR` | `func (c *Client) CreatePR(ctx context.Context, title, body, head, base string) (*github.PullRequest, error)` | PR作成 |
| `GetPR` | `func (c *Client) GetPR(ctx context.Context, number int) (*github.PullRequest, error)` | PR取得 |
| `ListPRs` | `func (c *Client) ListPRs(ctx context.Context) ([]*github.PullRequest, error)` | PR一覧 |
| `ReviewPR` | `func (c *Client) ReviewPR(ctx context.Context, number int, body string, approve bool) error` | レビュー投稿 |
| `MergePR` | `func (c *Client) MergePR(ctx context.Context, number int, message string) error` | PRマージ |
| `GetPRReviewStatus` | `func (c *Client) GetPRReviewStatus(ctx context.Context, number int) (PRReviewStatus, error)` | レビュー状態取得 |
| `ListPRsAwaitingReview` | `func (c *Client) ListPRsAwaitingReview(ctx context.Context) ([]*PRWithReviewStatus, error)` | レビュー待ちPR一覧 |

#### Watcher

```go
// Watcher はPRイベントを監視
type Watcher struct {
    client    *Client
    lastCheck time.Time
}
```

| メソッド | シグネチャ | 説明 |
|----------|-----------|------|
| `CheckEvents` | `func (w *Watcher) CheckEvents(ctx context.Context) ([]PREvent, error)` | 前回チェック以降のイベント取得 |
| `CheckEventsSince` | `func (w *Watcher) CheckEventsSince(ctx context.Context, since time.Time) ([]PREvent, error)` | 指定時刻以降のイベント取得 |

---

### pool パッケージ

エージェントワーカープール。

#### 型定義

```go
// Pool はエージェントごとのワーカープール
type Pool struct {
    workers    map[string][]*Worker
    queues     map[string]chan Task
    agents     *agent.Agents
    workersNum int
    queueSize  int
}

// Config はプール設定
type Config struct {
    WorkersPerAgent int // デフォルト: 1
    QueueSize       int // デフォルト: 10
}

// Task はキュー内のタスク
type Task struct {
    AgentName string
    Prompt    string
    Result    chan Result
}

// Result はタスク実行結果
type Result struct {
    Output string
    Err    error
}

// Stats はプール統計
type Stats struct {
    AgentName    string
    TotalWorkers int
    BusyWorkers  int
    QueueLength  int
}
```

#### 主要メソッド

| メソッド | シグネチャ | 説明 |
|----------|-----------|------|
| `New` | `func New(agents *agent.Agents, cfg Config) *Pool` | プール生成 |
| `Initialize` | `func (p *Pool) Initialize(agentName string) error` | エージェントのワーカーを初期化 |
| `Dispatch` | `func (p *Pool) Dispatch(ctx context.Context, agentName, prompt string) (string, error)` | タスクをディスパッチ |
| `GetStats` | `func (p *Pool) GetStats(agentName string) (Stats, error)` | 統計取得 |
| `Close` | `func (p *Pool) Close()` | プールをシャットダウン |

---

### heartbeat パッケージ

エージェントヘルスチェック。

#### 型定義

```go
// Config はハートビート設定
type Config struct {
    Interval   time.Duration // デフォルト: 10s
    Timeout    time.Duration // デフォルト: 30s
    MaxRetries int           // デフォルト: 3
}

// Status はヘルス状態
type Status int

const (
    StatusUnknown Status = iota
    StatusHealthy
    StatusUnresponsive
    StatusDead
)

// WorkerHealth はワーカーのヘルス状態
type WorkerHealth struct {
    Name         string
    Status       Status
    LastPing     time.Time
    LastResponse time.Time
    FailCount    int
}

// Monitor はヘルスモニター
type Monitor struct {
    cfg      Config
    workers  map[string]*WorkerHealth
    pingFn   PingFunc
    onUnresp UnresponsiveCallback
}

// PingFunc はワーカーにpingを送る関数
type PingFunc func(ctx context.Context, workerName string) error

// UnresponsiveCallback はワーカー無応答時のコールバック
type UnresponsiveCallback func(workerName string, failCount int)
```

#### 主要メソッド

| メソッド | シグネチャ | 説明 |
|----------|-----------|------|
| `NewMonitor` | `func NewMonitor(cfg Config, pingFn PingFunc) *Monitor` | モニター生成 |
| `RegisterWorker` | `func (m *Monitor) RegisterWorker(name string)` | ワーカー登録 |
| `Start` | `func (m *Monitor) Start()` | 監視開始 |
| `Stop` | `func (m *Monitor) Stop()` | 監視停止 |
| `GetHealth` | `func (m *Monitor) GetHealth(name string) (WorkerHealth, bool)` | ヘルス状態取得 |
| `IsHealthy` | `func (m *Monitor) IsHealthy(name string) bool` | 正常か判定 |
| `GetUnresponsiveWorkers` | `func (m *Monitor) GetUnresponsiveWorkers() []string` | 無応答ワーカー一覧 |

---

## 関連ドキュメント

- [アーキテクチャ詳細](architecture.md)
- [コマンド一覧](commands.md)
- [設定リファレンス](configuration.md)
- [エージェント](agents.md)
- [ワークフロー](workflows.md)
