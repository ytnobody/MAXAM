---
name: version-sync
description: This skill should be used when the user asks to "check go.mod/go.sum", "sync versions", "check dependency consistency", "version-sync", or needs to verify Go module integrity. Provides go.mod and go.sum consistency verification and repair.
version: 1.0.0
---

# Version Sync Skill

`go.mod` と `go.sum` の整合性をチェック・修正し、依存関係のバージョン不整合を解消します。

## 使用タイミング

- `/version-sync` コマンドを実行した時
- Go依存関係のアップグレード作業前
- CIが `go.mod` / `go.sum` 不整合エラーで落ちた時
- パッケージ追加後の整合性確認
- メンテナンス作業時の定期確認

## 実行手順

### 1. 現在の整合性確認

```bash
go mod verify
```

出力例：
- 正常: 何も出力されない（終了コード 0）
- エラー: `hash mismatch` 等のエラーメッセージ

### 2. 整合性チェックと修正

```bash
# 整合性チェック（読み取り専用）
go mod tidy -check

# 整合性修正（go.mod / go.sum を更新）
go mod tidy
```

### 3. テスト実行で整合性確認

```bash
go test ./... -v
```

## レポートフォーマット

### サマリ

| 項目 | 値 |
|------|-----|
| チェック対象 | go.mod / go.sum |
| チェック日時 | (YYYY-MM-DD HH:MM) |
| 整合性 | 正常 / 修正済み / エラー |

### チェック結果

#### パターン1: 整合性が正常

```
## go.mod / go.sum 整合性チェック結果

✅ 整合性は正常です。

- go mod verify: OK
- go mod tidy -check: OK
- テスト実行: PASS

**対応不要**
```

#### パターン2: 整合性を修正

```
## go.mod / go.sum 整合性チェック結果

⚠️ 不整合を検出し、修正しました。

### 修正前の状態

- go mod verify: ❌ hash mismatch エラー
- 不整合パッケージ: github.com/example/package (v1.2.3)

### 実行したコマンド

```bash
go mod tidy
```

### 修正後の確認

- go mod verify: ✅ OK
- go mod tidy -check: ✅ OK
- テスト実行: ✅ PASS

**修正完了。以下を実行してください:**
```bash
git add go.mod go.sum
git commit -m "chore: sync Go dependencies"
```

---
```

#### パターン3: エラー発生（手動対応が必要）

```
## go.mod / go.sum 整合性チェック結果

❌ エラーが発生しました。手動対応が必要です。

### エラー内容

```
hash mismatch in go.sum:
github.com/example/package v1.2.3 h1:XXXXXXXXXXX
wants: h1:YYYYYYY
```

### 推奨対応

1. 不整合パッケージをリストアップ
2. 明示的にリクエスト: `go get github.com/example/package@latest`
3. 再度 `go mod tidy` を実行
4. テストが通ることを確認
```

## 詳細な検査フロー

### 1. go.mod のバージョン確認

```bash
grep "^require (" -A 50 go.mod | grep "github.com/example"
```

### 2. go.sum の整合性検証

```bash
go mod verify
```

### 3. 不要な依存の検出

```bash
go mod graph | grep "example"
```

## よくある不整合パターン

| パターン | 原因 | 対応 |
|---------|------|------|
| `hash mismatch` | go.sum が古い / ファイル破損 | `go mod tidy` |
| `unknown module` | go.mod に記載があるがダウンロード不可 | `go clean -modcache` → `go mod tidy` |
| 不要な依存残存 | パッケージ削除後に依存が残った | `go mod tidy` |
| バージョン競合 | 複数パッケージが異なるバージョンを要求 | `go mod graph` で確認 → 手動調整 |

## トラブルシューティング

### ネットワークエラーが出た場合

```bash
# GoのModuleキャッシュをクリア
go clean -modcache

# 再度実行
go mod tidy
```

### 特定パッケージの問題が解決しない場合

```bash
# 該当パッケージを明示的に更新
go get -u github.com/example/package@latest

# 整合性確認
go mod verify
```

## 注意事項

- `go mod tidy` は不要な依存を削除するため、必要なパッケージが削除される場合がある
  - 削除前に `git diff` で確認すること
- CI環境では実行前にキャッシュをクリアすることを推奨
- Goのバージョンによって動作が異なる場合がある（1.16以降推奨）
