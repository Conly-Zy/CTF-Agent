package memory

import (
	"container/heap"
	"math"
	"math/rand"
	"sort"
	"sync"
)

// HNSWIndex HNSW 向量索引
type HNSWIndex struct {
	dimensions   int
	maxLayers    int
	efConstruction int
	m            int
	mMax         int
	mMax0        int
	ml           float64

	nodes    map[int]*HNSWNode
	entryID  int
	maxLevel int
	mu       sync.RWMutex
}

// HNSWNode HNSW 节点
type HNSWNode struct {
	ID       int
	Vector   []float64
	Links    map[int][]int // layer -> neighbors
	Level    int
}

// NewHNSWIndex 创建新的 HNSW 索引
func NewHNSWIndex(dimensions, m, efConstruction int) *HNSWIndex {
	return &HNSWIndex{
		dimensions:     dimensions,
		maxLayers:      16,
		efConstruction: efConstruction,
		m:              m,
		mMax:           m,
		mMax0:          m * 2,
		ml:             1.0 / math.Log(float64(m)),
		nodes:          make(map[int]*HNSWNode),
		entryID:        -1,
		maxLevel:       -1,
	}
}

// Insert 插入向量
func (idx *HNSWIndex) Insert(id int, vector []float64) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// 创建新节点
	node := &HNSWNode{
		ID:     id,
		Vector: vector,
		Links:  make(map[int][]int),
		Level:  idx.randomLevel(),
	}

	idx.nodes[id] = node

	// 如果是第一个节点
	if idx.entryID == -1 {
		idx.entryID = id
		idx.maxLevel = node.Level
		return
	}

	// 查找最近邻
	entryNode := idx.nodes[idx.entryID]
	currDist := distance(vector, entryNode.Vector)

	// 从顶层向下搜索
	for level := idx.maxLevel; level > node.Level; level-- {
		changed := true
		for changed {
			changed = false
			neighbors := entryNode.Links[level]
			for _, neighborID := range neighbors {
				neighbor := idx.nodes[neighborID]
				dist := distance(vector, neighbor.Vector)
				if dist < currDist {
					currDist = dist
					entryNode = neighbor
					changed = true
				}
			}
		}
	}

	// 在每一层插入
	for level := min(node.Level, idx.maxLevel); level >= 0; level-- {
		neighbors := idx.searchLayer(vector, entryNode.ID, idx.efConstruction, level)
		selected := idx.selectNeighbors(vector, neighbors, idx.m)

		// 添加双向链接
		node.Links[level] = selected
		for _, neighborID := range selected {
			neighbor := idx.nodes[neighborID]
			neighbor.Links[level] = append(neighbor.Links[level], id)

			// 如果超过最大邻居数，裁剪
			maxNeighbors := idx.mMax
			if level == 0 {
				maxNeighbors = idx.mMax0
			}
			if len(neighbor.Links[level]) > maxNeighbors {
				neighbor.Links[level] = idx.selectNeighbors(
					neighbor.Vector,
					neighbor.Links[level],
					maxNeighbors,
				)
			}
		}

		entryNode = idx.nodes[selected[0]]
	}

	// 更新入口点
	if node.Level > idx.maxLevel {
		idx.maxLevel = node.Level
		idx.entryID = id
	}
}

// Search 搜索最近邻
func (idx *HNSWIndex) Search(query []float64, k, ef int) []SearchResult {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if idx.entryID == -1 {
		return nil
	}

	entryNode := idx.nodes[idx.entryID]
	currDist := distance(query, entryNode.Vector)

	// 从顶层向下搜索
	for level := idx.maxLevel; level > 0; level-- {
		changed := true
		for changed {
			changed = false
			neighbors := entryNode.Links[level]
			for _, neighborID := range neighbors {
				neighbor := idx.nodes[neighborID]
				dist := distance(query, neighbor.Vector)
				if dist < currDist {
					currDist = dist
					entryNode = neighbor
					changed = true
				}
			}
		}
	}

	// 在第 0 层搜索 ef 个候选
	candidates := idx.searchLayer(query, entryNode.ID, ef, 0)

	// 返回 top-k
	results := make([]SearchResult, 0, k)
	for i, candidateID := range candidates {
		if i >= k {
			break
		}
		candidate := idx.nodes[candidateID]
		results = append(results, SearchResult{
			ID:         candidateID,
			Similarity: 1.0 - distance(query, candidate.Vector),
		})
	}

	return results
}

