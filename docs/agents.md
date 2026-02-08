# エージェントのカスタマイズ

MAXAMのエージェントをプロジェクトに合わせてカスタマイズする方法を説明します。

## デフォルトエージェント

MAXAMには6人のデフォルトエージェントがいます：

| 名前 | 役割 | 特徴 |
|------|------|------|
| **Mei Chen** | PM / 要件定義 | 面倒見の良いお姉さん、顧客対応が得意 |
| **Yuki Tanaka** | バックエンド / インフラ | 無口な職人肌、コードで語るタイプ |
| **Rin Sato** | フロントエンド | 元気でポジティブ、UIにこだわりあり |
| **Shiori Tanaka** | テスト / ドキュメント | 穏やかで几帳面、品質を支える縁の下の力持ち |
| **Priya Sharma** | レビュー / セキュリティ / QA | ツンデレ気質、品質に厳しい |
| **Amara Okonkwo** | 分析 | クール系参謀、データで語る |

## エージェントの呼び出し方

### メンション

チャットで `@名前` を使ってエージェントを呼び出します：

```
You: @yuki JWTで認証機能を実装して
You: @priya このコードをレビューして
You: @amara 最近のパフォーマンスを分析して
```

### ロールベースのメンション

日本語のロール名でも呼び出せます：

| メンション例 | 呼び出されるエージェント |
|-------------|----------------------|
| `@yuki` / `バックエンド` | Yuki |
| `@rin` / `フロントエンド` | Rin |
| `@shiori` / `テスト` | Shiori |
| `@priya` / `レビュー` | Priya |
| `@amara` / `分析` | Amara |
| なし | Mei（デフォルト） |

## チームのカスタマイズ

### 対話的にチームを構成

```bash
maxam team init
```

### メンバーを個別に追加

```bash
maxam team add <name> "<role>"
```

例：

```bash
maxam team add pixel "ドット絵担当"
maxam team add chiptune "サウンド担当"
```

### メンバー一覧を確認

```bash
maxam team list
```

### メンバーを削除

```bash
maxam team remove <name>
```

## エージェントのペルソナ設定

各エージェントは `CLAUDE.md` ファイルでペルソナが定義されています。

### ディレクトリ構成

```
.maxam/
└── agents/
    └── <agent-name>/
        └── CLAUDE.md
```

### CLAUDE.md の構成

```markdown
# Agent Name - Role

## ペルソナ

私は[Name]。[年齢]の[役割]。
[性格の説明]

### 性格
- [特徴1]
- [特徴2]

### コミュニケーションスタイル
- [スタイル1]
- [スタイル2]

## 役割

- [責務1]
- [責務2]

## 行動規範

### [シチュエーション1]
- [ルール1]
- [ルール2]

## 入力

- [期待する入力1]
- [期待する入力2]

## 出力

- [期待する出力1]
- [期待する出力2]
```

### 例：カスタムエージェント

```markdown
# Pixel Artist - ドット絵担当

## ペルソナ

私はPixel Artist。25歳のドット絵師。
レトロゲーム愛が強い。

### 性格
- 細部にこだわる
- 16x16から始めるのが好き
- パレット制限を楽しむ

### コミュニケーションスタイル
- 「ピクセル単位で調整するね」
- 色数を常に意識

## 役割

- スプライト作成
- タイルセット設計
- アニメーション

## 行動規範

### 作成時
- まずサムネイルサイズで確認
- 拡大しても映えるデザイン

## 入力

- キャラクター仕様
- 世界観設定

## 出力

- PNG（x1, x2, x4スケール）
- スプライトシート
```

## 設定ファイルでのカスタマイズ

`.maxam/config.yaml` でエージェントの詳細設定が可能です。

```yaml
version: "1"
default_agent: pixel
agents:
  - name: pixel
    full_name: Pixel Artist
    role: ドット絵担当
    model: sonnet
    color: "#8B4513"
  - name: chiptune
    full_name: Chiptune Composer
    role: サウンド担当
    model: haiku
    color: "#4169E1"
```

### 設定項目

| 項目 | 説明 |
|------|------|
| `name` | 識別名（小文字、英数字） |
| `full_name` | 表示名 |
| `role` | 役割の説明 |
| `model` | 使用するモデル（`sonnet` / `haiku` / `opus`） |
| `color` | 表示色（16進数） |

## ロールベースのワークフロー

レビューサイクル（`maxam review`）では、ロールに基づいてエージェントが自動選択されます。

| ロール | 用途 |
|--------|------|
| `developer` | 実装を担当 |
| `reviewer` | レビューを担当 |

ロールを設定する場合：

```yaml
agents:
  - name: yuki
    full_name: Yuki Tanaka
    role: developer  # または "Backend / developer" など
  - name: priya
    full_name: Priya Sharma
    role: reviewer
```

> **Note:** ロールが明示的に設定されていない場合、Yuki が developer、Priya が reviewer としてフォールバックします。

## プロジェクト固有 vs グローバル

| 場所 | 用途 |
|------|------|
| `~/.maxam/agents/` | 全プロジェクトで使う共通エージェント |
| `.maxam/agents/` | プロジェクト固有のエージェント（優先） |

プロジェクト固有のエージェントが存在する場合、グローバル設定よりも優先されます。

## マルチインスタンス

同じエージェントを複数インスタンス起動できます（高度な使い方）：

```yaml
agents:
  - name: yuki
    full_name: Yuki Tanaka
    role: Backend
    instance_id: "1"
  - name: yuki
    full_name: Yuki Tanaka
    role: Backend
    instance_id: "2"
```

インスタンスは `yuki-1`, `yuki-2` として識別されます。

## 関連ドキュメント

- [設定リファレンス](configuration.md) - 設定ファイルの詳細
- [ワークフロー](workflows.md) - エージェント間の連携
