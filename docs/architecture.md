# アーキテクチャ

MAXAMの内部アーキテクチャについて説明します。

## 概要

MAXAMは、複数のAIエージェントが協調して開発業務を遂行するシステムです。Claude Code CLIをラップし、エージェント間の連携・ワークフローを管理します。

```
┌─────────────────────────────────────────────────────────────────┐
│                           MAXAM                                  │
├─────────────────────────────────────────────────────────────────┤
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐        │
│  │   TUI    │  │  Router  │  │  Mode    │  │ Workflow │        │
│  │ (BubbleTea)│  │          │  │ Manager  │  │          │        │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘        │
│       │             │             │             │               │
│       └─────────────┴─────────────┴─────────────┘               │
│                           │                                      │
│  ┌────────────────────────┴────────────────────────┐            │
│  │                   Agent Pool                     │            │
│  ├──────┬──────┬──────┬──────┬──────┬──────────────┤            │
│  │ Mei  │ Yuki │ Rin  │Shiori│Priya │ Amara        │            │
│  └──────┴──────┴──────┴──────┴──────┴──────────────┘            │
│                           │                                      │
│  ┌────────────────────────┴────────────────────────┐            │
│  │                 Claude Code CLI                  │            │
│  └──────────────────────────────────────────────────┘            │
└─────────────────────────────────────────────────────────────────┘
```

## ディレクトリ構成

```
MAXAM/
├── cmd/
│   └── maxam/              # メインエントリーポイント
│       ├── main.go         # コマンドディスパッチ
│       ├── chat.go         # チャット機能
│       ├── team.go         # チーム管理
│       └── taskboard.go    # タスクボード
│
├── internal/
│   ├── agent/              # エージェント実行
│   │   ├── runner.go       # Claude Code実行ラッパー
│   │   ├── embed.go        # 埋め込みルール
│   │   └── worker/         # ワーカープール
│   │
│   ├── router/             # メッセージルーティング
│   │   └── router.go       # @メンション解析
│   │
│   ├── mode/               # 動作モード管理
│   │   ├── mode.go         # モード状態マシン
│   │   ├── plan.go         # Planモード
│   │   ├── auto_plan.go    # Autoモード
│   │   └── transition.go   # モード遷移
│   │
│   ├── workflow/           # ワークフロー
│   │   ├── review.go       # レビューサイクル
│   │   ├── analysis.go     # 分析サイクル
│   │   └── proposal.go     # 提案フロー
│   │
│   ├── config/             # 設定管理
│   │   ├── config.go       # 設定読み込み
│   │   └── init.go         # 初期化
│   │
│   ├── taskboard/          # タスクボード
│   │   ├── task.go         # タスクモデル
│   │   ├── memory.go       # メモリストア
│   │   └── file_store.go   # ファイルストア
│   │
│   ├── tui/                # ターミナルUI
│   │   └── tasklist/       # タスクリストUI
│   │
│   ├── logger/             # ログ管理
│   ├── mention/            # メンションチェック
│   ├── approval/           # 承認フロー
│   ├── plan/               # プラン分析
│   ├── github/             # GitHub連携
│   ├── slack/              # Slack連携
│   └── worktree/           # Git worktree管理
│
├── agents/                 # デフォルトエージェント定義
│   ├── mei/CLAUDE.md
│   ├── yuki/CLAUDE.md
│   ├── rin/CLAUDE.md
│   ├── shiori/CLAUDE.md
│   ├── priya/CLAUDE.md
│   └── amara/CLAUDE.md
│
└── .maxam/                 # プロジェクト設定
    ├── config.yaml
    ├── CLAUDE.md
    └── agents/
```

## コアコンポーネント

### 1. Agent Runner (`internal/agent/runner.go`)

Claude Code CLIを実行し、エージェントのペルソナを適用します。

```go
type Runner struct {
    Name        string           // エージェント名
    WorkDir     string           // 作業ディレクトリ
    ClaudeMDDir string           // CLAUDE.mdの場所
    Timeout     time.Duration    // タイムアウト
    ContextMode config.ContextMode // full / summary
    Model       Model            // haiku / sonnet / opus
}
```

**システムプロンプトの構築:**

1. 埋め込み共通ルール（EmbeddedRules）
2. プロジェクトの `CLAUDE.md`
3. `.maxam/CLAUDE.md`（プロジェクト固有）
4. エージェント固有の `CLAUDE.md`

これらを連結してシステムプロンプトとして渡します。

### 2. Router (`internal/router/router.go`)

メッセージ内の `@メンション` を解析し、どのエージェントが応答すべきか決定します。

```go
type Router struct {
    agents       []AgentInfo
    defaultAgent string
    mentionRegex *regexp.Regexp
}
```

**ルーティングロジック:**

1. `@name` 形式のメンションを抽出
2. 有効なエージェント名にマッチするか確認
3. マッチしたエージェントに振り分け
4. メンションがなければデフォルトエージェントへ

### 3. Mode Manager (`internal/mode/mode.go`)

3つの動作モードを状態マシンとして管理します。

```
┌─────────────┐    EnterPlanMode    ┌───────────┐
│ Interactive │ ──────────────────► │   Plan    │
│    Mode     │ ◄────────────────── │   Mode    │
└─────────────┘       Stop()        └─────┬─────┘
       ▲                                  │
       │              EnterAutoMode       │
       │              (with approval)     ▼
       │                            ┌───────────┐
       └──────────── Stop() ─────── │   Auto    │
                                    │   Mode    │
                                    └───────────┘
```

**モード遷移ルール:**

