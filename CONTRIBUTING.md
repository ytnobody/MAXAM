# Contributing to MAXAM

MAXAM へのコントリビューションをありがとうございます！

## 開発環境のセットアップ

### 必要なもの

- Go 1.23 以上
- Git
- Claude Code CLI (`claude`)

### ビルド

```bash
git clone https://github.com/ytnobody/MAXAM.git
cd MAXAM
go build -o bin/maxam ./cmd/maxam
```

### テスト実行

```bash
go test ./...
```

## コントリビューションの流れ

1. **Issue を確認する**
   - 取り組みたい Issue があれば、コメントで意思表示してください
   - 新機能や大きな変更は、先に Issue を作成して議論しましょう

2. **ブランチを作成する**
   ```bash
   git checkout -b feature/your-feature-name
   ```

3. **変更を加える**
   - コーディング規約に従ってください（下記参照）
   - テストを書いてください

4. **コミットする**
   ```bash
   git commit -m "feat: 機能の説明"
   ```

5. **Pull Request を作成する**
   - 変更内容を説明してください
   - 関連する Issue があれば紐付けてください

## コーディング規約

### Go

- `gofmt` でフォーマットしてください
- エラーは適切にハンドリングしてください
- パッケージ名は短く明確に

### コミットメッセージ

```
<type>: <subject>

<body>

Closes #XX
```

**type の種類:**
- `feat`: 新機能
- `fix`: バグ修正
- `refactor`: リファクタリング
- `docs`: ドキュメント
- `test`: テスト
- `chore`: その他

### Pull Request

- タイトルは変更内容を簡潔に
- Issue 番号を紐付け
- 動作確認方法を記載

## 行動規範

- 建設的なフィードバックを心がけてください
- 多様な意見を尊重してください
- 初心者にも優しく接してください

## スーパーバイザーへの相談

開発中に困ったことや判断に迷ったときは、オーナー（スーパーバイザー）に相談してください。

### 相談のトリガー

- **判断に迷ったとき**: 「AとBどちらがいいか分からない」
- **設計の分岐点**: アーキテクチャに影響する決定
- **要件の解釈**: 複数の解釈ができるとき
- **エスカレーション対象**: セキュリティ問題、優先度衝突、顧客影響のある判断

### 相談のコツ

- 完璧な質問でなくてOK
- 「〜でいいですか？」レベルで気軽に確認
- 大きな作業に入る前は方針を共有してOKをもらう
- 下流で無理に解決せず、問題は上流に伝播させる

### 例

```
❌ 悩んだまま実装を進める
✅ 「JWT と セッション、どちらで実装すべきですか？」と確認
```

## チームメンバーの追加

新しいAIエージェントをチームに追加する際のチェックリスト。

### 変更が必要なファイル

1. **`internal/config/config.go`** - `DefaultConfig()` に新メンバーを追加
   ```go
   {Name: "name", FullName: "Full Name", Role: "役割"},
   ```

2. **`agents/{name}/CLAUDE.md`** - エージェントのペルソナファイルを作成
   - 既存エージェントのファイルを参考に
   - 性格、役割、行動規範を定義

3. **`internal/agent/rules/CLAUDE.md`** - 共通規約のチームメンバー表を更新

4. **`CLAUDE.md`** - ルートの共通規約のチームメンバー表を更新

5. **`README.md`** - チームメンバー表を更新

### 手順

```bash
# 1. ブランチ作成
git checkout -b feature/add-agent-{name}

# 2. config.go を編集（DefaultConfig に追加）
# 3. agents/{name}/CLAUDE.md を作成
# 4. 各ドキュメントのチームメンバー表を更新
# 5. ビルド・テスト
go build ./... && go test ./...

# 6. コミット・PR作成
```

### 備考

- `FullName` は必須。省略すると `Name` がそのまま使われる
- ペルソナファイルは `agents/` 配下に配置
- ビルド時に `agents/` は `~/.maxam/agents/` へ自動展開される

## 質問がある場合

Issue を作成するか、Discussion でお気軽にどうぞ！
