package cache

import (
	"sync"
	"time"
)

// CacheItem 缓存项
type CacheItem struct {
	Key        string
	Value      interface{}
	Expiration time.Time
	AccessedAt time.Time
	Size       int
}

// Cache 缓存接口
type Cache interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{}, ttl time.Duration)
	Delete(key string)
	Clear()
	Size() int
	Stats() map[string]interface{}
}

// MemoryCache 内存缓存
type MemoryCache struct {
	mu       sync.RWMutex
	items    map[string]*CacheItem
	maxSize  int
	ttl      time.Duration
	stopCh   chan struct{}
	onEvict  func(key string, value interface{})
}

// NewMemoryCache 创建内存缓存
func NewMemoryCache(maxSize int, defaultTTL time.Duration) *MemoryCache {
	cache := &MemoryCache{
		items:   make(map[string]*CacheItem),
		maxSize: maxSize,
		ttl:     defaultTTL,
		stopCh:  make(chan struct{}),
	}

	// 启动清理协程
	go cache.cleanupLoop()

	return cache
}

// Get 获取缓存
func (c *MemoryCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	item, ok := c.items[key]
	c.mu.RUnlock()

	if !ok {
		return nil, false
	}

	// 检查是否过期
	if time.Now().After(item.Expiration) {
		c.Delete(key)
		return nil, false
	}

	// 更新访问时间
	c.mu.Lock()
	item.AccessedAt = time.Now()
	c.mu.Unlock()

	return item.Value, true
}

// Set 设置缓存
func (c *MemoryCache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 如果达到最大大小，驱逐旧项
	if len(c.items) >= c.maxSize {
		c.evict()
	}

	// 计算过期时间
	expiration := time.Now().Add(ttl)
	if ttl == 0 {
		expiration = time.Now().Add(c.ttl)
	}

	c.items[key] = &CacheItem{
		Key:        key,
		Value:      value,
		Expiration: expiration,
		AccessedAt: time.Now(),
	}
}

// Delete 删除缓存
func (c *MemoryCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, ok := c.items[key]
	if ok {
		if c.onEvict != nil {
			c.onEvict(key, item.Value)
		}
		delete(c.items, key)
	}
}

// Clear 清空缓存
func (c *MemoryCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.onEvict != nil {
		for key, item := range c.items {
			c.onEvict(key, item.Value)
		}
	}

	c.items = make(map[string]*CacheItem)
}

// Size 获取缓存大小
func (c *MemoryCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// Stats 获取缓存统计
func (c *MemoryCache) Stats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return map[string]interface{}{
		"size":     len(c.items),
		"max_size": c.maxSize,
		"ttl":      c.ttl.String(),
	}
}

// SetOnEvict 设置驱逐回调
func (c *MemoryCache) SetOnEvict(callback func(key string, value interface{})) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onEvict = callback
}

// Stop 停止缓存
func (c *MemoryCache) Stop() {
	close(c.stopCh)
}

func (c *MemoryCache) evict() {
	// 使用 LRU 策略驱逐
	var oldestKey string
	var oldestTime time.Time

	for key, item := range c.items {
		if oldestKey == "" || item.AccessedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = item.AccessedAt
		}
	}

	if oldestKey != "" {
		item := c.items[oldestKey]
		if c.onEvict != nil {
			c.onEvict(oldestKey, item.Value)
		}
		delete(c.items, oldestKey)
	}
}

func (c *MemoryCache) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.cleanup()
		case <-c.stopCh:
			return
		}
	}
}

func (c *MemoryCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, item := range c.items {
		if now.After(item.Expiration) {
			if c.onEvict != nil {
				c.onEvict(key, item.Value)
			}
			delete(c.items, key)
		}
	}
}

// LRUCache LRU 缓存
type LRUCache struct {
	mu       sync.RWMutex
	items    map[string]*listItem
	list     *doublyLinkedList
	maxSize  int
	onEvict  func(key string, value interface{})
}

type listItem struct {
	key   string
	value interface{}
	prev  *listItem
	next  *listItem
}

type doublyLinkedList struct {
	head *listItem
	tail *listItem
	size int
}

func newDoublyLinkedList() *doublyLinkedList {
	head := &listItem{}
	tail := &listItem{}
	head.next = tail
	tail.prev = head
	return &doublyLinkedList{
		head: head,
		tail: tail,
	}
}

func (l *doublyLinkedList) pushFront(item *listItem) {
	item.next = l.head.next
	item.prev = l.head
	l.head.next.prev = item
	l.head.next = item
	l.size++
}

func (l *doublyLinkedList) moveToFront(item *listItem) {
	l.remove(item)
	l.pushFront(item)
}

func (l *doublyLinkedList) remove(item *listItem) {
	item.prev.next = item.next
	item.next.prev = item.prev
	l.size--
}

func (l *doublyLinkedList) back() *listItem {
	if l.size == 0 {
		return nil
	}
	return l.tail.prev
}

// NewLRUCache 创建 LRU 缓存
func NewLRUCache(maxSize int) *LRUCache {
	return &LRUCache{
		items:   make(map[string]*listItem),
		list:    newDoublyLinkedList(),
		maxSize: maxSize,
	}
}

// Get 获取缓存
func (c *LRUCache) Get(key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, ok := c.items[key]
	if !ok {
		return nil, false
	}

	// 移动到前面
	c.list.moveToFront(item)

	return item.value, true
}

// Set 设置缓存
func (c *LRUCache) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 如果已存在，更新
	if item, ok := c.items[key]; ok {
		item.value = value
		c.list.moveToFront(item)
		return
	}

	// 如果达到最大大小，驱逐
	if len(c.items) >= c.maxSize {
		c.evict()
	}

	// 添加新项
	item := &listItem{
		key:   key,
		value: value,
	}
	c.items[key] = item
	c.list.pushFront(item)
}

// Delete 删除缓存
func (c *LRUCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, ok := c.items[key]
	if ok {
		c.list.remove(item)
		delete(c.items, key)

		if c.onEvict != nil {
			c.onEvict(key, item.value)
		}
	}
}

// Clear 清空缓存
func (c *LRUCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.onEvict != nil {
		for key, item := range c.items {
			c.onEvict(key, item.value)
		}
	}

	c.items = make(map[string]*listItem)
	c.list = newDoublyLinkedList()
}

// Size 获取缓存大小
func (c *LRUCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

func (c *LRUCache) evict() {
	item := c.list.back()
	if item != nil {
		c.list.remove(item)
		delete(c.items, item.key)

		if c.onEvict != nil {
			c.onEvict(item.key, item.value)
		}
	}
}

// SetOnEvict 设置驱逐回调
func (c *LRUCache) SetOnEvict(callback func(key string, value interface{})) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onEvict = callback
}
