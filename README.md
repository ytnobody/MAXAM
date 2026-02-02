# MAXAM
![alt text](logo.png)
**M**ulti **A**gent e**X**ecution **A**nd **M**anagement - AIエージェントチームによる開発支援システム

## 概要

MAXAMは6人のAIエージェントが協調して開発業務を遂行するシステムです。

![MAXAMチーム](image.png)

*前列左から: Shiori Tanaka（田中 栞）、Mei Chen（メイ・チェン）、Yuki Tanaka（田中 雪）*
*後列左から: Amara Okonkwo（アマラ・オコンクォ）、Rin Sato（佐藤 凛）、Priya Sharma（プリヤ・シャルマ）*

| エージェント | 役割 | 特徴 |
|------------|------|------|
| **Mei Chen** | PM / 要件定義 | 面倒見の良いお姉さん、顧客対応が得意 |
| **Yuki Tanaka** | バックエンド / インフラ | 無口な職人肌、コードで語るタイプ |
| **Rin Sato** | フロントエンド | 元気でポジティブ、UIにこだわりあり |
| **Shiori Tanaka** | テスト / ドキュメント | 穏やかで几帳面、品質を支える縁の下の力持ち |
| **Priya Sharma** | レビュー / セキュリティ / QA | ツンデレ気質、品質に厳しい |
| **Amara Okonkwo** | 分析 | クール系参謀、データで語る |

## セットアップ

### 必要環境

- Claude Code CLI (`claude`)

### インストール方法

#### ダウンロード（推奨）

GitHub Releases からお使いの環境に合ったバイナリをダウンロード：

https://github.com/ytnobody/MAXAM/releases

#### go install

```bash
go install github.com/ytnobody/MAXAM/cmd/maxam@latest
```

#### ソースからビルド

```bash
git clone https://github.com/ytnobody/MAXAM.git
cd MAXAM
make build
```

## 使い方

### チームチャット（メイン）

```bash
./bin/maxam
```

リッチなターミナルUIでチームとチャットできます。

#### 操作方法

| 操作 | 動作 |
|------|------|
| `Tab` | チャット/タスクボード切り替え |
| `↑` / `↓` | 入力履歴をナビゲート（チャット）/ タスク選択（タスクボード） |
| `PgUp` / `PgDown` | チャットをスクロール |
| マウスホイール | チャットをスクロール |
| `Ctrl+L` | 画面再描画（表示崩れのリカバリ） |
| `Ctrl+Y` | YOLOモード切り替え（確認なしで進めるモード） |
| `Enter` | メッセージ送信（チャット）/ ステータス変更（タスクボード） |
| `d` | タスク削除（タスクボード） |
| `Esc` / `Ctrl+C` | 終了 |

#### コマンド

| コマンド | 動作 |
|----------|------|
| `exit` / `quit` | 終了 |
| `clear` | 会話をリセット |

#### 使用例

```
┌─ MAXAM Team Chat ─────────────────────────────────┐
│                                                   │
│ You: ログイン機能作りたいんだけど                   │
│                                                   │
│ Mei: いいですね！いくつか確認させてください：       │
│ 1. 認証方式は？（JWT / セッション / OAuth）        │
│ 2. ...                                            │
│                                                   │
│ You: @yuki JWTでお願い                            │
│                                                   │
│ Yuki: 了解。JWT認証で実装する。                    │
│                                                   │
├───────────────────────────────────────────────────┤
│ 履歴:2 ↑↓:履歴 PgUp/Dn:スクロール                 │
│ You: _                                            │
└───────────────────────────────────────────────────┘
```

#### 機能

- **プロジェクト自動分析**: 起動時にカレントディレクトリを分析し、README.md等の情報をエージェントと共有
- **カラー表示**: 各エージェントが異なる色で表示
- **入力履歴**: 過去の入力を↑↓キーで再利用可能
- **コンテキストモード**: トークン消費を抑えるsummaryモード対応
- **YOLOモード**: `Ctrl+Y`で切り替え。エージェントが確認質問をせず自分の判断で進める

### 設定ファイル

設定ファイルは以下の順序で検索されます：

1. **`.maxam/config.yaml`** - プロジェクトローカル（優先）
2. **`~/.maxam/config.yaml`** - ユーザーグローバル

プロジェクトごとに設定を変えたい場合は、リポジトリ直下に`.maxam/`ディレクトリを作成してください。

```bash
# プロジェクトローカル設定の作成
mkdir -p .maxam
cp ~/.maxam/config.yaml .maxam/config.yaml
```

設定例：

```yaml
# コンテキストモード: full（デフォルト）または summary
# summaryモードはCLAUDE.summary.mdを使用し、トークン消費を約80%削減
context_mode: summary

# YOLOモード: 起動時からONにする場合
yolo_mode: true

# エージェントごとのモデル指定（省略時はデフォルトモデル）
agents:
  - name: mei
    model: sonnet
  - name: yuki
    model: sonnet
  - name: priya
    model: haiku  # レビューサイクルはhaiku推奨
  - name: amara
    model: sonnet
```

