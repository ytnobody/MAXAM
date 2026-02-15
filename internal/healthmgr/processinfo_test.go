package healthmgr

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"
)

// =============================================================================
// ProcessInfo 構造体テスト
// =============================================================================

func TestProcessInfo_StructDefinition(t *testing.T) {
	// ProcessInfo 構造体が正しく定義されているか確認
	info := ProcessInfo{
		PID:       1234,
		CPUUsage:  25.5,
		MemUsage:  1024 * 1024 * 100, // 100MB
		Children:  []int{5678, 9012},
		Timestamp: time.Now(),
	}

	if info.PID != 1234 {
		t.Errorf("PID = %d, want 1234", info.PID)
	}
	if info.CPUUsage != 25.5 {
		t.Errorf("CPUUsage = %f, want 25.5", info.CPUUsage)
	}
	if info.MemUsage != 1024*1024*100 {
		t.Errorf("MemUsage = %d, want %d", info.MemUsage, 1024*1024*100)
	}
	if len(info.Children) != 2 {
		t.Errorf("len(Children) = %d, want 2", len(info.Children))
	}
}

// =============================================================================
// 正常系テスト - プロセス情報の取得
// =============================================================================

func TestProcessInfo_GetProcessInfo_Success(t *testing.T) {
	// 現在のプロセスの情報を取得
	pid := os.Getpid()
	ctx := context.Background()

	info, err := GetProcessInfo(ctx, pid)
	if err != nil {
		t.Fatalf("GetProcessInfo failed: %v", err)
	}

	if info.PID != pid {
		t.Errorf("PID = %d, want %d", info.PID, pid)
	}
	// CPU使用率は0以上
	if info.CPUUsage < 0 {
		t.Errorf("CPUUsage = %f, want >= 0", info.CPUUsage)
	}
	// メモリ使用量は正の値
	if info.MemUsage == 0 {
		t.Errorf("MemUsage = 0, want > 0")
	}
	// Timestamp が設定されている
	if info.Timestamp.IsZero() {
		t.Error("Timestamp is zero")
	}
}

// =============================================================================
// 履歴管理テスト（リングバッファ）
// =============================================================================

func TestProcessInfo_History_Accumulation(t *testing.T) {
	pm := NewProcessMonitor(5)

	// 3件追加
	for i := 0; i < 3; i++ {
		pm.UpdateProcessInfo("agent1", ProcessInfo{
			PID:       1000 + i,
			CPUUsage:  float64(i * 10),
			Timestamp: time.Now(),
		})
	}

	history := pm.GetHistory("agent1")
	if len(history) != 3 {
		t.Errorf("len(history) = %d, want 3", len(history))
	}
}

func TestProcessInfo_History_Overflow(t *testing.T) {
	pm := NewProcessMonitor(5)

	// 7件追加（5件を超える）
	for i := 0; i < 7; i++ {
		pm.UpdateProcessInfo("agent1", ProcessInfo{
			PID:       1000 + i,
			CPUUsage:  float64(i * 10),
			Timestamp: time.Now(),
		})
	}

	history := pm.GetHistory("agent1")
	if len(history) != 5 {
		t.Errorf("len(history) = %d, want 5", len(history))
	}

	// 最古のエントリが押し出されている（PID 1000, 1001 はない）
	// 最古は PID 1002 のはず
	if history[0].PID != 1002 {
		t.Errorf("oldest PID = %d, want 1002", history[0].PID)
	}
	// 最新は PID 1006
	if history[4].PID != 1006 {
		t.Errorf("newest PID = %d, want 1006", history[4].PID)
	}
}

func TestProcessInfo_MultipleAgents(t *testing.T) {
	pm := NewProcessMonitor(5)

	pm.UpdateProcessInfo("agent1", ProcessInfo{PID: 1000})
	pm.UpdateProcessInfo("agent2", ProcessInfo{PID: 2000})
	pm.UpdateProcessInfo("agent1", ProcessInfo{PID: 1001})

	h1 := pm.GetHistory("agent1")
	h2 := pm.GetHistory("agent2")

	if len(h1) != 2 {
		t.Errorf("agent1 history len = %d, want 2", len(h1))
	}
	if len(h2) != 1 {
		t.Errorf("agent2 history len = %d, want 1", len(h2))
	}
}