- **Interactive → Plan**: `EnterPlanMode()` で遷移
- **Plan → Auto**: プラン承認後、`EnterAutoMode()` で遷移
- **Any → Interactive**: `Stop()` で戻る

### 4. Review Cycle (`internal/workflow/review.go`)

開発者 → レビュアーの自動レビューサイクルを実行します。

```go
type ReviewCycle struct {
    agents        *agent.Agents
    logMgr        *logger.Manager
    MaxIterations int  // デフォルト: 3
}
```

**フロー:**

1. Developer（Yuki）が実装
2. Reviewer（Priya）がレビュー
3. タグに応じて処理:
   - `APPROVED` → 完了
   - `[MINOR]` → 修正して再レビュー
   - `[DESIGN]`/`[REQUIREMENTS]`/`[SEC-CRITICAL]` → エスカレーション
4. 最大3ラウンドで自動エスカレーション

**モデル選択ロジック:**

- Round 1: 常にデフォルト（Sonnet）
- Round 2+: 100行以下ならHaiku（高速化）

### 5. Config (`internal/config/config.go`)

設定ファイルの読み込みとマージを担当します。

```go
type Config struct {
    Version               string        `yaml:"version"`
    TeamName              string        `yaml:"team_name"`
    DefaultAgent          string        `yaml:"default_agent"`
    DefaultMode           Mode          `yaml:"default_mode"`
    ContextMode           ContextMode   `yaml:"context_mode"`
    YOLOMode              bool          `yaml:"yolo_mode"`
    WorkersPerAgent       int           `yaml:"workers_per_agent"`
    MentionCheckerEnabled *bool         `yaml:"mention_checker_enabled"`
    AnalysisMinMessages   int           `yaml:"analysis_min_messages"`
    Agents                []AgentConfig `yaml:"agents"`
}
```

**設定のマージ順:**

1. デフォルト値
2. グローバル設定（`~/.maxam/config.yaml`）
3. プロジェクト設定（`.maxam/config.yaml`）← 最優先

## データフロー

### チャットメッセージの流れ

```
ユーザー入力
    │
    ▼
┌───────────────┐
│   TUI Input   │
└───────┬───────┘
        │
        ▼
┌───────────────┐
│    Router     │ ─── @メンション解析
└───────┬───────┘
        │
        ▼
┌───────────────┐
│  Agent Pool   │ ─── 対象エージェント選択
└───────┬───────┘
        │
        ▼
┌───────────────┐
│    Runner     │ ─── Claude Code実行
└───────┬───────┘
        │
        ▼
┌───────────────┐
│    Logger     │ ─── ログ記録
└───────┬───────┘
        │
        ▼
┌───────────────┐
│  TUI Output   │
└───────────────┘
```

### レビューサイクルの流れ

```
maxam review "タスク説明"
        │
        ▼
┌───────────────┐
│ ReviewCycle   │
└───────┬───────┘
        │
        ▼
┌───────────────────────────────────────┐
│ Round 1                                │
│  ┌─────────┐       ┌─────────┐        │
│  │Developer│ ────► │Reviewer │        │
│  │  (Yuki) │       │ (Priya) │        │
│  └─────────┘       └────┬────┘        │
│                         │             │
│                  ┌──────┴──────┐      │
│                  │             │      │
│              APPROVED      [MINOR]    │
│                  │             │      │
│                  ▼             ▼      │
│               完了          Round 2   │
└───────────────────────────────────────┘
```

## 外部連携

### Claude Code CLI

MAXAMはClaude Code CLIをラップして実行します。

```bash
claude --print \
  --system-prompt "<エージェントのペルソナ>" \
  --permission-mode bypassPermissions \
  --model <haiku|sonnet|opus> \
  "<プロンプト>"
```

### GitHub連携

`internal/github/` で以下をサポート:

- Webhook受信（PRイベント等）
- PR監視
- Issue操作

### Slack連携

`internal/slack/` で以下をサポート:

- イベント受信
- メッセージ送信

## 並行処理

### ワーカープール (`internal/pool/pool.go`)

エージェントごとにワーカープールを持ち、並行処理を制御します。

```go
type Pool struct {
    workers chan struct{}
}
```

`workers_per_agent` 設定で並列度を制御。

### Worktree管理 (`internal/worktree/worktree.go`)

Git worktreeを使った並列作業環境を管理します。

```
/tmp/maxam/{agent}/{project}/
```

各エージェントが独立したworktreeで作業することで、コンフリクトなく並列作業可能。

## エラーハンドリング

### リトライ (`internal/retry/retry.go`)

一時的なエラーに対するリトライ機構。

```go
type Config struct {
    MaxAttempts     int
    InitialInterval time.Duration
    MaxInterval     time.Duration
    Multiplier      float64
}
```

### エラー検出 (`internal/errorwatch/detector.go`)

エージェントのタイムアウト等を検出し、フォローアップを促します。

## 拡張ポイント

### 新しいエージェントの追加

1. `agents/<name>/CLAUDE.md` を作成
2. または `.maxam/agents/<name>/CLAUDE.md`

### 新しいワークフローの追加

1. `internal/workflow/` に新しいワークフロー実装
2. `cmd/maxam/main.go` にコマンド追加

### 外部サービス連携

1. `internal/` に新しいパッケージ追加
2. 必要に応じてWebhookハンドラを実装

## 関連ドキュメント

- [Getting Started](getting-started.md) - 初期設定
- [設定リファレンス](configuration.md) - 設定の詳細
- [エージェントのカスタマイズ](agents.md) - エージェント追加方法
- [ワークフロー](workflows.md) - レビューサイクル等の詳細
