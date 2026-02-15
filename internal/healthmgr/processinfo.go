package healthmgr

import (
	"context"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/process"
)

// ProcessInfo represents the resource usage of an agent process.
type ProcessInfo struct {
	PID       int
	CPUUsage  float64 // percentage (0-100)
	MemUsage  uint64  // bytes
	Children  []int   // child process PIDs
	Timestamp time.Time
}

// ProcessMonitor tracks process information with history.
type ProcessMonitor struct {
	mu          sync.RWMutex
	historySize int
	history     map[string]*ringBuffer
}

// ringBuffer implements a fixed-size circular buffer for ProcessInfo.
type ringBuffer struct {
	data  []ProcessInfo
	size  int
	head  int
	count int
}

func newRingBuffer(size int) *ringBuffer {
	return &ringBuffer{
		data: make([]ProcessInfo, size),
		size: size,
	}
}

func (r *ringBuffer) push(info ProcessInfo) {
	r.data[r.head] = info
	r.head = (r.head + 1) % r.size
	if r.count < r.size {
		r.count++
	}
}

func (r *ringBuffer) getAll() []ProcessInfo {
	if r.count == 0 {
		return []ProcessInfo{}
	}

	result := make([]ProcessInfo, r.count)
	start := 0
	if r.count == r.size {
		start = r.head
	}

	for i := 0; i < r.count; i++ {
		idx := (start + i) % r.size
		result[i] = r.data[idx]
	}
	return result
}

func (r *ringBuffer) getLatest() *ProcessInfo {
	if r.count == 0 {
		return nil
	}
	idx := (r.head - 1 + r.size) % r.size
	info := r.data[idx]
	return &info
}

// NewProcessMonitor creates a new ProcessMonitor with the specified history size.
func NewProcessMonitor(historySize int) *ProcessMonitor {
	if historySize <= 0 {
		historySize = 5 // default: 5 samples
	}
	return &ProcessMonitor{
		historySize: historySize,
		history:     make(map[string]*ringBuffer),
	}
}

// GetProcessInfo retrieves process information for a given PID.
// Returns nil and error if the process doesn't exist or can't be accessed.
func GetProcessInfo(ctx context.Context, pid int) (*ProcessInfo, error) {
	proc, err := process.NewProcessWithContext(ctx, int32(pid))
	if err != nil {
		return nil, err
	}

	info := &ProcessInfo{
		PID:       pid,
		Timestamp: time.Now(),
	}

	// Get CPU usage (percentage)
	cpuPercent, err := proc.CPUPercentWithContext(ctx)
	if err == nil {
		info.CPUUsage = cpuPercent
	}

	// Get memory usage (RSS)
	memInfo, err := proc.MemoryInfoWithContext(ctx)
	if err == nil && memInfo != nil {
		info.MemUsage = memInfo.RSS
	}

	// Get child processes
	children, err := proc.ChildrenWithContext(ctx)
	if err == nil {
		info.Children = make([]int, len(children))
		for i, child := range children {
			info.Children[i] = int(child.Pid)
		}
	} else {
		info.Children = []int{} // empty slice on error
	}

	return info, nil
}

// UpdateProcessInfo updates the process information for an agent.
func (pm *ProcessMonitor) UpdateProcessInfo(agentName string, info ProcessInfo) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	rb, exists := pm.history[agentName]
	if !exists {
		rb = newRingBuffer(pm.historySize)
		pm.history[agentName] = rb
	}
	rb.push(info)
}

// GetHistory returns the process info history for an agent.
func (pm *ProcessMonitor) GetHistory(agentName string) []ProcessInfo {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	rb, exists := pm.history[agentName]
	if !exists {
		return []ProcessInfo{}
	}
	return rb.getAll()
}

// GetLatest returns the most recent process info for an agent.
func (pm *ProcessMonitor) GetLatest(agentName string) *ProcessInfo {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	rb, exists := pm.history[agentName]
	if !exists {
		return nil
	}
	return rb.getLatest()
}

// Clear removes all history for an agent.
func (pm *ProcessMonitor) Clear(agentName string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	delete(pm.history, agentName)
}

// ClearAll removes all history for all agents.
func (pm *ProcessMonitor) ClearAll() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.history = make(map[string]*ringBuffer)
}
