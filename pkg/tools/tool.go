package tools

import (
	"context"
	"slices"
	"sync"
	"time"
)

// Tool representa una herramienta de memoria invocable. Es el mismo struct
// plano del agente de Goulm, sin dependencias del bucle ReAct.
type Tool struct {
	Name             string
	Description      string
	Parameters       interface{}
	Execute          func(ctx context.Context, input string) (string, error)
	RequiresApproval bool
	IsReadOnly       bool
	Metadata         ToolMetadata
	RiskLevel        RiskLevel
	Category         ToolCategory
	Timeout          time.Duration
	Tags             []string
}

// Registry es un registro de herramientas con los mismos defaults que el
// agente de Goulm (timeout 30s, categoría por defecto, metadata derivada).
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

func (r *Registry) Register(tool Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if tool.Timeout == 0 {
		tool.Timeout = 30 * time.Second
	}
	if tool.Category == "" {
		tool.Category = CategoryInspect
	}
	if tool.RiskLevel == 0 && !tool.RequiresApproval {
		tool.RiskLevel = RiskLow
	} else if tool.RiskLevel == 0 && tool.RequiresApproval {
		tool.RiskLevel = RiskHigh
	}
	tool.Metadata = ToolMetadata{
		Name:             tool.Name,
		Description:      tool.Description,
		Category:         tool.Category,
		RiskLevel:        tool.RiskLevel,
		RequiresApproval: tool.RequiresApproval,
		Timeout:          tool.Timeout,
		Tags:             tool.Tags,
	}
	r.tools[tool.Name] = tool
}

func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

func (r *Registry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, t)
	}
	return out
}

func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.tools))
	for name := range r.tools {
		out = append(out, name)
	}
	slices.Sort(out)
	return out
}

func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tools)
}
