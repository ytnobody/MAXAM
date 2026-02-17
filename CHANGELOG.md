# Changelog

All notable changes to this project will be documented in this file.

## [v0.13.0] - 2025-02-16

### ✨ Features
- **Health Monitoring**: エージェントのヘルスチェック機能を実装。定期監視と自動リカバリー対応
- **Auto-Recovery**: エージェント自動復旧メカニズムの追加。接続失敗時の自動リトライとタスク引き継ぎ
- **Task Handover**: エージェント間のタスク引き継ぎAPI実装。シームレスなタスク移行を実現
- **Reset Command**: `/reset` コマンドでエージェントの同期リセットが可能に
- **PR Reviewer Check**: CI実行時にPRレビュアー指定をチェック。ガバナンス強化

### 📝 Documentation
- マージフロー判断基準の境界ケース明確化（タグ, 依存パッケージ, スキーマ変更）
- エスカレーションタイムアウト基準を明文化（10分→5分→システム化）
- GitHub LGTMコメント必須ルール追加
- PR テンプレートとレビュアー指定ガイドラインを追加
- local check ガイドラインとチーム効果測定指標を実装
- ヘルスモニタリング・段階的実装・先読み実装の設計パターンを `.maxam/CLAUDE.md` に追加
- レビュー分散ルールの明文化（typo/テスト → 誰でも、機能追加 → Priya推奨）
- コンテキスト管理ガイドの確立（役割別の保持情報を最小化）

### 🔧 Infrastructure
- GitHub Actions の PR merge イベント監視を追加
- Handover API Client 実装
- pre-commit hook 準備（フェーズ2検討）

### 🐛 Bug Fixes
- TUI viewport フォーマッティング修正
- Context deadline exceeded 対応（タイムアウト基準化）

### 📈 Improvements
- 段階的実装による効率化（フェーズ分け）
- 先読み実装の判断基準を確立
- チーム効果測定ダッシュボード準備（フェーズ1完了）
- worktree 管理ガイドの確立（複数インスタンス対応）
- 並行実行ガイドラインの明文化（依存関係管理）

### 🎯 Team Improvements
- タスク分配の偏り防止ガイド追加
- 作業開始報告ルールの確立
- 発言即着手の原則化
- メンバーエラー発生時のフォロー手順明文化
- autoモード運用ルールの確立

## [v0.10.0] - Previous Release

See commit history for details.

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
