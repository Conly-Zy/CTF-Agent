package agent

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// ParallelExecutor 并行执行器
type ParallelExecutor struct {
	logger     *slog.Logger
	maxWorkers int
}

func NewParallelExecutor(logger *slog.Logger, maxWorkers int) *ParallelExecutor {
	return &ParallelExecutor{
		logger:     logger,
		maxWorkers: maxWorkers,
	}
}

// TaskResult 任务结果
type TaskResult struct {
	TaskID  string
	Result  *Result
	Error   error
	Elapsed time.Duration
}

// ExecuteParallel 并行执行多个任务
func (e *ParallelExecutor) ExecuteParallel(ctx context.Context, tasks []Task, agents map[AgentType]Agent) []TaskResult {
	results := make([]TaskResult, len(tasks))
	taskCh := make(chan int, len(tasks))
	resultCh := make(chan int, len(tasks))

	// 启动 worker
	var wg sync.WaitGroup
	for i := 0; i < e.maxWorkers && i < len(tasks); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for taskIdx := range taskCh {
				task := tasks[taskIdx]
				start := time.Now()

				// 获取对应的 Agent
				agentType := AgentType(task.Type)
				agent, ok := agents[agentType]
				if !ok {
					// 默认使用 PrimaryAgent
					agent = agents[AgentTypePrimary]
				}

				// 执行任务
				result, err := agent.Run(ctx, task)

				results[taskIdx] = TaskResult{
					TaskID:  task.ID,
					Result:  result,
					Error:   err,
					Elapsed: time.Since(start),
				}

				resultCh <- taskIdx
			}
		}()
	}

	// 分发任务
	go func() {
		for i := range tasks {
			taskCh <- i
		}
		close(taskCh)
	}()

	// 等待结果
	for i := 0; i < len(tasks); i++ {
		<-resultCh
	}

	wg.Wait()

	return results
}

// ExecuteWithFallback 带回退的并行执行
func (e *ParallelExecutor) ExecuteWithFallback(ctx context.Context, task Task, agents []Agent) *Result {
	if len(agents) == 0 {
		return &Result{
			TaskID: task.ID,
			Success: false,
			Error:  "no agents available",
		}
	}

	// 如果只有一个 Agent，直接执行
	if len(agents) == 1 {
		result, err := agents[0].Run(ctx, task)
		if err != nil {
			return &Result{
				TaskID: task.ID,
				Success: false,
				Error:  err.Error(),
			}
		}
		return result
	}

	// 并行执行，使用第一个成功的结果
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	resultCh := make(chan *Result, len(agents))
	errorCh := make(chan error, len(agents))

	var wg sync.WaitGroup
	for _, agent := range agents {
		wg.Add(1)
		go func(a Agent) {
			defer wg.Done()
			result, err := a.Run(ctx, task)
			if err != nil {
				errorCh <- err
				return
			}
			resultCh <- result
		}(agent)
	}

	// 等待第一个成功的结果
	go func() {
		wg.Wait()
		close(resultCh)
		close(errorCh)
	}()

	// 收集结果
	var lastError error
	for {
		select {
		case result, ok := <-resultCh:
			if !ok {
				// 所有任务完成，没有成功的结果
				if lastError != nil {
					return &Result{
						TaskID: task.ID,
						Success: false,
						Error:  lastError.Error(),
					}
				}
				return &Result{
					TaskID: task.ID,
					Success: false,
					Error:  "all agents failed",
				}
			}
			if result.Success {
				cancel() // 取消其他任务
				return result
			}
		case err, ok := <-errorCh:
			if ok {
				lastError = err
			}
		case <-ctx.Done():
			return &Result{
				TaskID: task.ID,
				Success: false,
				Error:  "context cancelled",
			}
		}
	}
}

// ExecuteWithRetry 带重试的执行
func (e *ParallelExecutor) ExecuteWithRetry(ctx context.Context, task Task, agent Agent, maxRetries int) *Result {
	var lastResult *Result
	var lastError error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			e.logger.Info("retrying task",
				"task_id", task.ID,
				"attempt", attempt,
				"max_retries", maxRetries)

			// 指数退避
			backoff := time.Duration(attempt*100) * time.Millisecond
			if backoff > 2*time.Second {
				backoff = 2 * time.Second
			}
			time.Sleep(backoff)
		}

		result, err := agent.Run(ctx, task)
		if err != nil {
			lastError = err
			continue
		}

		if result.Success {
			return result
		}

		lastResult = result
	}

	if lastError != nil {
		return &Result{
			TaskID: task.ID,
			Success: false,
			Error:  fmt.Sprintf("failed after %d retries: %v", maxRetries, lastError),
		}
	}

	return lastResult
}

// TaskScheduler 任务调度器
type TaskScheduler struct {
	logger     *slog.Logger
	executor   *ParallelExecutor
	taskQueue  chan Task
	resultCh   chan TaskResult
	agents     map[AgentType]Agent
	workers    int
	mu         sync.RWMutex
	running    bool
}

func NewTaskScheduler(logger *slog.Logger, workers int) *TaskScheduler {
	return &TaskScheduler{
		logger:    logger,
		executor:  NewParallelExecutor(logger, workers),
		taskQueue: make(chan Task, 100),
		resultCh:  make(chan TaskResult, 100),
		agents:    make(map[AgentType]Agent),
		workers:   workers,
	}
}

// RegisterAgent 注册 Agent
func (s *TaskScheduler) RegisterAgent(agentType AgentType, agent Agent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.agents[agentType] = agent
}

// Start 启动调度器
func (s *TaskScheduler) Start(ctx context.Context) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	// 启动 worker
	for i := 0; i < s.workers; i++ {
		go s.worker(ctx, i)
	}

	s.logger.Info("task scheduler started", "workers", s.workers)
}

// Stop 停止调度器
func (s *TaskScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = false
	close(s.taskQueue)
}

// Submit 提交任务
func (s *TaskScheduler) Submit(task Task) {
	s.taskQueue <- task
}

// Results 获取结果通道
func (s *TaskScheduler) Results() <-chan TaskResult {
	return s.resultCh
}

func (s *TaskScheduler) worker(ctx context.Context, id int) {
	for task := range s.taskQueue {
		s.mu.RLock()
		agentType := AgentType(task.Type)
		agent, ok := s.agents[agentType]
		if !ok {
			agent = s.agents[AgentTypePrimary]
		}
		s.mu.RUnlock()

		if agent == nil {
			s.resultCh <- TaskResult{
				TaskID: task.ID,
				Error:  fmt.Errorf("no agent for type %s", task.Type),
			}
			continue
		}

		start := time.Now()
		result, err := agent.Run(ctx, task)

		s.resultCh <- TaskResult{
			TaskID:  task.ID,
			Result:  result,
			Error:   err,
			Elapsed: time.Since(start),
		}
	}
}
