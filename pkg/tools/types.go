// Package tools expone la capa de herramientas de memoria como un Registry
// standalone. Portado de Goulm (pkg/tools/core + pkg/agent) y adaptado para
// no depender del agente: solo necesita un *memory.MemoryStore (y, opcional,
// un *memory.SessionTracker / *memory.Ledger).
package tools

import "time"

type RiskLevel int

const (
	RiskLow RiskLevel = iota
	RiskMedium
	RiskHigh
	RiskCritical
)

func (r RiskLevel) String() string {
	switch r {
	case RiskLow:
		return "Low"
	case RiskMedium:
		return "Medium"
	case RiskHigh:
		return "High"
	case RiskCritical:
		return "Critical"
	default:
		return "Unknown"
	}
}

func (r RiskLevel) Color() string {
	switch r {
	case RiskLow:
		return "green"
	case RiskMedium:
		return "yellow"
	case RiskHigh:
		return "orange"
	case RiskCritical:
		return "red"
	default:
		return "gray"
	}
}

type ToolCategory string

const (
	CategoryFile    ToolCategory = "file"
	CategorySystem  ToolCategory = "system"
	CategoryGit     ToolCategory = "git"
	CategorySearch  ToolCategory = "search"
	CategoryInspect ToolCategory = "inspect"
	CategoryManage  ToolCategory = "manage"
	CategoryDynamic ToolCategory = "dynamic"
	CategoryCustom  ToolCategory = "custom"
)

type ToolMetadata struct {
	Name             string
	Description      string
	Category         ToolCategory
	RiskLevel        RiskLevel
	RequiresApproval bool
	Timeout          time.Duration
	Tags             []string
	Version          string
}
