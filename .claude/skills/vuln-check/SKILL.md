---
name: vuln-check
description: This skill should be used when the user asks to "check vulnerabilities", "check for security issues in dependencies", "run vuln-check", "check CVEs", or discusses dependency security scanning. Provides Go module vulnerability scanning using govulncheck.
version: 1.0.0
---

# Vulnerability Check Skill

依存パッケージの脆弱性をチェックし、レポートを生成します。

## 使用タイミング

- `/vuln-check` コマンドを実行した時
- 依存パッケージのアップグレード作業前
- CIが謎に落ちた時の原因切り分け
- セキュリティアップデートの必要性を確認したい時

## 実行手順

### 1. govulncheckのインストール確認

```bash
which govulncheck || go install golang.org/x/vuln/cmd/govulncheck@latest
```

### 2. 脆弱性スキャン実行

```bash
govulncheck ./...
```

### 3. 結果の解析

govulncheckの出力を解析し、以下の形式でレポートを作成：

## レポートフォーマット

### サマリ

| 項目 | 値 |
|------|-----|
| スキャン対象 | (モジュール名) |
| スキャン日時 | (YYYY-MM-DD HH:MM) |
| 検出数 | N件 |

### 脆弱性一覧

| ID | パッケージ | 影響バージョン | 修正バージョン | 重要度 |
|----|-----------|--------------|--------------|-------|
| GO-XXXX-XXXX | example.com/pkg | < 1.2.3 | 1.2.3 | High/Medium/Low |

### 影響を受けるコード箇所

脆弱性ごとに、実際に呼び出しているコード箇所を特定：

```
Vulnerability: GO-XXXX-XXXX
  Package: example.com/pkg
  Symbol: VulnerableFunc
  Called from:
    - internal/foo/bar.go:123
    - cmd/main.go:45
```

### 推奨対応

| 優先度 | 対応内容 |
|--------|---------|
| 1 | (パッケージ)を(バージョン)にアップグレード |
| 2 | ... |

## 出力がない場合

脆弱性が検出されなかった場合は以下を報告：

```
## 脆弱性チェック結果

脆弱性は検出されませんでした。

- スキャン対象: (モジュール名)
- スキャン日時: (YYYY-MM-DD HH:MM)
- govulncheckバージョン: (バージョン)
```

## 注意事項

- govulncheckはGoモジュールのみ対応
- 他言語の依存関係（npm、pip等）は別途ツールが必要
- CI環境では `govulncheck -json ./...` でJSON出力も可能
