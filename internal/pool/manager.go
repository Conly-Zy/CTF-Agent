package pool

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// ConnectionPool 连接池
type ConnectionPool struct {
	mu          sync.RWMutex
	connections chan interface{}
	factory     func() (interface{}, error)
	closeFunc   func(interface{}) error
	maxSize     int
	minSize     int
	currentSize int
	timeout     time.Duration
	logger      *slog.Logger
	closed      bool
}

// NewConnectionPool 创建连接池
func NewConnectionPool(
	factory func() (interface{}, error),
	closeFunc func(interface{}) error,
	minSize, maxSize int,
	timeout time.Duration,
	logger *slog.Logger,
) (*ConnectionPool, error) {
	if minSize < 0 || maxSize < minSize {
		return nil, fmt.Errorf("invalid pool size: min=%d, max=%d", minSize, maxSize)
	}

	pool := &ConnectionPool{
		connections: make(chan interface{}, maxSize),
		factory:     factory,
		closeFunc:   closeFunc,
		maxSize:     maxSize,
		minSize:     minSize,
		timeout:     timeout,
		logger:      logger,
	}

	// 初始化最小连接数
	for i := 0; i < minSize; i++ {
		conn, err := factory()
		if err != nil {
			// 关闭已创建的连接
			pool.Close()
			return nil, fmt.Errorf("create initial connection: %w", err)
		}
		pool.connections <- conn
		pool.currentSize++
	}

	logger.Info("connection pool created",
		"min_size", minSize,
		"max_size", maxSize)

	return pool, nil
}

// Get 获取连接
func (p *ConnectionPool) Get(ctx context.Context) (interface{}, error) {
	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		return nil, fmt.Errorf("pool is closed")
	}
	p.mu.RUnlock()

	// 尝试从池中获取连接
	select {
	case conn := <-p.connections:
		return conn, nil
	default:
		// 池中没有空闲连接，尝试创建新连接
	}

	// 检查是否达到最大连接数
	p.mu.Lock()
	if p.currentSize < p.maxSize {
		p.currentSize++
		p.mu.Unlock()

		conn, err := p.factory()
		if err != nil {
			p.mu.Lock()
			p.currentSize--
			p.mu.Unlock()
			return nil, fmt.Errorf("create connection: %w", err)
		}
		return conn, nil
	}
	p.mu.Unlock()

	// 等待连接释放
	select {
	case conn := <-p.connections:
		return conn, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(p.timeout):
		return nil, fmt.Errorf("timeout waiting for connection")
	}
}

// Put 归还连接
func (p *ConnectionPool) Put(conn interface{}) error {
	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		return p.closeFunc(conn)
	}
	p.mu.RUnlock()

	select {
	case p.connections <- conn:
		return nil
	default:
		// 池已满，关闭连接
		p.mu.Lock()
		p.currentSize--
		p.mu.Unlock()
		return p.closeFunc(conn)
	}
}

// Close 关闭连接池
func (p *ConnectionPool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	p.closed = true
	close(p.connections)

	// 关闭所有连接
	for conn := range p.connections {
		if err := p.closeFunc(conn); err != nil {
			p.logger.Error("close connection error", "error", err)
		}
	}

	p.logger.Info("connection pool closed")
	return nil
}

// Stats 获取连接池统计
func (p *ConnectionPool) Stats() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return map[string]interface{}{
		"current_size": p.currentSize,
		"max_size":     p.maxSize,
		"min_size":     p.minSize,
		"available":    len(p.connections),
		"closed":       p.closed,
	}
}

// WorkerPool 工作池
type WorkerPool struct {
	mu       sync.RWMutex
	workers  int
	taskCh   chan func()
	quit     chan struct{}
	wg       sync.WaitGroup
	logger   *slog.Logger
	running  bool
}

// NewWorkerPool 创建工作池
func NewWorkerPool(workers int, queueSize int, logger *slog.Logger) *WorkerPool {
	return &WorkerPool{
		workers: workers,
		taskCh:  make(chan func(), queueSize),
		quit:    make(chan struct{}),
		logger:  logger,
	}
}

// Start 启动工作池
func (p *WorkerPool) Start() {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return
	}
	p.running = true
	p.mu.Unlock()

	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}

	p.logger.Info("worker pool started", "workers", p.workers)
}

// Stop 停止工作池
func (p *WorkerPool) Stop() {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return
	}
	p.running = false
	p.mu.Unlock()

	close(p.quit)
	p.wg.Wait()

	p.logger.Info("worker pool stopped")
}

// Submit 提交任务
func (p *WorkerPool) Submit(task func()) error {
	p.mu.RLock()
	if !p.running {
		p.mu.RUnlock()
		return fmt.Errorf("worker pool is not running")
	}
	p.mu.RUnlock()

	select {
	case p.taskCh <- task:
		return nil
	default:
		return fmt.Errorf("task queue is full")
	}
}

// SubmitWithContext 提交带上下文的任务
func (p *WorkerPool) SubmitWithContext(ctx context.Context, task func(context.Context)) error {
	p.mu.RLock()
	if !p.running {
		p.mu.RUnlock()
		return fmt.Errorf("worker pool is not running")
	}
	p.mu.RUnlock()

	wrappedTask := func() {
		task(ctx)
	}

	select {
	case p.taskCh <- wrappedTask:
		return nil
	default:
		return fmt.Errorf("task queue is full")
	}
}

func (p *WorkerPool) worker(id int) {
	defer p.wg.Done()

	for {
		select {
		case task, ok := <-p.taskCh:
			if !ok {
				return
			}
			p.executeTask(id, task)
		case <-p.quit:
			return
		}
	}
}

func (p *WorkerPool) executeTask(workerID int, task func()) {
	defer func() {
		if r := recover(); r != nil {
			p.logger.Error("worker panic",
				"worker_id", workerID,
				"error", r)
		}
	}()

	task()
}

// Stats 获取工作池统计
func (p *WorkerPool) Stats() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return map[string]interface{}{
		"workers":    p.workers,
		"queue_size": len(p.taskCh),
		"queue_cap":  cap(p.taskCh),
		"running":    p.running,
	}
}

// Semaphore 信号量
type Semaphore struct {
	ch chan struct{}
}

// NewSemaphore 创建信号量
func NewSemaphore(maxConcurrent int) *Semaphore {
	return &Semaphore{
		ch: make(chan struct{}, maxConcurrent),
	}
}

// Acquire 获取信号量
func (s *Semaphore) Acquire() {
	s.ch <- struct{}{}
}

// Release 释放信号量
func (s *Semaphore) Release() {
	<-s.ch
}

// TryAcquire 尝试获取信号量
func (s *Semaphore) TryAcquire() bool {
	select {
	case s.ch <- struct{}{}:
		return true
	default:
		return false
	}
}

// AcquireWithContext 带上下文获取信号量
func (s *Semaphore) AcquireWithContext(ctx context.Context) error {
	select {
	case s.ch <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Available 获取可用信号量数
func (s *Semaphore) Available() int {
	return cap(s.ch) - len(s.ch)
}
