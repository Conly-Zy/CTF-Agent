package taskqueue

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"
)

// TaskStatus 任务状态
type TaskStatus string

const (
	StatusPending    TaskStatus = "pending"
	StatusRunning    TaskStatus = "running"
	StatusCompleted  TaskStatus = "completed"
	StatusFailed     TaskStatus = "failed"
	StatusCancelled  TaskStatus = "cancelled"
)

// QueuedTask 队列任务
type QueuedTask struct {
	ID          string     `json:"id"`
	SessionID   int64      `json:"session_id"`
	Type        string     `json:"type"`
	Description string     `json:"description"`
	Target      string     `json:"target"`
	Files       []string   `json:"files,omitempty"`
	Status      TaskStatus `json:"status"`
	Priority    int        `json:"priority"`
	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Error       string     `json:"error,omitempty"`
	Result      string     `json:"result,omitempty"`
	RetryCount  int        `json:"retry_count"`
	MaxRetries  int        `json:"max_retries"`
}

// TaskQueue 任务队列
type TaskQueue struct {
	mu       sync.RWMutex
	tasks    []*QueuedTask
	taskMap  map[string]*QueuedTask
	filePath string
	logger   *slog.Logger
	stopCh   chan struct{}
}

// NewTaskQueue 创建任务队列
func NewTaskQueue(filePath string, logger *slog.Logger) *TaskQueue {
	q := &TaskQueue{
		tasks:    make([]*QueuedTask, 0),
		taskMap:  make(map[string]*QueuedTask),
		filePath: filePath,
		logger:   logger,
		stopCh:   make(chan struct{}),
	}

	// 从文件恢复任务
	q.loadFromFile()
	return q
}

// Enqueue 入队任务
func (q *TaskQueue) Enqueue(task *QueuedTask) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if task.ID == "" {
		task.ID = fmt.Sprintf("task-%d", time.Now().UnixNano())
	}
	task.Status = StatusPending
	task.CreatedAt = time.Now()

	q.tasks = append(q.tasks, task)
	q.taskMap[task.ID] = task

	q.logger.Info("task enqueued", "id", task.ID, "type", task.Type)
	q.saveToFile()
	return nil
}

// Dequeue 出队最高优先级的待处理任务
func (q *TaskQueue) Dequeue() *QueuedTask {
	q.mu.Lock()
	defer q.mu.Unlock()

	var best *QueuedTask
	bestIdx := -1
	for i, task := range q.tasks {
		if task.Status == StatusPending {
			if best == nil || task.Priority > best.Priority {
				best = task
				bestIdx = i
			}
		}
	}

	if best != nil {
		best.Status = StatusRunning
		now := time.Now()
		best.StartedAt = &now
		q.tasks = append(q.tasks[:bestIdx], q.tasks[bestIdx+1:]...)
		q.saveToFile()
	}

	return best
}

// Complete 标记任务完成
func (q *TaskQueue) Complete(id string, result string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	task, ok := q.taskMap[id]
	if !ok {
		return fmt.Errorf("task not found: %s", id)
	}

	task.Status = StatusCompleted
	task.Result = result
	now := time.Now()
	task.CompletedAt = &now

	q.logger.Info("task completed", "id", id)
	q.saveToFile()
	return nil
}

// Fail 标记任务失败
func (q *TaskQueue) Fail(id string, errStr string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	task, ok := q.taskMap[id]
	if !ok {
		return fmt.Errorf("task not found: %s", id)
	}

	task.Error = errStr

	// 检查是否可以重试
	if task.RetryCount < task.MaxRetries {
		task.RetryCount++
		task.Status = StatusPending
		task.StartedAt = nil
		q.tasks = append(q.tasks, task)
		q.logger.Info("task retry", "id", id, "attempt", task.RetryCount)
	} else {
		task.Status = StatusFailed
		now := time.Now()
		task.CompletedAt = &now
		q.logger.Info("task failed", "id", id, "error", errStr)
	}

	q.saveToFile()
	return nil
}

// Cancel 取消任务
func (q *TaskQueue) Cancel(id string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	task, ok := q.taskMap[id]
	if !ok {
		return fmt.Errorf("task not found: %s", id)
	}

	if task.Status == StatusCompleted || task.Status == StatusFailed {
		return fmt.Errorf("task already finished: %s", id)
	}

	task.Status = StatusCancelled
	now := time.Now()
	task.CompletedAt = &now

	q.saveToFile()
	return nil
}

// Get 获取任务
func (q *TaskQueue) Get(id string) (*QueuedTask, bool) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	task, ok := q.taskMap[id]
	return task, ok
}

// List 列出任务
func (q *TaskQueue) List(status TaskStatus, limit int) []*QueuedTask {
	q.mu.RLock()
	defer q.mu.RUnlock()

	var result []*QueuedTask
	for i := len(q.tasks) - 1; i >= 0; i-- {
		if status == "" || q.tasks[i].Status == status {
			result = append(result, q.tasks[i])
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}

	// Also check completed/failed tasks in taskMap
	if status != StatusPending && status != StatusRunning {
		for _, task := range q.taskMap {
			if task.Status == status {
				found := false
				for _, r := range result {
					if r.ID == task.ID {
						found = true
						break
					}
				}
				if !found {
					result = append(result, task)
				}
			}
		}
	}

	return result
}

// PendingCount 待处理任务数
func (q *TaskQueue) PendingCount() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	count := 0
	for _, task := range q.tasks {
		if task.Status == StatusPending {
			count++
		}
	}
	return count
}

// Stats 队列统计
func (q *TaskQueue) Stats() map[string]interface{} {
	q.mu.RLock()
	defer q.mu.RUnlock()

	stats := map[string]interface{}{
		"total":    len(q.taskMap),
		"pending":  0,
		"running":  0,
		"completed": 0,
		"failed":   0,
	}

	for _, task := range q.taskMap {
		switch task.Status {
		case StatusPending:
			stats["pending"] = stats["pending"].(int) + 1
		case StatusRunning:
			stats["running"] = stats["running"].(int) + 1
		case StatusCompleted:
			stats["completed"] = stats["completed"].(int) + 1
		case StatusFailed:
			stats["failed"] = stats["failed"].(int) + 1
		}
	}

	return stats
}

func (q *TaskQueue) saveToFile() {
	if q.filePath == "" {
		return
	}
	data, err := json.MarshalIndent(q.taskMap, "", "  ")
	if err != nil {
		q.logger.Error("save task queue", "error", err)
		return
	}
	os.WriteFile(q.filePath, data, 0644)
}

func (q *TaskQueue) loadFromFile() {
	if q.filePath == "" {
		return
	}
	data, err := os.ReadFile(q.filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			q.logger.Error("load task queue", "error", err)
		}
		return
	}

	var taskMap map[string]*QueuedTask
	if err := json.Unmarshal(data, &taskMap); err != nil {
		q.logger.Error("parse task queue", "error", err)
		return
	}

	for id, task := range taskMap {
		q.taskMap[id] = task
		// Re-queue pending and running tasks
		if task.Status == StatusPending || task.Status == StatusRunning {
			task.Status = StatusPending
			task.StartedAt = nil
			q.tasks = append(q.tasks, task)
		}
	}

	q.logger.Info("task queue loaded", "tasks", len(q.taskMap))
}
