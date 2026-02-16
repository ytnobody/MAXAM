package taskqueue

import (
	"errors"
	"sync"
	"time"
)

var (
	// ErrTaskNotFound is returned when a task is not found.
	ErrTaskNotFound = errors.New("task not found")
	// ErrTaskNotHandoverable is returned when a task cannot be handed over.
	ErrTaskNotHandoverable = errors.New("task is not handoverable")
	// ErrMaxHandoversExceeded is returned when max handovers are exceeded.
	ErrMaxHandoversExceeded = errors.New("max handovers exceeded")
	// ErrNoAvailableAgent is returned when no agent is available for handover.
	ErrNoAvailableAgent = errors.New("no available agent for handover")
	// ErrAgentNotFound is returned when an agent is not found.
	ErrAgentNotFound = errors.New("agent not found")
)

// Queue manages tasks and their assignments to agents.
type Queue struct {
	mu           sync.RWMutex
	tasks        map[string]*Task
	agents       map[string]*AgentCapability
	tasksByAgent map[string][]string // agentName -> []taskID
}

// NewQueue creates a new task queue.
func NewQueue() *Queue {
	return &Queue{
		tasks:        make(map[string]*Task),
		agents:       make(map[string]*AgentCapability),
		tasksByAgent: make(map[string][]string),
	}
}

// RegisterAgent registers an agent with its capabilities.
func (q *Queue) RegisterAgent(cap AgentCapability) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.agents[cap.AgentName] = &cap
	if _, exists := q.tasksByAgent[cap.AgentName]; !exists {
		q.tasksByAgent[cap.AgentName] = []string{}
	}
}

// UnregisterAgent removes an agent from the queue.
func (q *Queue) UnregisterAgent(agentName string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	delete(q.agents, agentName)
	delete(q.tasksByAgent, agentName)
}

// SetAgentAvailability sets whether an agent is available for new tasks.
func (q *Queue) SetAgentAvailability(agentName string, available bool) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	agent, exists := q.agents[agentName]
	if !exists {
		return ErrAgentNotFound
	}
	agent.Available = available
	return nil
}

// AddTask adds a new task to the queue.
func (q *Queue) AddTask(task *Task) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if task.CreatedAt.IsZero() {
		task.CreatedAt = time.Now()
	}
	if task.Status == "" {
		task.Status = TaskStatusPending
	}
	if task.Metadata == nil {
		task.Metadata = make(map[string]string)
	}
	if task.PreviousAgents == nil {
		task.PreviousAgents = []string{}
	}

	q.tasks[task.ID] = task

	if task.AssignedTo != "" {
		q.tasksByAgent[task.AssignedTo] = append(q.tasksByAgent[task.AssignedTo], task.ID)
		q.updateAgentTaskCount(task.AssignedTo)
	}
}

// GetTask returns a task by ID.
func (q *Queue) GetTask(taskID string) (*Task, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	task, exists := q.tasks[taskID]
	if !exists {
		return nil, ErrTaskNotFound
	}

	// Return a copy to prevent external mutations.
	t := *task
	t.Metadata = copyMap(task.Metadata)
	t.PreviousAgents = copySlice(task.PreviousAgents)
	return &t, nil
}

// AssignTask assigns a task to an agent.
func (q *Queue) AssignTask(taskID, agentName string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	task, exists := q.tasks[taskID]
	if !exists {
		return ErrTaskNotFound
	}

	agent, exists := q.agents[agentName]
	if !exists {
		return ErrAgentNotFound
	}

	// Remove from old agent if reassigning.
	if task.AssignedTo != "" && task.AssignedTo != agentName {
		q.removeTaskFromAgent(task.AssignedTo, taskID)
	}

	task.AssignedTo = agentName
	task.Status = TaskStatusInProgress
	task.StartedAt = time.Now()

	q.tasksByAgent[agentName] = append(q.tasksByAgent[agentName], taskID)
	agent.CurrentTasks++

	return nil
}

// CompleteTask marks a task as completed.
func (q *Queue) CompleteTask(taskID string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	task, exists := q.tasks[taskID]
	if !exists {
		return ErrTaskNotFound
	}

	task.Status = TaskStatusCompleted
	task.CompletedAt = time.Now()

	if task.AssignedTo != "" {
		q.removeTaskFromAgent(task.AssignedTo, taskID)
	}

	return nil
}

// FailTask marks a task as failed.
func (q *Queue) FailTask(taskID, reason string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	task, exists := q.tasks[taskID]
	if !exists {
		return ErrTaskNotFound
	}

	task.Status = TaskStatusFailed
	task.CompletedAt = time.Now()
	task.Metadata["fail_reason"] = reason

	if task.AssignedTo != "" {
		q.removeTaskFromAgent(task.AssignedTo, taskID)
	}

	return nil
}

