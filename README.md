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

### チームチャット（メイン）

```bash
./bin/orchestrator
```

起動すると、チームとチャットできます：

```
MAXAM Team Chat
===============

チームメンバー:
  Mei    - PM/要件定義 (デフォルト)
  Yuki   - 実装/インフラ (@yuki)
  Priya  - レビュー/QA (@priya)
  Amara  - 分析 (@amara)

使い方: 普通に話しかけてください。@yukiなどでメンション可。
終了: exit
--------------------------------------------------

You: ログイン機能作りたいんだけど

Mei: いいですね！いくつか確認させてください：
1. 認証方式は？（JWT / セッション / OAuth）
2. ...

You: @yuki JWTでお願い

Yuki: 了解。JWT認証で実装する。
...
```

### CLIコマンド

```bash
# 対話モード
./bin/maxam chat yuki    # Yukiと1対1で対話
./bin/maxam chat team    # チーム全体と対話

# 単発コマンド
./bin/maxam task "ログイン機能を実装して"
./bin/maxam review "テストを書いて"
./bin/maxam ask priya "このコード大丈夫？"
./bin/maxam analyze      # Amaraの分析
./bin/maxam status       # ステータス確認
```

## エージェントへの指示

### メンション

| メンション | エージェント |
|-----------|-------------|
| `@yuki` / `ゆき` / `実装` | Yuki |
| `@priya` / `プリヤ` / `レビュー` | Priya |
| `@amara` / `アマラ` / `分析` | Amara |
| なし | Mei (デフォルト) |

### 自然に話しかける

```
You: 新しいAPI作りたい
Mei: どんなAPIですか？詳しく教えてください。

You: ユーザー一覧を返すGET /users
Mei: 了解です。Yukiに実装を依頼しますね。@yuki、お願いできる？

You: @yuki お願い
Yuki: 了解。GET /users 作る。DBスキーマはある？
```

情報が足りないときはエージェントが質問してきます。

## ディレクトリ構造

```
MAXAM/
├── cmd/
│   ├── maxam/          # CLIツール
│   └── orchestrator/   # チームチャット
├── internal/
│   ├── agent/          # エージェント管理
│   ├── workflow/       # ワークフロー
│   └── logger/         # ログ
├── agents/             # 各エージェントのCLAUDE.md
│   ├── mei/
│   ├── yuki/
│   ├── priya/
│   └── amara/
├── logs/               # 実行ログ
└── CLAUDE.md           # 共通規約
```

## ワークフロー

### 基本フロー

```
ユーザー → Mei（要件整理）→ Yuki（実装）→ Priya（レビュー）
                ↑                              │
                └──────── フィードバック ───────┘
```

### 自己改善

```
ログ蓄積 → Amara分析 → CLAUDE.md更新
```

## ライセンス

Private

## 作者

ytnobody
