# Getting Started

このガイドでは、MAXAMを5分で動かせるようになることを目指します。

## 必要環境

- **Claude Code CLI** (`claude`) がインストール済みであること
- Go 1.21以上（ソースからビルドする場合のみ）

## インストール

### 方法1: バイナリをダウンロード（推奨）

GitHub Releasesから、お使いの環境に合ったバイナリをダウンロードしてください：

https://github.com/ytnobody/MAXAM/releases

```bash
# 例：Linux (amd64) の場合
curl -LO https://github.com/ytnobody/MAXAM/releases/latest/download/maxam-linux-amd64
chmod +x maxam-linux-amd64
sudo mv maxam-linux-amd64 /usr/local/bin/maxam
```

### 方法2: go install

```bash
go install github.com/ytnobody/MAXAM/cmd/maxam@latest
```

### 方法3: ソースからビルド

```bash
git clone https://github.com/ytnobody/MAXAM.git
cd MAXAM
make build
# バイナリは ./bin/maxam に生成されます
```

## 初期設定

### 1. チームを構成する

MAXAMを使う前に、チームメンバーを設定します：

```bash
maxam team init
```

対話形式で質問に答えていきます：

```
? Team name: My Project Team
? Add a team member? Yes
? Member name (e.g., alex): yuki
? Member full name (e.g., Alex Developer): Yuki Tanaka
? Member role (e.g., Backend / Frontend): Backend / Infrastructure
? Add another member? Yes
...
? Add another member? No
Team configuration saved to .maxam/config.yaml
```

### 2. チャットを開始

設定が完了したら、チームチャットを起動します：

```bash
maxam
```

リッチなターミナルUIが表示されます。

## 基本的な使い方

### チャットでエージェントと会話

```
You: ログイン機能を作りたい

Mei: いいですね！いくつか確認させてください：
     1. 認証方式は？（JWT / セッション / OAuth）
     2. ...
```

### 特定のエージェントを呼び出す

`@名前` でメンションすると、特定のエージェントが応答します：

```
You: @yuki JWTで認証機能をお願い

Yuki: 了解。JWT認証で実装する。
```

### 操作方法

| 操作 | 動作 |
|------|------|
| `Tab` | チャット/タスクボード切り替え |
| `↑` / `↓` | 入力履歴をナビゲート |
| `PgUp` / `PgDown` | チャットをスクロール |
| `Enter` | メッセージ送信 |
| `Esc` / `Ctrl+C` | 終了 |

### 便利なコマンド

| コマンド | 動作 |
|----------|------|
| `exit` / `quit` | チャットを終了 |
| `clear` | 会話をリセット |

## エージェント一覧

MAXAMには6人のAIエージェントがいます：

| エージェント | 役割 | 得意なこと |
|------------|------|-----------|
| **Mei** | PM / 要件定義 | 顧客対応、要件整理、タスク振り分け |
| **Yuki** | バックエンド / インフラ | API実装、DB設計、インフラ構築 |
| **Rin** | フロントエンド | UI/UX、React/Vue、スタイリング |
| **Shiori** | テスト / ドキュメント | テスト設計、ドキュメント作成 |
| **Priya** | レビュー / QA | コードレビュー、セキュリティ監査 |
| **Amara** | 分析 | データ分析、パターン抽出 |

## よくある質問

### Q: チームを構成しないと使えない？

デフォルトのエージェント設定が用意されているので、`maxam team init` をスキップしても使えます。ただし、プロジェクト固有のチーム構成にカスタマイズしたい場合は、`maxam team init` で設定してください。

### Q: 設定ファイルはどこに保存される？

- グローバル設定: `~/.maxam/config.yaml`
- プロジェクト固有設定: `プロジェクト/.maxam/config.yaml`

プロジェクト固有の設定が優先されます。

### Q: エラーが出て起動しない

[トラブルシューティング](troubleshooting.md) を参照してください。

## 次のステップ

- [コマンド一覧](commands.md) - 全コマンドの詳細
- [設定リファレンス](configuration.md) - 設定ファイルの詳細
- [エージェントのカスタマイズ](agents.md) - エージェントの追加・カスタマイズ方法
- [ワークフロー](workflows.md) - レビューサイクルなどの詳細
- [トラブルシューティング](troubleshooting.md) - よくある問題と解決策
