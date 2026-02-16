# Changelog

All notable changes to this project will be documented in this file.

## [v0.10.0] - 2025-02-16

### ✨ Features
- **Health Monitoring**: エージェントのヘルスチェック機能を実装。定期監視とリバースエンジニアリング対応
- **Auto-Recovery**: エージェント自動復旧メカニズムの追加。接続失敗時の自動リトライとタスク引き継ぎ
- **Task Handover**: エージェント間のタスク引き継ぎAPI実装。シームレスなタスク移行を実現
- **Reset Command**: `/reset` コマンドでエージェントの同期リセットが可能に
- **PR Reviewer Check**: CI実行時にPRレビュアー指定をチェック。ガバナンス強化

### 📝 Documentation
- マージフロー判断基準の境界ケース明確化（タグ, 依存パッケージ, スキーマ変更）
- エスカレーションタイムアウト基準を明文化（10分→5分）
- GitHub LGTMコメント必須ルール追加
- PR テンプレートとレビュアー指定ガイドラインを追加
- local check ガイドラインとチーム効果測定を実装
- ヘルスモニタリング設計パターンを `.maxam/CLAUDE.md` に追加

### 🔧 Infrastructure
- GitHub Actions の PR merge イベント監視を追加
- Handover API Client 実装

### 🐛 Bug Fixes
- TUI viewport フォーマッティング修正

### 📈 Improvements
- 段階的実装による効率化
- 先読み実装の判断基準を確立
- チーム効果測定ダッシュボード準備（フェーズ1完了）

## [v0.9.0] - Previous Release

See commit history for details.

---

*For more details, see [GitHub Releases](https://github.com/ytnobody/MAXAM/releases)*
