# コマンド一覧

MAXAMで使用できる全コマンドの詳細リファレンスです。

## 基本構文

```bash
maxam [command] [subcommand] [arguments] [flags]
```

## コマンド一覧

### メインコマンド

| コマンド | 説明 |
|----------|------|
| `maxam` | チームチャットTUIを起動（デフォルト） |
| `maxam chat` | エージェントと対話 |
| `maxam task` | タスク管理 |
| `maxam team` | チーム管理 |
| `maxam review` | レビューサイクルを実行 |
| `maxam ask` | エージェントに質問 |
| `maxam analyze` | 分析を実行 |
| `maxam status` | ステータス確認 |
| `maxam help` | ヘルプを表示 |

---

## maxam（引数なし）

チームチャットのTUI（Terminal User Interface）を起動します。

```bash
maxam
```

### 画面構成

```
┌─ MAXAM Team Chat ─────────────────────────────────┐
│                                                   │
│ You: ログイン機能作りたいんだけど                   │
│                                                   │
│ Mei: いいですね！いくつか確認させてください：       │
│ ...                                               │
│                                                   │
├───────────────────────────────────────────────────┤
│ You: _                                            │
└───────────────────────────────────────────────────┘
```

### 操作方法

| 操作 | 動作 |
|------|------|
| `Tab` | チャット/タスクボード切り替え |
| `↑` / `↓` | 入力履歴をナビゲート |
| `PgUp` / `PgDown` | チャットをスクロール |
| `Ctrl+L` | 画面再描画 |
| `Enter` | メッセージ送信 |
| `Esc` / `Ctrl+C` | 終了 |

### チャット内コマンド

| コマンド | 動作 |
|----------|------|
| `exit` / `quit` | チャットを終了 |
| `clear` | 会話をリセット |

---

## maxam chat

特定のエージェントと対話します。

### 構文

```bash
maxam chat <agent> [flags]
```

### 引数

| 引数 | 説明 |
|------|------|
| `<agent>` | エージェント名（`mei`, `yuki`, `rin`, `shiori`, `priya`, `amara`）または `team` |

### フラグ

| フラグ | 説明 |
|--------|------|
| `--daemon` | デーモンモード（外部連携用） |
| `--mention-check` | メンションチェッカーを有効化 |
| `--no-mention-check` | メンションチェッカーを無効化 |

### 例

```bash
# Yukiと対話
maxam chat yuki

# チーム全体と対話
maxam chat team

# デーモンモード（Slack連携等で使用）
maxam chat team --daemon
```

---

## maxam task

タスクボードを管理します。

### サブコマンド

| サブコマンド | 説明 |
|-------------|------|
| `add <title>` | タスクを追加 |
| `list` | タスク一覧を表示 |
| `delete <id>` | タスクを削除 |
| `status <id> <status>` | ステータスを変更 |
| `<description>` | タスクをYukiに委譲（レガシー） |

### ステータス

| ステータス | 説明 |
|-----------|------|
| `pending` | 未着手 |
| `in_progress` | 作業中 |
| `completed` | 完了 |

### 例

```bash
# タスクを追加
maxam task add "Implement user authentication"

# タスク一覧を表示
maxam task list

# ステータスを変更
maxam task status 1 in_progress

# タスクを削除
maxam task delete 1

# Yukiにタスクを依頼（レガシー）
maxam task "Add a login button to the header"
```

---

## maxam team

チームメンバーを管理します。

### サブコマンド

| サブコマンド | 説明 |
|-------------|------|
| `init` | 対話的にチームを初期化 |
| `add <name> <role>` | メンバーを追加 |
| `list` | メンバー一覧を表示 |
| `remove <name>` / `rm <name>` | メンバーを削除 |

### 例

```bash
# 対話的にチームを初期化
maxam team init

# メンバーを追加
maxam team add yuki "Backend / Infrastructure"

# メンバー一覧を表示
maxam team list

# メンバーを削除
maxam team remove yuki
maxam team rm yuki  # 短縮形
```

### init の対話フロー

```
? Team name: My Project Team
? Add a team member? Yes
? Member name (e.g., alex): yuki
? Member full name (e.g., Alex Developer): Yuki Tanaka
? Member role (e.g., Backend / Frontend): Backend / Infrastructure
? Add another member? No
Team configuration saved to .maxam/config.yaml
```

---

## maxam review

レビューサイクル（実装 → レビュー → 修正）を自動実行します。

### 構文

```bash
maxam review <description>
```

### 動作

1. Yuki（または developer ロールのエージェント）がタスクを実装
2. Priya（または reviewer ロールのエージェント）がレビュー
3. 問題があれば修正 → 再レビュー（最大3ラウンド）
4. 3ラウンド後もApproveされない場合はエスカレーション

### レビュータグ

| タグ | 意味 | 対応 |
|-----|------|------|
| `APPROVED` | 承認 | 完了 |
| `[MINOR]` | 軽微な問題 | 実装者が修正 |
| `[DESIGN]` | 設計問題 | エスカレーション |
| `[REQUIREMENTS]` | 要件不明確 | エスカレーション |
| `[SEC-CRITICAL]` | セキュリティ重大 | 即時エスカレーション |

### 例

```bash
maxam review "Create unit tests for the auth module"
```

### 出力例

```
MAXAM Review Cycle
==================
Task: Create unit tests for the auth module

=== Round 1 ===
[yuki] Implementing...
[yuki] Done (2.3s)
[priya] Reviewing...
[priya] Done (1.1s)
[priya] Approved!

=== Summary ===
Status: APPROVED
Iterations: 1
```

---

## maxam ask

特定のエージェントに単発の質問をします。

### 構文

```bash
maxam ask <agent> <prompt>
```

### 例

```bash
maxam ask yuki "How would you implement caching here?"
maxam ask priya "このコードにセキュリティ問題はある？"
```

---

## maxam analyze

Amara（分析担当）による分析を実行します。

### 構文

```bash
maxam analyze
```

### 動作

1. ログを収集
2. パターンを抽出
3. 改善提案を生成

### 出力例

```
MAXAM Weekly Analysis
=====================

=== Analysis Report ===
今週の主な活動:
- PRマージ数: 12
- レビュー差し戻し: 3回
...

=== Recommendations ===
1. テストカバレッジの向上を推奨
2. ...
```

---

## maxam status

チームのステータスを確認します。

### 構文

```bash
maxam status
```

### 出力例

```
MAXAM Team Status
=================

Agents:
  mei      PM / Requirements        [Ready]
  yuki     Backend / Infrastructure [Ready]
  rin      Frontend                 [Ready]
  shiori   Test / Documentation     [Ready]
  priya    Review / QA              [Ready]
  amara    Analysis                 [Ready]

Logs:
  mei: 5 log files
  yuki: 12 log files
  ...
  (location: ~/.maxam/logs/MAXAM/)
```

---

## 終了コード

| コード | 意味 |
|--------|------|
| 0 | 正常終了 |
| 1 | エラー（引数不正、エージェント不明など） |

## 関連ドキュメント

- [Getting Started](getting-started.md) - 初期設定
- [設定リファレンス](configuration.md) - 設定詳細
- [ワークフロー](workflows.md) - レビューサイクルの詳細