// =============================================================================
// 異常系・エッジケーステスト
// =============================================================================

func TestProcessInfo_Error_InvalidPID(t *testing.T) {
	ctx := context.Background()

	// 存在しないPID
	_, err := GetProcessInfo(ctx, 999999999)
	if err == nil {
		t.Error("expected error for invalid PID, got nil")
	}
}

func TestProcessInfo_GetLatest(t *testing.T) {
	pm := NewProcessMonitor(5)

	// 空の状態
	latest := pm.GetLatest("agent1")
	if latest != nil {
		t.Error("expected nil for empty history")
	}

	// データ追加後
	pm.UpdateProcessInfo("agent1", ProcessInfo{PID: 1000})
	pm.UpdateProcessInfo("agent1", ProcessInfo{PID: 1001})

	latest = pm.GetLatest("agent1")
	if latest == nil {
		t.Fatal("expected non-nil")
	}
	if latest.PID != 1001 {
		t.Errorf("latest PID = %d, want 1001", latest.PID)
	}
}

// =============================================================================
// 境界値テスト
// =============================================================================

func TestProcessInfo_Boundary_EmptyHistory(t *testing.T) {
	pm := NewProcessMonitor(5)

	history := pm.GetHistory("nonexistent")
	if history == nil {
		t.Error("expected empty slice, got nil")
	}
	if len(history) != 0 {
		t.Errorf("len(history) = %d, want 0", len(history))
	}
}

func TestProcessInfo_Boundary_ZeroCPU(t *testing.T) {
	pm := NewProcessMonitor(5)

	pm.UpdateProcessInfo("agent1", ProcessInfo{
		PID:      1000,
		CPUUsage: 0.0,
	})

	latest := pm.GetLatest("agent1")
	if latest == nil {
		t.Fatal("expected non-nil")
	}
	if latest.CPUUsage != 0.0 {
		t.Errorf("CPUUsage = %f, want 0.0", latest.CPUUsage)
	}
}

func TestProcessInfo_Boundary_NoChildren(t *testing.T) {
	pm := NewProcessMonitor(5)

	pm.UpdateProcessInfo("agent1", ProcessInfo{
		PID:      1000,
		Children: []int{},
	})

	latest := pm.GetLatest("agent1")
	if latest == nil {
		t.Fatal("expected non-nil")
	}
	if len(latest.Children) != 0 {
		t.Errorf("len(Children) = %d, want 0", len(latest.Children))
	}
}

func TestProcessInfo_DefaultHistorySize(t *testing.T) {
	// historySize <= 0 の場合、デフォルト値5が使われる
	pm := NewProcessMonitor(0)

	// 6件追加
	for i := 0; i < 6; i++ {
		pm.UpdateProcessInfo("agent1", ProcessInfo{PID: 1000 + i})
	}

	history := pm.GetHistory("agent1")
	if len(history) != 5 {
		t.Errorf("len(history) = %d, want 5 (default)", len(history))
	}
}

// =============================================================================
// 並行性テスト
// =============================================================================

func TestProcessInfo_Concurrent_MultipleReaders(t *testing.T) {
	pm := NewProcessMonitor(5)

	// 初期データ
	for i := 0; i < 5; i++ {
		pm.UpdateProcessInfo("agent1", ProcessInfo{PID: 1000 + i})
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = pm.GetHistory("agent1")
				_ = pm.GetLatest("agent1")
			}
		}()
	}
	wg.Wait()
}

