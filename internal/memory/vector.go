package memory

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

// VectorStore 向量存储接口
type VectorStore interface {
	// Store 存储向量
	Store(ctx context.Context, entry VectorEntry) error
	// Search 语义搜索
	Search(ctx context.Context, query string, limit int) ([]VectorEntry, error)
	// Delete 删除向量
	Delete(ctx context.Context, id string) error
	// List 列出所有向量
	List(ctx context.Context, limit int) ([]VectorEntry, error)
}

// VectorEntry 向量条目
type VectorEntry struct {
	ID       string    `json:"id"`
	Content  string    `json:"content"`
	Vector   []float64 `json:"vector"`
	Metadata Metadata  `json:"metadata"`
}

// Metadata 元数据
type Metadata struct {
	Type      string   `json:"type"`      // guide, answer, code
	Title     string   `json:"title"`
	Tags      []string `json:"tags"`
	SessionID int64    `json:"session_id"`
}

// SimpleVectorStore 简单的向量存储实现
type SimpleVectorStore struct {
	logger  *slog.Logger
	entries map[string]*VectorEntry
	mu      sync.RWMutex
	embedder Embedder
}

// Embedder 向量嵌入接口
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float64, error)
}

func NewSimpleVectorStore(logger *slog.Logger, embedder Embedder) *SimpleVectorStore {
	return &SimpleVectorStore{
		logger:   logger,
		entries:  make(map[string]*VectorEntry),
		embedder: embedder,
	}
}

// Store 存储向量
func (s *SimpleVectorStore) Store(ctx context.Context, entry VectorEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 如果没有向量，生成一个
	if len(entry.Vector) == 0 && s.embedder != nil {
		vector, err := s.embedder.Embed(ctx, entry.Content)
		if err != nil {
			return fmt.Errorf("embed failed: %w", err)
		}
		entry.Vector = vector
	}

	s.entries[entry.ID] = &entry
	s.logger.Info("stored vector entry",
		"id", entry.ID,
		"type", entry.Metadata.Type,
		"content_len", len(entry.Content))

	return nil
}

// Search 语义搜索
func (s *SimpleVectorStore) Search(ctx context.Context, query string, limit int) ([]VectorEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 生成查询向量
	queryVector, err := s.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embed query failed: %w", err)
	}

	// 计算相似度
	type scoredEntry struct {
		entry      *VectorEntry
		similarity float64
	}

	var scored []scoredEntry
	for _, entry := range s.entries {
		if len(entry.Vector) > 0 {
			similarity := cosineSimilarity(queryVector, entry.Vector)
			scored = append(scored, scoredEntry{
				entry:      entry,
				similarity: similarity,
			})
		}
	}

	// 按相似度排序
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].similarity > scored[j].similarity
	})

	// 返回 top-k 结果
	result := make([]VectorEntry, 0, limit)
	for i := 0; i < limit && i < len(scored); i++ {
		result = append(result, *scored[i].entry)
		s.logger.Debug("search result",
			"id", scored[i].entry.ID,
			"similarity", scored[i].similarity)
	}

	return result, nil
}

// Delete 删除向量
func (s *SimpleVectorStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.entries, id)
	return nil
}

// List 列出所有向量
func (s *SimpleVectorStore) List(ctx context.Context, limit int) ([]VectorEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]VectorEntry, 0, limit)
	for _, entry := range s.entries {
		result = append(result, *entry)
		if len(result) >= limit {
			break
		}
	}

	return result, nil
}

// cosineSimilarity 计算余弦相似度
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := 0; i < len(a); i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// MemoryManager 记忆管理器
type MemoryManager struct {
	logger      *slog.Logger
	vectorStore VectorStore
}

func NewMemoryManager(logger *slog.Logger, vectorStore VectorStore) *MemoryManager {
	return &MemoryManager{
		logger:      logger,
		vectorStore: vectorStore,
	}
}

// StoreGuide 存储漏洞利用方法
func (m *MemoryManager) StoreGuide(ctx context.Context, title, content string, tags []string) error {
	return m.vectorStore.Store(ctx, VectorEntry{
		ID:      fmt.Sprintf("guide-%s", generateID()),
		Content: content,
		Metadata: Metadata{
			Type:  "guide",
			Title: title,
			Tags:  tags,
		},
	})
}

