package tools

import (
	"fmt"
	"sync"
)

// ToolGroup 工具分组
type ToolGroup string

const (
	GroupCommon  ToolGroup = "common"
	GroupWeb     ToolGroup = "web"
	GroupPwn     ToolGroup = "pwn"
	GroupCrypto  ToolGroup = "crypto"
	GroupReverse ToolGroup = "reverse"
	GroupBarrier ToolGroup = "barrier"
)

// AgentType 用于工具分组查询
type AgentType string

const (
	AgentTypePrimary AgentType = "primary"
	AgentTypeWeb     AgentType = "web"
	AgentTypePwn     AgentType = "pwn"
	AgentTypeCrypto  AgentType = "crypto"
	AgentTypeReverse AgentType = "reverse"
)

type Registry struct {
	mu     sync.RWMutex
	tools  map[string]Tool
	groups map[ToolGroup][]string // 分组索引
}

func NewRegistry() *Registry {
	return &Registry{
		tools:  make(map[string]Tool),
		groups: make(map[ToolGroup][]string),
	}
}

func (r *Registry) Register(tool Tool) error {
	return r.RegisterWithGroup(tool, GroupCommon)
}

func (r *Registry) RegisterWithGroup(tool Tool, group ToolGroup) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := tool.Name()
	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("tool %q already registered", name)
	}

	r.tools[name] = tool
	r.groups[group] = append(r.groups[group], name)
	return nil
}

func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, ok := r.tools[name]
	return tool, ok
}

// Remove 移除工具
func (r *Registry) Remove(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.tools, name)
	for group, names := range r.groups {
		for i, n := range names {
			if n == name {
				r.groups[group] = append(names[:i], names[i+1:]...)
				break
			}
		}
	}
}

func (r *Registry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		result = append(result, tool)
	}
	return result
}

func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]string, 0, len(r.tools))
	for name := range r.tools {
		result = append(result, name)
	}
	return result
}

func (r *Registry) ToClaudeTools() []map[string]any {
	return r.ToClaudeToolsForAgent(AgentTypePrimary)
}

func (r *Registry) ToClaudeToolsForAgent(agentType AgentType) []map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := r.getToolsForAgent(agentType)
	result := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		result = append(result, map[string]any{
			"name":         tool.Name(),
			"description":  tool.Description(),
			"input_schema": tool.Schema(),
		})
	}
	return result
}

// GetByGroup 获取指定分组的所有工具
func (r *Registry) GetByGroup(group ToolGroup) []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.getToolsByGroup(group)
}

// GetForAgent 获取特定 Agent 可用的工具
func (r *Registry) GetForAgent(agentType AgentType) []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.getToolsForAgent(agentType)
}

func (r *Registry) getToolsForAgent(agentType AgentType) []Tool {
	var result []Tool

	// 所有 Agent 都可以使用通用工具和屏障工具
	result = append(result, r.getToolsByGroup(GroupCommon)...)
	result = append(result, r.getToolsByGroup(GroupBarrier)...)

	// 根据 Agent 类型添加专业工具
	switch agentType {
	case AgentTypeWeb:
		result = append(result, r.getToolsByGroup(GroupWeb)...)
	case AgentTypePwn:
		result = append(result, r.getToolsByGroup(GroupPwn)...)
	case AgentTypeCrypto:
		result = append(result, r.getToolsByGroup(GroupCrypto)...)
	case AgentTypeReverse:
		result = append(result, r.getToolsByGroup(GroupReverse)...)
	case AgentTypePrimary:
		// Primary Agent 可以使用所有工具
		result = append(result, r.getToolsByGroup(GroupWeb)...)
		result = append(result, r.getToolsByGroup(GroupPwn)...)
		result = append(result, r.getToolsByGroup(GroupCrypto)...)
		result = append(result, r.getToolsByGroup(GroupReverse)...)
	}

	return result
}

func (r *Registry) getToolsByGroup(group ToolGroup) []Tool {
	names, ok := r.groups[group]
	if !ok {
		return nil
	}

	result := make([]Tool, 0, len(names))
	for _, name := range names {
		if tool, exists := r.tools[name]; exists {
			result = append(result, tool)
		}
	}
	return result
}
