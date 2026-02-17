# Issue #253 自動復旧機能 テストケース骨子

## 概要

このドキュメントはYukiの設計に基づくテストケースの骨子です。
具体的なパラメータ（ヘルスチェック間隔、リトライ回数など）は設計ドキュメント確定後に詳細化します。

## 設計パラメータ（予定）

| パラメータ | 値 | 備考 |
|-----------|-----|------|
| ヘルスチェック間隔 | 5秒 | 設計ドキュメントより |
| リトライ上限 | 3回 | 同上 |
| バックオフ | 指数バックオフ | 同上 |

---

## 1. taskqueue パッケージ（タスクキュー）

### 1.1 正常系

| ケース名 | 入力 | 期待値 |
|----------|------|--------|
| `Enqueue_SingleTask` | 1タスク追加 | キューにタスク存在、長さ=1 |
| `Enqueue_MultipleTasks` | 複数タスク追加 | FIFO順序で取得可能 |
| `Dequeue_Success` | キューにタスクあり | 先頭タスク取得、長さ-1 |
| `Peek_NotRemove` | Peek呼び出し | タスク参照のみ、長さ変化なし |
| `AssignTask_ToAgent` | タスク割り当て | 担当エージェント設定済み |

### 1.2 異常系

| ケース名 | 入力 | 期待値 |
|----------|------|--------|
| `Dequeue_EmptyQueue` | 空キュー | nil 返却 or エラー |
| `Enqueue_NilTask` | nil タスク | エラー返却 |
| `Enqueue_QueueFull` | キュー上限到達時 | エラー or 古いタスク破棄 |

### 1.3 エッジケース

| ケース名 | 入力 | 期待値 |
|----------|------|--------|
| `ConcurrentEnqueue` | 複数goroutineから同時追加 | データ競合なし、全タスク追加 |
| `ConcurrentDequeue` | 複数goroutineから同時取得 | 各タスクは1回のみ取得 |
| `EnqueueDuringDequeue` | Enqueue/Dequeue並行実行 | デッドロックなし |

---

## 2. coordinator パッケージ（タスク引き継ぎ統合）

### 2.1 正常系：引き継ぎフロー

| ケース名 | 入力 | 期待値 |
|----------|------|--------|
| `Handoff_ToSameRole` | 同一ロール別インスタンス存在 | タスク再割り当て成功 |
| `Handoff_ToAlternativeAgent` | 代替エージェント存在 | 代替に割り当て成功 |
| `Handoff_WithTaskQueue` | キューにタスクあり | キューから取得→割り当て |
| `Handoff_NotifyPM` | 引き継ぎ完了 | PM通知に引き継ぎ情報含む |

### 2.2 異常系：引き継ぎ失敗

| ケース名 | 入力 | 期待値 |
|----------|------|--------|
| `Handoff_NoAvailableAgent` | 全エージェント応答不能 | エラー返却、PM通知 |
| `Handoff_AllInstancesFailed` | 同一ロール全インスタンス障害 | フォールバック or エラー |
| `Handoff_TaskQueueEmpty` | 引き継ぐタスクなし | 何もせず正常終了 |

### 2.3 エッジケース

| ケース名 | 入力 | 期待値 |
|----------|------|--------|
| `Handoff_DuringRecovery` | 復旧中に別エージェント障害 | 両方の引き継ぎ処理 |
| `Handoff_SameTaskTwice` | 同一タスク重複引き継ぎ | 2回目は無視 or 冪等 |
| `Handoff_LargeQueue` | 大量タスク（100件） | タイムアウトなし、全件処理 |

---

## 3. 統合テスト（heartbeat + recovery + coordinator）

### 3.1 E2Eシナリオ

| ケース名 | シナリオ | 期待値 |
|----------|----------|--------|
| `E2E_DetectAndRecover` | エージェント無応答→検知→リトライ→復旧 | 状態がhealthyに戻る |
| `E2E_DetectAndHandoff` | 無応答→リトライ上限→タスク引き継ぎ | 別エージェントがタスク継続 |
| `E2E_PMNotification` | 復旧失敗→PM通知 | 通知メッセージに詳細含む |
| `E2E_MultipleAgentFailure` | 複数エージェント同時障害 | 各々独立して復旧処理 |

### 3.2 状態遷移テスト

```
正常 → 応答待ち → タイムアウト検知 → リトライ → 復旧 or 完全失敗
```

| ケース名 | 開始状態 | トリガー | 終了状態 |
|----------|----------|----------|----------|
| `Transition_HealthyToUnresponsive` | healthy | ping timeout | unresponsive |
| `Transition_UnresponsiveToHealthy` | unresponsive | ping success | healthy |
| `Transition_UnresponsiveToDead` | unresponsive | max retries | dead |
| `Transition_DeadToHandoff` | dead | 引き継ぎ発動 | タスク移譲完了 |

---

## 4. パフォーマンス・負荷テスト（nice to have）

| ケース名 | 条件 | 期待値 |
|----------|------|--------|
| `Load_100Agents` | 100エージェント監視 | メモリ・CPU正常範囲 |
| `Load_RapidFailure` | 1秒間に10回障害発生 | 全件検知、ログ出力正常 |
| `Load_QueueBackpressure` | キュー満杯時の挙動 | 適切なバックプレッシャー |

---

## テストコード構造（table-driven）

```go
func TestTaskQueue_Enqueue(t *testing.T) {
    tests := []struct {
        name    string
        input   /* TBD: 設計確定後 */
        want    /* TBD */
        wantErr bool
    }{
        {
            name:    "SingleTask",
            input:   /* TBD */,
            want:    /* TBD */,
            wantErr: false,
        },
        // ... more cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // TBD: 実装
        })
    }
}
```

---

## 次のステップ

1. [ ] Yukiの設計ドキュメント確定を待つ
2. [ ] パラメータ（間隔、リトライ数など）を反映
3. [ ] テストコード雛形を作成
4. [ ] 実装と並行してテスト実装

---

*作成: Shiori Tanaka*
*最終更新: 2025-02-16*
