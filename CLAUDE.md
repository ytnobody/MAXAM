# MAXAM 共通規約

## プロジェクト概要

MAXAMは複数のAIエージェントが協調して開発業務を遂行するシステム。

## チームメンバー

| 名前 | 役割 |
|------|------|
| Mei Chen | 要件定義 + PM |
| Yuki Tanaka | バックエンド + インフラ |
| Rin Sato | フロントエンド |
| Shiori Tanaka | テスト + ドキュメント |
| Priya Sharma | レビュー + セキュリティ + QA |
| Amara Okonkwo | 分析 |

## コーディング規約

### 全般
- シンプルに保つ
- 過度な抽象化を避ける
- 必要なコメントのみ
- テストを書く

### Go
- `gofmt`でフォーマット
- エラーは適切にハンドリング
- パッケージ名は短く明確に

### コミットメッセージ
```
<type>: <subject>

<body>

Closes #XX
```

type: feat, fix, refactor, docs, test, chore

### PR
- タイトルは変更内容を簡潔に
- Issue番号を紐付け
- 動作確認方法を記載

### レビューフロー

1. PRが作成されたらCIのステータスを確認
2. ✅ CI通過（緑）→ レビュー開始
3. ❌ CI失敗（赤）→ レビュー保留、実装者へ差し戻し

**CIが通っていないPRはレビュー対象外**

### マージフロー

変更規模によってマージフローを使い分ける。

| 変更規模 | 例 | フロー |
|----------|-----|--------|
| 軽微 | typo、コメント、設定ファイルの微調整 | Priyaのレビュー通過後、即マージOK |
| 大きい | 機能追加、アーキテクチャ変更、複数ファイルにまたがる修正 | approve後にオーナーへ一報、確認もらってからマージ |

## 通信規約

### メッセージフォーマット
```markdown
## [YYYY-MM-DD HH:MM] From: {送信元} To: {宛先}

### Subject
{件名}

### Body
{本文}

### Action Required
{必要なアクション}

---
```

### チャンネル（非推奨）

> **Note**: ファイルベースの通信チャンネルは廃止されました。現在はチームチャットで直接コミュニケーションを行います。

### チームチャット
- 宛先は `@名前` で明示する
- 誰宛か分からないメッセージは避ける

## エスカレーションルール

1. 同一PR差し戻し3回超過 → Meiへ
2. `[SEC-CRITICAL]` → 即ブロック、Meiへ
3. 要件不明確 → Meiへ → 顧客確認

下流で無理に解決しない。問題は上流に伝播させる。

## Worktree運用

並列作業のため、各エージェントは専用のworktreeで作業する。

### ディレクトリ構成

```
/tmp/maxam/{agent-name}/{parent}_{child}/
```

親ディレクトリからの相対パスを `_` で連結してフラットに管理する。

| パス | 用途 |
|------|------|
| `/home/ubuntu/{project}/` | 元リポジトリ（main）、Meiの作業場所 |
| `/tmp/maxam/yuki/{project}/` | Yuki用worktree（単独リポジトリ） |
| `/tmp/maxam/yuki/{parent}_{child}/` | Yuki用worktree（ネストしたリポジトリ） |
| `/tmp/maxam/rin/{project}/` | Rin用worktree |
| `/tmp/maxam/shiori/{project}/` | Shiori用worktree |
| `/tmp/maxam/priya/{project}/` | Priya用worktree |
| `/tmp/maxam/amara/{project}/` | Amara用worktree |

**例：単独プロジェクト**
```
/tmp/maxam/yuki/MAXAM/      # ~/MAXAMで作業
/tmp/maxam/priya/MAXAM/     # PriyaのMAXAM作業
```

**例：ネストしたサブプロジェクト（親ディレクトリ配下に複数リポジトリ）**
```
~/calcium-lang/             # 親ディレクトリ（gitリポジトリではない）
├── calcium/               # リポジトリ1
├── boneyard/              # リポジトリ2
└── json/                  # リポジトリ3

↓ worktreeパス（_で連結）

/tmp/maxam/yuki/calcium-lang_calcium/    # calcium-lang/calcium
/tmp/maxam/yuki/calcium-lang_boneyard/   # calcium-lang/boneyard
/tmp/maxam/yuki/calcium-lang_json/       # calcium-lang/json
```

**例：深いネストの場合**
```
~/some/deep/project/ → /tmp/maxam/yuki/project/
~/parent/child/grandchild/ → /tmp/maxam/yuki/parent_child_grandchild/
```

### 運用ルール

1. **作業開始時**: worktreeが存在しなければ作成
   ```bash
   # 単独リポジトリ
   git worktree add /tmp/maxam/{name}/{project} {branch}

   # ネストしたリポジトリ（親ディレクトリ名を含める）
   git worktree add /tmp/maxam/{name}/{parent}_{child} {branch}
   ```
2. **ブランチ管理**: worktree内で自由に切り替えOK
3. **マシン再起動時**: `/tmp`は消えるので再作成
4. **サブプロジェクト検出**: MAXAMは起動時に配下のgitリポジトリを自動検出

### 備考

- Meiは元リポジトリで要件定義・ドキュメント作業
- 他メンバーは自分のworktreeで独立して作業可能
- 衝突を気にせず並列作業できる
- `/tmp/maxam/` 配下はMAXAMチーム共通の作業場所として、複数プロジェクトを管理
- **サブプロジェクト対応**: 親ディレクトリ配下のgitリポジトリを再帰的に検出し、コンテキストに含める

## 学習事項

### 2026-01-27 分析結果 (by Amara)

#### 環境整備
- レビュー担当（Priya）の環境にも開発言語のランタイムをインストールし、独立した動作確認を可能にする
- 実装者の報告のみに依存せず、独立検証できる体制を整える

**Goインストール手順（Ubuntu 24.04）:**
```bash
# 方法1: aptでインストール（シンプル）
sudo apt update && sudo apt install -y golang-go
go version

# 方法2: 最新版が必要な場合
wget https://go.dev/dl/go1.23.5.linux-amd64.tar.gz
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.23.5.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
go version

# 動作確認
cd /tmp/maxam/priya/MAXAM
go test ./...
```

#### 報告フォーマット
実装完了報告には以下を含める:
1. 実装コードの要点
2. テストケース数とカバー範囲
3. テスト実行結果

#### 発見されたパターン
- 同一タスクの重複実行に注意（トリガーの確認）
- 報告の詳細度が上がるとレビュー品質も向上する傾向

#### GitHubアカウント制約
- オーナーアカウントで作成したPRは、同一アカウントではApproveできない
- 軽微な変更（ドキュメントのみ等）はレビュアーのLGTM確認後、オーナー判断でマージ可

#### タスク実行の重複防止
- 同一タスクを複数エージェントが拾わないよう、作業開始時に宣言する
- 「○○やります」と明示してから着手

---

*このファイルはチームの学習により継続的に更新される*