// StoreAnswer 存储 CTF 题目解法
func (m *MemoryManager) StoreAnswer(ctx context.Context, title, content string, sessionID int64, tags []string) error {
	return m.vectorStore.Store(ctx, VectorEntry{
		ID:      fmt.Sprintf("answer-%s", generateID()),
		Content: content,
		Metadata: Metadata{
			Type:      "answer",
			Title:     title,
			Tags:      tags,
			SessionID: sessionID,
		},
	})
}

// StoreCode 存储代码片段
func (m *MemoryManager) StoreCode(ctx context.Context, title, content string, tags []string) error {
	return m.vectorStore.Store(ctx, VectorEntry{
		ID:      fmt.Sprintf("code-%s", generateID()),
		Content: content,
		Metadata: Metadata{
			Type:  "code",
			Title: title,
			Tags:  tags,
		},
	})
}

// SearchGuides 搜索漏洞利用方法
func (m *MemoryManager) SearchGuides(ctx context.Context, query string, limit int) ([]VectorEntry, error) {
	results, err := m.vectorStore.Search(ctx, query, limit*2)
	if err != nil {
		return nil, err
	}

	// 过滤出 guide 类型
	var guides []VectorEntry
	for _, entry := range results {
		if entry.Metadata.Type == "guide" {
			guides = append(guides, entry)
			if len(guides) >= limit {
				break
			}
		}
	}

	return guides, nil
}

// SearchAnswers 搜索 CTF 题目解法
func (m *MemoryManager) SearchAnswers(ctx context.Context, query string, limit int) ([]VectorEntry, error) {
	results, err := m.vectorStore.Search(ctx, query, limit*2)
	if err != nil {
		return nil, err
	}

	// 过滤出 answer 类型
	var answers []VectorEntry
	for _, entry := range results {
		if entry.Metadata.Type == "answer" {
			answers = append(answers, entry)
			if len(answers) >= limit {
				break
			}
		}
	}

	return answers, nil
}

// SearchCode 搜索代码片段
func (m *MemoryManager) SearchCode(ctx context.Context, query string, limit int) ([]VectorEntry, error) {
	results, err := m.vectorStore.Search(ctx, query, limit*2)
	if err != nil {
		return nil, err
	}

	// 过滤出 code 类型
	var codes []VectorEntry
	for _, entry := range results {
		if entry.Metadata.Type == "code" {
			codes = append(codes, entry)
			if len(codes) >= limit {
				break
			}
		}
	}

	return codes, nil
}

// generateID 生成唯一 ID
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// SimpleEmbedder 简单的嵌入实现（使用 TF-IDF 近似）
type SimpleEmbedder struct {
	vocabulary map[string]int
	dimension  int
}

func NewSimpleEmbedder(dimension int) *SimpleEmbedder {
	return &SimpleEmbedder{
		vocabulary: make(map[string]int),
		dimension:  dimension,
	}
}

// Embed 生成文本的向量表示
func (e *SimpleEmbedder) Embed(ctx context.Context, text string) ([]float64, error) {
	// 简单的 TF-IDF 近似实现
	words := tokenize(text)
	vector := make([]float64, e.dimension)

	for _, word := range words {
		// 使用哈希将单词映射到向量维度
		idx := hashWord(word, e.dimension)
		vector[idx] += 1.0
	}

	// 归一化
	norm := 0.0
	for _, v := range vector {
		norm += v * v
	}
	if norm > 0 {
		norm = math.Sqrt(norm)
		for i := range vector {
			vector[i] /= norm
		}
	}

	return vector, nil
}

// tokenize 分词
func tokenize(text string) []string {
	// 简单的分词实现
	words := strings.Fields(strings.ToLower(text))
	var tokens []string
	for _, word := range words {
		// 移除标点符号
		word = strings.Trim(word, ".,;:!?\"'()[]{}")
		if len(word) > 2 {
			tokens = append(tokens, word)
		}
	}
	return tokens
}

// hashWord 将单词哈希到指定范围
func hashWord(word string, max int) int {
	hash := 0
	for _, ch := range word {
		hash = hash*31 + int(ch)
	}
	if hash < 0 {
		hash = -hash
	}
	return hash % max
}
