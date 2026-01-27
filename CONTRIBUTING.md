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

## 質問がある場合

Issue を作成するか、Discussion でお気軽にどうぞ！