// searchLayer 在指定层搜索最近邻
func (idx *HNSWIndex) searchLayer(query []float64, entryID int, ef int, level int) []int {
	visited := make(map[int]bool)
	visited[entryID] = true

	candidates := &MinHeap{}
	heap.Init(candidates)
	heap.Push(candidates, &HeapItem{ID: entryID, Dist: distance(query, idx.nodes[entryID].Vector)})

	results := &MaxHeap{}
	heap.Init(results)
	heap.Push(results, &HeapItem{ID: entryID, Dist: distance(query, idx.nodes[entryID].Vector)})

	for candidates.Len() > 0 {
		curr := heap.Pop(candidates).(*HeapItem)

		if curr.Dist > results.Peek().Dist {
			break
		}

		neighbors := idx.nodes[curr.ID].Links[level]
		for _, neighborID := range neighbors {
			if visited[neighborID] {
				continue
			}
			visited[neighborID] = true

			neighbor := idx.nodes[neighborID]
			dist := distance(query, neighbor.Vector)

			if results.Len() < ef || dist < results.Peek().Dist {
				heap.Push(candidates, &HeapItem{ID: neighborID, Dist: dist})
				heap.Push(results, &HeapItem{ID: neighborID, Dist: dist})

				if results.Len() > ef {
					heap.Pop(results)
				}
			}
		}
	}

	// 转换为有序列表
	resultIDs := make([]int, results.Len())
	for i := results.Len() - 1; i >= 0; i-- {
		resultIDs[i] = heap.Pop(results).(*HeapItem).ID
	}

	return resultIDs
}

// selectNeighbors 选择最近的邻居
func (idx *HNSWIndex) selectNeighbors(query []float64, candidates []int, m int) []int {
	if len(candidates) <= m {
		return candidates
	}

	// 计算距离并排序
	items := make([]HeapItem, len(candidates))
	for i, id := range candidates {
		items[i] = HeapItem{ID: id, Dist: distance(query, idx.nodes[id].Vector)}
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Dist < items[j].Dist
	})

	// 选择前 m 个
	result := make([]int, m)
	for i := 0; i < m; i++ {
		result[i] = items[i].ID
	}

	return result
}

// randomLevel 随机生成层级
func (idx *HNSWIndex) randomLevel() int {
	level := 0
	for rand.Float64() < 1.0/float64(idx.m) && level < idx.maxLayers-1 {
		level++
	}
	return level
}

// SearchResult 搜索结果
type SearchResult struct {
	ID         int
	Similarity float64
}

// HeapItem 堆元素
type HeapItem struct {
	ID   int
	Dist float64
}

// MinHeap 最小堆
type MinHeap []*HeapItem

func (h MinHeap) Len() int           { return len(h) }
func (h MinHeap) Less(i, j int) bool { return h[i].Dist < h[j].Dist }
func (h MinHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *MinHeap) Push(x interface{}) {
	*h = append(*h, x.(*HeapItem))
}

func (h *MinHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

// MaxHeap 最大堆
type MaxHeap []*HeapItem

func (h MaxHeap) Len() int           { return len(h) }
func (h MaxHeap) Less(i, j int) bool { return h[i].Dist > h[j].Dist }
func (h MaxHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *MaxHeap) Push(x interface{}) {
	*h = append(*h, x.(*HeapItem))
}

func (h *MaxHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

func (h MaxHeap) Peek() *HeapItem {
	if len(h) == 0 {
		return &HeapItem{Dist: math.MaxFloat64}
	}
	return h[0]
}

// distance 计算欧氏距离
func distance(a, b []float64) float64 {
	if len(a) != len(b) {
		return math.MaxFloat64
	}

	sum := 0.0
	for i := 0; i < len(a); i++ {
		diff := a[i] - b[i]
		sum += diff * diff
	}

	return math.Sqrt(sum)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