| 設定項目 | 値 | 説明 |
|---------|-----|------|
| `context_mode` | `full` / `summary` | エージェントに渡すコンテキストの量 |
| `yolo_mode` | `true` / `false` | 確認なしで進めるモード（デフォルト: false） |
| `agents[].model` | `sonnet` / `haiku` / `opus` | エージェントごとに使用するモデル |

**context_modeの選び方:**
- `full`: フルコンテキストが必要な複雑な作業向け
- `summary`: 日常的な作業でコスト削減したい場合（`CLAUDE.summary.md`が必要）

**モデル選択のヒント:**
- `sonnet`: バランス型、通常の実装・分析向け
- `haiku`: 高速・低コスト、レビュー・軽量タスク向け
- `opus`: 複雑な判断が必要な場合

### プロジェクトローカル設定

プロジェクトごとに異なるチーム構成や設定を使いたい場合、プロジェクトルートに `.maxam/` ディレクトリを作成します：

```
your-project/
├── .maxam/
│   ├── config.yaml      # プロジェクト固有設定
│   ├── CLAUDE.md        # プロジェクト固有の規約・学習事項
│   └── agents/          # プロジェクト固有エージェント
│       ├── pixel/
│       │   └── CLAUDE.md
│       └── chiptune/
│           └── CLAUDE.md
└── CLAUDE.md            # 共通規約
```

**優先順位:** `プロジェクト/.maxam/ > ~/.maxam/ > デフォルト`

**例：ファミコン開発用チーム**

```yaml
# your-project/.maxam/config.yaml
version: "1"
default_agent: pixel
agents:
  - name: pixel
    full_name: Pixel Artist
    role: ドット絵担当
  - name: chiptune
    full_name: Chiptune Composer
    role: サウンド担当
  - name: asm
    full_name: ASM Wizard
    role: 6502アセンブラ担当
```

#### CLAUDE.local.md からの移行

従来の `CLAUDE.local.md` は `.maxam/CLAUDE.md` に移行することを推奨します：

```bash
# 移行手順
mkdir -p .maxam
mv CLAUDE.local.md .maxam/CLAUDE.md
```

**メリット:**
- MAXAM関連ファイルが `.maxam/` に集約される
- プロジェクトルートがすっきりする
- 「.maxamフォルダを見ればMAXAM設定が全部わかる」

> **Note:** 後方互換性のため `CLAUDE.local.md` も引き続き読み込まれますが、将来のバージョンで廃止予定です。

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
| `@yuki` / `ゆき` / `バックエンド` | Yuki |
| `@rin` / `りん` / `フロントエンド` | Rin |
| `@shiori` / `しおり` / `テスト` | Shiori |
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
│   └── maxam/          # CLIツール（チームチャット）
├── internal/
│   ├── agent/          # エージェント管理
│   ├── tui/            # ターミナルUI
│   ├── task/           # タスク管理
│   └── ...             # その他のパッケージ
├── agents/             # 各エージェントのCLAUDE.md
│   ├── mei/
│   ├── yuki/
│   ├── rin/
│   ├── shiori/
│   ├── priya/
│   └── amara/
├── logs/               # 実行ログ
└── CLAUDE.md           # 共通規約
```

### 設定ファイルの場所

| 場所 | 用途 |
|------|------|
| `~/.maxam/config.yaml` | グローバル設定 |
| `~/.maxam/agents/` | グローバルエージェント定義 |
| `プロジェクト/.maxam/config.yaml` | プロジェクト固有設定（優先） |
| `プロジェクト/.maxam/agents/` | プロジェクト固有エージェント（優先） |
| `プロジェクト/.maxam/CLAUDE.md` | プロジェクト固有の規約・学習事項 |
| `プロジェクト/CLAUDE.md` | 共通規約 |

## ワークフロー

### 基本フロー

```
ユーザー → Mei（要件整理）→ Yuki/Rin（実装）→ Shiori（テスト）→ Priya（レビュー）
                ↑                                                      │
                └─────────────────── フィードバック ───────────────────┘
```

### 自己改善

```
ログ蓄積 → Amara分析 → CLAUDE.md更新
```

### スーパーバイザー運用

チーム開発では、オーナー（スーパーバイザー）との連携が重要です。

**相談するタイミング:**
- 判断に迷ったとき
- 設計の大きな分岐点
- 要件の解釈が複数あるとき
- セキュリティに関わる問題

**気軽に聞いてOK:**
- 「〜でいいですか？」レベルで確認
- 完璧な質問でなくていい
- 大きな作業に入る前は方針を共有

```
エージェント → オーナーに相談 → 方針決定 → 実行
```

## コントリビューション

コントリビューションを歓迎します！詳しくは [CONTRIBUTING.md](CONTRIBUTING.md) をご覧ください。

## セキュリティ

セキュリティに関する問題を発見された場合は [SECURITY.md](SECURITY.md) をご確認ください。

## ライセンス

MIT License - 詳細は [LICENSE](LICENSE) を参照してください。

## 作者

ytnobody