func TestProcessInfo_Concurrent_ReadWhileWrite(t *testing.T) {
	pm := NewProcessMonitor(5)

	done := make(chan struct{})

	// Writer
	go func() {
		for i := 0; i < 100; i++ {
			pm.UpdateProcessInfo("agent1", ProcessInfo{PID: 1000 + i})
		}
		close(done)
	}()

	// Reader
	for {
		select {
		case <-done:
			return
		default:
			_ = pm.GetHistory("agent1")
			_ = pm.GetLatest("agent1")
		}
	}
}

// =============================================================================
// Clear テスト
// =============================================================================

func TestProcessInfo_Clear(t *testing.T) {
	pm := NewProcessMonitor(5)

	pm.UpdateProcessInfo("agent1", ProcessInfo{PID: 1000})
	pm.UpdateProcessInfo("agent2", ProcessInfo{PID: 2000})

	pm.Clear("agent1")

	h1 := pm.GetHistory("agent1")
	h2 := pm.GetHistory("agent2")

	if len(h1) != 0 {
		t.Errorf("agent1 should be cleared, got len=%d", len(h1))
	}
	if len(h2) != 1 {
		t.Errorf("agent2 should not be affected, got len=%d", len(h2))
	}
}

func TestProcessInfo_ClearAll(t *testing.T) {
	pm := NewProcessMonitor(5)

	pm.UpdateProcessInfo("agent1", ProcessInfo{PID: 1000})
	pm.UpdateProcessInfo("agent2", ProcessInfo{PID: 2000})

	pm.ClearAll()

	h1 := pm.GetHistory("agent1")
	h2 := pm.GetHistory("agent2")

	if len(h1) != 0 {
		t.Errorf("agent1 should be cleared, got len=%d", len(h1))
	}
	if len(h2) != 0 {
		t.Errorf("agent2 should be cleared, got len=%d", len(h2))
	}
}

// =============================================================================
// リングバッファ内部テスト
// =============================================================================

func TestRingBuffer_Basic(t *testing.T) {
	rb := newRingBuffer(3)

	// 空の状態
	if rb.getLatest() != nil {
		t.Error("expected nil for empty buffer")
	}
	if len(rb.getAll()) != 0 {
		t.Error("expected empty slice for empty buffer")
	}

	// 1件追加
	rb.push(ProcessInfo{PID: 1})
	if rb.count != 1 {
		t.Errorf("count = %d, want 1", rb.count)
	}

	latest := rb.getLatest()
	if latest == nil || latest.PID != 1 {
		t.Errorf("latest = %v, want PID=1", latest)
	}

	// 3件追加（バッファフル）
	rb.push(ProcessInfo{PID: 2})
	rb.push(ProcessInfo{PID: 3})

	all := rb.getAll()
	if len(all) != 3 {
		t.Errorf("len(all) = %d, want 3", len(all))
	}

	// 4件目（オーバーフロー）
	rb.push(ProcessInfo{PID: 4})

	all = rb.getAll()
	if len(all) != 3 {
		t.Errorf("len(all) = %d, want 3", len(all))
	}
	// 最古は PID=2
	if all[0].PID != 2 {
		t.Errorf("oldest PID = %d, want 2", all[0].PID)
	}
	// 最新は PID=4
	if all[2].PID != 4 {
		t.Errorf("newest PID = %d, want 4", all[2].PID)
	}
}

// =============================================================================
// ベンチマーク
// =============================================================================

func BenchmarkProcessInfo_GetInfo(b *testing.B) {
	ctx := context.Background()
	pid := os.Getpid()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = GetProcessInfo(ctx, pid)
	}
}

func BenchmarkProcessInfo_HistoryWrite(b *testing.B) {
	pm := NewProcessMonitor(5)
	info := ProcessInfo{PID: 1000, CPUUsage: 25.5}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pm.UpdateProcessInfo("agent1", info)
	}
}

func BenchmarkProcessInfo_HistoryRead(b *testing.B) {
	pm := NewProcessMonitor(5)
	for i := 0; i < 5; i++ {
		pm.UpdateProcessInfo("agent1", ProcessInfo{PID: 1000 + i})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pm.GetHistory("agent1")
	}
}
