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

### チャンネル
- `mei_to_yuki.md`: Issue割り当て
- `yuki_to_priya.md`: PR通知
- `priya_to_yuki.md`: レビュー結果
- `priya_to_mei.md`: エスカレーション

## エスカレーションルール

1. 同一PR差し戻し3回超過 → Meiへ
2. `[SEC-CRITICAL]` → 即ブロック、Meiへ
3. 要件不明確 → Meiへ → 顧客確認

下流で無理に解決しない。問題は上流に伝播させる。

## 学習事項

（Amaraの分析結果がここに追記される）

---

*このファイルはチームの学習により継続的に更新される*
