# 設定リファレンス

MAXAMの設定ファイルについて詳しく説明します。

## 設定ファイルの場所

設定ファイルは以下の順序で検索されます：

1. **`.maxam/config.yaml`** - プロジェクトローカル（優先）
2. **`~/.maxam/config.yaml`** - ユーザーグローバル

プロジェクトローカル設定がある場合、グローバル設定を上書きします。

## 設定ファイルの構造

```yaml
version: "1"
team_name: "My Team"
default_agent: yuki
default_mode: interactive
context_mode: summary
yolo_mode: false
workers_per_agent: 1
mention_checker_enabled: true
analysis_min_messages: 10

agents:
  - name: yuki
    full_name: Yuki Tanaka
    role: Backend / Infrastructure
    model: sonnet
    color: "#3498db"
  - name: priya
    full_name: Priya Sharma
    role: Review / Security
    model: haiku
    color: "#e74c3c"
```

## 設定項目の詳細

### 基本設定

| 項目 | 型 | デフォルト | 説明 |
|------|-----|-----------|------|
| `version` | string | `"1"` | 設定ファイルのバージョン |
| `team_name` | string | `"Team"` | チーム名 |
| `default_agent` | string | 最初に追加されたエージェント | メンションがない場合に応答するエージェント |
| `default_mode` | string | `interactive` | 起動時のモード |

### 動作設定

| 項目 | 型 | デフォルト | 説明 |
|------|-----|-----------|------|
| `context_mode` | string | `summary` | コンテキストモード（`full` / `summary`） |
| `yolo_mode` | bool | `false` | 確認なしで自動実行するモード |
| `workers_per_agent` | int | `1` | エージェントごとの並列ワーカー数 |
| `mention_checker_enabled` | bool | `true` | メンションチェッカーの有効/無効 |
| `analysis_min_messages` | int | `10` | 分析を実行する最小メッセージ数 |

### エージェント設定

`agents` は配列で、各エージェントは以下のフィールドを持ちます：

| フィールド | 型 | 必須 | 説明 |
|-----------|-----|------|------|
| `name` | string | ✅ | エージェントの識別名（小文字、英数字） |
| `full_name` | string | ✅ | 表示名 |
| `role` | string | ✅ | 役割 |
| `model` | string | - | 使用するモデル（`sonnet` / `haiku` / `opus`） |
| `color` | string | - | 表示色（16進数カラーコード） |
| `instance_id` | string | - | マルチインスタンス時の識別子 |

## コンテキストモード

### full モード

フルコンテキストを使用します。`CLAUDE.md` の全内容がエージェントに渡されます。

```yaml
context_mode: full
```

**使用場面:**
- 複雑な作業で全ての規約を参照したい
- 初期の実装フェーズ

### summary モード

要約されたコンテキストを使用します。`CLAUDE.summary.md` が自動生成され、使用されます。

```yaml
context_mode: summary
```

**使用場面:**
- トークン消費を抑えたい（約80%削減）
- 日常的な軽量タスク

> **Note:** `CLAUDE.summary.md` は `maxam` 起動時に自動更新されます。

## 動作モード

MAXAMは3つの動作モードをサポートします。

### Interactive モード（デフォルト）

自由な会話モード。メンションなしで話しかけるとデフォルトエージェントが応答。

```yaml
default_mode: interactive
```

### Plan モード

プロジェクト分析・計画立案モード。Issue作成などを自動化。

```yaml
default_mode: plan
```

### Auto モード

承認済みの計画に基づいて自動実装。Plan モードからのみ遷移可能。

> **Note:** Auto モードに直接入ることはできません。Plan モードで計画が承認された後に遷移します。

## モデル選択

エージェントごとに使用するモデルを指定できます。

| モデル | 特徴 | 推奨用途 |
|--------|------|---------|
| `sonnet` | バランス型 | 通常の実装・分析 |
| `haiku` | 高速・低コスト | レビュー、軽量タスク |
| `opus` | 高精度・高コスト | 複雑な判断が必要な場合 |

```yaml
agents:
  - name: yuki
    model: sonnet  # 実装は Sonnet
  - name: priya
    model: haiku   # レビューは Haiku で高速化
```

> **Tip:** レビューサイクルでは Haiku を使うとコストを抑えられます。

## YOLO モード

確認なしで自動実行するモードです。開発中の試行錯誤を高速化できます。

```yaml
yolo_mode: true
```

> **Warning:** 本番環境では使用しないでください。

## プロジェクトローカル設定

プロジェクトごとに異なるチーム構成を使いたい場合：

```bash
mkdir -p .maxam
```

```yaml
# .maxam/config.yaml
version: "1"
default_agent: pixel
agents:
  - name: pixel
    full_name: Pixel Artist
    role: ドット絵担当
  - name: chiptune
    full_name: Chiptune Composer
    role: サウンド担当
```

## ディレクトリ構成

```
<project>/
├── .maxam/                  # プロジェクト設定（git管理推奨）
│   ├── config.yaml          # プロジェクト固有設定
│   ├── CLAUDE.md            # プロジェクト固有の規約・学習事項
│   └── agents/              # プロジェクト固有エージェント
│       └── <agent>/
│           └── CLAUDE.md
└── CLAUDE.md                # 共通規約

~/.maxam/                    # ユーザーグローバル設定
├── config.yaml              # グローバル設定
├── secrets.yaml             # 機密情報（git管理外）
└── agents/                  # グローバルエージェント
    └── <agent>/
        └── CLAUDE.md
```

## 設定のマージルール

プロジェクト設定とグローバル設定がマージされる際のルール：

1. **完全上書き**: `agents` 配列はプロジェクト設定が優先（マージではなく置換）
2. **値上書き**: その他のスカラー値はプロジェクト設定が優先
3. **フォールバック**: プロジェクト設定に値がない場合はグローバル設定を使用

## 関連ドキュメント

- [Getting Started](getting-started.md) - 初期設定の手順
- [エージェントのカスタマイズ](agents.md) - エージェントの詳細設定
