# MAXAM

**M**ulti **A**gent e**X**ecution **A**nd **M**anagement - AIエージェントチームによる開発支援システム

## 概要

MAXAMは4人のAIエージェントが協調して開発業務を遂行するシステムです。

| エージェント | 役割 | 特徴 |
|------------|------|------|
| **Mei Chen** | PM / 要件定義 | 面倒見の良いお姉さん、顧客対応が得意 |
| **Yuki Tanaka** | 実装 / インフラ | 無口な職人肌、コードで語るタイプ |
| **Priya Sharma** | レビュー / セキュリティ / QA | ツンデレ気質、品質に厳しい |
| **Amara Okonkwo** | 分析 | クール系参謀、データで語る |

## セットアップ

### 必要環境

- Go 1.23+
- Claude Code CLI (`claude`)

### ビルド

```bash
git clone https://github.com/ytnobody/MAXAM.git
cd MAXAM
go build -o bin/maxam ./cmd/maxam
go build -o bin/orchestrator ./cmd/orchestrator
```

## 使い方

### CLI モード

```bash
# タスクをYukiに依頼
./bin/maxam task "ログイン機能を実装して"

# レビューサイクル実行 (Yuki実装 → Priyaレビュー)
./bin/maxam review "Add関数のユニットテストを作成して"

# 特定のエージェントに質問
./bin/maxam ask mei "このプロジェクトの状況を教えて"
./bin/maxam ask yuki "キャッシュの実装方法は？"
./bin/maxam ask priya "このコードにセキュリティ上の問題はある？"
./bin/maxam ask amara "最近の傾向を分析して"

# Amaraの週次分析を実行
./bin/maxam analyze

# チームステータス確認
./bin/maxam status
```

### サーバーモード

```bash
./bin/orchestrator
```

サーバーモードでは以下が有効になります：
- GitHub Webhook受信 (`:8080/webhook`)
- ヘルスチェック (`:8080/health`)
- ステータスAPI (`:8080/status`)
- comms/ディレクトリの監視
- Slack連携（環境変数設定時）

## 環境変数

### GitHub連携

```bash
export GITHUB_TOKEN="ghp_..."
export GITHUB_OWNER="ytnobody"
export GITHUB_REPO="MAXAM"
export WEBHOOK_SECRET="your-webhook-secret"
```

### Slack連携

```bash
export SLACK_BOT_TOKEN="xoxb-..."
export SLACK_APP_TOKEN="xapp-..."
export SLACK_WAIT_SECONDS=120  # メッセージ待機時間（デフォルト120秒）
```

## ディレクトリ構造

```
MAXAM/
├── cmd/
│   ├── maxam/          # CLIツール
│   └── orchestrator/   # サーバーモード
├── internal/
│   ├── agent/          # エージェント管理
│   ├── comms/          # エージェント間通信
│   ├── github/         # GitHub API連携
│   ├── logger/         # ログシステム
│   ├── mcp/            # MCPプロトコル
│   ├── slack/          # Slack連携
│   └── workflow/       # ワークフロー（レビューサイクル等）
├── agents/             # 各エージェントのCLAUDE.md
│   ├── mei/
│   ├── yuki/
│   ├── priya/
│   └── amara/
├── comms/              # エージェント間メッセージ
├── logs/               # 実行ログ
└── CLAUDE.md           # 共通規約
```

## ワークフロー

### レビューサイクル

```
1. タスク投入
      ↓
2. Yuki が実装
      ↓
3. Priya がレビュー
      ↓
   ┌─ APPROVED → 完了
   │
   └─ Request Changes
         ↓
      ┌─ [MINOR] → Yukiが修正 → 3へ戻る
      │
      └─ [DESIGN]/[REQUIREMENTS]/[SEC-CRITICAL]
            ↓
         Meiへエスカレーション
```

### 自己改善サイクル

```
ログ蓄積 → Amara週次分析 → CLAUDE.md更新提案 → 適用
```

## 通信規約

エージェント間の通信は `comms/` ディレクトリのMarkdownファイルで行われます。

```markdown
## [2026-01-27 12:00] From: yuki To: priya

### Subject
PR Review Request

### Body
実装内容...

### Action Required
レビューお願いします

---
```

## エスカレーションルール

- 同一PR差し戻し3回超過 → Meiへエスカレーション
- `[SEC-CRITICAL]` タグ → 即ブロック、Meiへ
- 要件不明確 → Mei経由で顧客確認

## ライセンス

Private

## 作者

- Human Supervisor + MAXAM AI Team
