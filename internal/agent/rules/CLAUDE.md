# MAXAM 共通規約

## プロジェクト概要

MAXAMは複数のAIエージェントが協調して開発業務を遂行するシステム。

## チームメンバー

| 名前 | 役割 |
|------|------|
| Mei Chen | 要件定義 + PM |
| Yuki Tanaka | 実装 + インフラ |
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

## エスカレーションルール

1. 同一PR差し戻し3回超過 → Meiへ
2. `[SEC-CRITICAL]` → 即ブロック、Meiへ
3. 要件不明確 → Meiへ → 顧客確認

下流で無理に解決しない。問題は上流に伝播させる。

## Worktree運用

並列作業のため、各エージェントは専用のworktreeで作業する。

### ディレクトリ構成

```
/tmp/maxam/{agent-name}/{project-name}/
```

| パス | 用途 |
|------|------|
| `~/{project}/` | 元リポジトリ（main）、Meiの作業場所 |
| `/tmp/maxam/yuki/{project}/` | Yuki用worktree |
| `/tmp/maxam/priya/{project}/` | Priya用worktree |
| `/tmp/maxam/amara/{project}/` | Amara用worktree |

**例：複数プロジェクトを担当する場合**
```
/tmp/maxam/yuki/MAXAM/      # YukiのMAXAM作業
/tmp/maxam/yuki/HOGEHOGE/   # YukiのHOGEHOGE作業
/tmp/maxam/priya/MAXAM/     # PriyaのMAXAM作業
/tmp/maxam/priya/HOGEHOGE/  # PriyaのHOGEHOGE作業
```

### 運用ルール

1. **作業開始時**: worktreeが存在しなければ作成
   ```bash
   git worktree add /tmp/maxam/{name}/{project} {branch}
   ```
2. **ブランチ管理**: worktree内で自由に切り替えOK
3. **マシン再起動時**: `/tmp`は消えるので再作成

### 備考

- Meiは元リポジトリで要件定義・ドキュメント作業
- 他メンバーは自分のworktreeで独立して作業可能
- 衝突を気にせず並列作業できる
- `/tmp/maxam/` 配下はMAXAMチーム共通の作業場所として、複数プロジェクトを管理