// Handover attempts to hand over a task to another agent.
func (q *Queue) Handover(req HandoverRequest) HandoverResult {
	q.mu.Lock()
	defer q.mu.Unlock()

	result := HandoverResult{
		TaskID:    req.TaskID,
		FromAgent: req.FromAgent,
	}

	task, exists := q.tasks[req.TaskID]
	if !exists {
		result.Error = ErrTaskNotFound.Error()
		return result
	}

	if !task.Handoverable {
		result.Error = ErrTaskNotHandoverable.Error()
		return result
	}

	if task.MaxHandovers > 0 && task.HandoverCount >= task.MaxHandovers {
		result.Error = ErrMaxHandoversExceeded.Error()
		return result
	}

	// Find target agent.
	targetAgent := req.ToAgent
	if targetAgent == "" {
		// Auto-assign to an available agent.
		targetAgent = q.findAvailableAgentLocked(req.FromAgent, task.PreviousAgents)
		if targetAgent == "" {
			result.Error = ErrNoAvailableAgent.Error()
			return result
		}
	}

	// Verify target agent exists and is available.
	agent, exists := q.agents[targetAgent]
	if !exists {
		result.Error = ErrAgentNotFound.Error()
		return result
	}
	if !agent.Available {
		result.Error = "target agent is not available"
		return result
	}
	if agent.MaxTasks > 0 && agent.CurrentTasks >= agent.MaxTasks {
		result.Error = "target agent has reached max tasks"
		return result
	}

	// Perform handover.
	if task.AssignedTo != "" {
		task.PreviousAgents = append(task.PreviousAgents, task.AssignedTo)
		q.removeTaskFromAgent(task.AssignedTo, req.TaskID)
	}

	task.AssignedTo = targetAgent
	task.Status = TaskStatusHandover
	task.HandoverCount++
	task.Metadata["handover_reason"] = req.Reason
	task.Metadata["handover_time"] = time.Now().Format(time.RFC3339)

	q.tasksByAgent[targetAgent] = append(q.tasksByAgent[targetAgent], req.TaskID)
	agent.CurrentTasks++

	result.Success = true
	result.ToAgent = targetAgent
	return result
}

// GetTasksForAgent returns all tasks assigned to an agent.
func (q *Queue) GetTasksForAgent(agentName string) []*Task {
	q.mu.RLock()
	defer q.mu.RUnlock()

	taskIDs, exists := q.tasksByAgent[agentName]
	if !exists {
		return nil
	}

	tasks := make([]*Task, 0, len(taskIDs))
	for _, id := range taskIDs {
		if task, exists := q.tasks[id]; exists {
			t := *task
			t.Metadata = copyMap(task.Metadata)
			t.PreviousAgents = copySlice(task.PreviousAgents)
			tasks = append(tasks, &t)
		}
	}

	return tasks
}

// GetPendingTasks returns all pending tasks.
func (q *Queue) GetPendingTasks() []*Task {
	q.mu.RLock()
	defer q.mu.RUnlock()

	var tasks []*Task
	for _, task := range q.tasks {
		if task.Status == TaskStatusPending {
			t := *task
			t.Metadata = copyMap(task.Metadata)
			t.PreviousAgents = copySlice(task.PreviousAgents)
			tasks = append(tasks, &t)
		}
	}

	return tasks
}

// GetHandoverableTasks returns tasks that can be handed over for a given agent.
func (q *Queue) GetHandoverableTasks(agentName string) []*Task {
	q.mu.RLock()
	defer q.mu.RUnlock()

	taskIDs, exists := q.tasksByAgent[agentName]
	if !exists {
		return nil
	}

	var tasks []*Task
	for _, id := range taskIDs {
		task, exists := q.tasks[id]
		if !exists {
			continue
		}
		if !task.Handoverable {
			continue
		}
		if task.MaxHandovers > 0 && task.HandoverCount >= task.MaxHandovers {
			continue
		}

		t := *task
		t.Metadata = copyMap(task.Metadata)
		t.PreviousAgents = copySlice(task.PreviousAgents)
		tasks = append(tasks, &t)
	}

	return tasks
}

// GetAvailableAgents returns agents that can accept new tasks.
func (q *Queue) GetAvailableAgents() []AgentCapability {
	q.mu.RLock()
	defer q.mu.RUnlock()

	var agents []AgentCapability
	for _, agent := range q.agents {
		if !agent.Available {
			continue
		}
		if agent.MaxTasks > 0 && agent.CurrentTasks >= agent.MaxTasks {
			continue
		}
		agents = append(agents, *agent)
	}

	return agents
}

// RemoveTask removes a task from the queue.
func (q *Queue) RemoveTask(taskID string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	task, exists := q.tasks[taskID]
	if !exists {
		return ErrTaskNotFound
	}

	if task.AssignedTo != "" {
		q.removeTaskFromAgent(task.AssignedTo, taskID)
	}

	delete(q.tasks, taskID)
	return nil
}

// --- Internal helpers ---

func (q *Queue) removeTaskFromAgent(agentName, taskID string) {
	tasks := q.tasksByAgent[agentName]
	for i, id := range tasks {
		if id == taskID {
			q.tasksByAgent[agentName] = append(tasks[:i], tasks[i+1:]...)
			break
		}
	}
	q.updateAgentTaskCount(agentName)
}

func (q *Queue) updateAgentTaskCount(agentName string) {
	if agent, exists := q.agents[agentName]; exists {
		agent.CurrentTasks = len(q.tasksByAgent[agentName])
	}
}

func (q *Queue) findAvailableAgentLocked(excludeAgent string, excludePrevious []string) string {
	excluded := make(map[string]bool)
	excluded[excludeAgent] = true
	for _, agent := range excludePrevious {
		excluded[agent] = true
	}

	var bestAgent string
	minTasks := -1

	for name, agent := range q.agents {
		if excluded[name] {
			continue
		}
		if !agent.Available {
			continue
		}
		if agent.MaxTasks > 0 && agent.CurrentTasks >= agent.MaxTasks {
			continue
		}

		// Choose agent with fewest tasks.
		if minTasks < 0 || agent.CurrentTasks < minTasks {
			bestAgent = name
			minTasks = agent.CurrentTasks
		}
	}

	return bestAgent
}

func copyMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	result := make(map[string]string, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}

func copySlice(s []string) []string {
	if s == nil {
		return nil
	}
	result := make([]string, len(s))
	copy(result, s)
	return result
}
