package mcp

import (
	"context"
	"encoding/json"

	"github.com/LRGolden/goulm-memory/pkg/tools"
)

// Adapter puentea el Registry de goulm-memory hacia un Server MCP.
type Adapter struct {
	reg *tools.Registry
}

// NewAdapter crea un nuevo puente.
func NewAdapter(reg *tools.Registry) *Adapter {
	return &Adapter{reg: reg}
}

// RegisterHandlers registra los métodos "initialize", "tools/list" y "tools/call".
func (a *Adapter) RegisterHandlers(s *Server) {
	s.Handle("initialize", a.handleInitialize)
	s.Handle("ping", a.handlePing)
	s.Handle("tools/list", a.handleToolsList)
	s.Handle("tools/call", a.handleToolsCall)
}

func (a *Adapter) handlePing(ctx context.Context, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{}, nil
}

func (a *Adapter) handleInitialize(ctx context.Context, params json.RawMessage) (interface{}, error) {
	return InitializeResult{
		ProtocolVersion: "2024-11-05", // Versión actual del protocolo MCP
		Capabilities: ServerCapabilities{
			Tools: map[string]interface{}{},
		},
		ServerInfo: ServerInfo{
			Name:    "goulm-memory-mcp",
			Version: "0.4.8",
		},
	}, nil
}

func (a *Adapter) handleToolsList(ctx context.Context, params json.RawMessage) (interface{}, error) {
	allTools := a.reg.List()
	mcpTools := make([]Tool, 0, len(allTools))

	for _, t := range allTools {
		mcpTools = append(mcpTools, Tool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.Parameters,
		})
	}

	return ListToolsResult{
		Tools: mcpTools,
	}, nil
}

func (a *Adapter) handleToolsCall(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var call CallToolParams
	if err := json.Unmarshal(params, &call); err != nil {
		return nil, err
	}

	t, ok := a.reg.Get(call.Name)
	if !ok {
		// Tool not found
		return CallToolResult{
			Content: []Content{{Type: "text", Text: "Herramienta no encontrada: " + call.Name}},
			IsError: true,
		}, nil
	}

	// MCP envía los parámetros estructurados en JSON object (Arguments).
	// Nuestra interfaz Execute espera un string JSON.
	argsBytes, err := json.Marshal(call.Arguments)
	if err != nil {
		return CallToolResult{
			Content: []Content{{Type: "text", Text: "Error codificando argumentos: " + err.Error()}},
			IsError: true,
		}, nil
	}

	out, err := t.Execute(ctx, string(argsBytes))
	if err != nil {
		// En MCP es mejor devolver el error en el cuerpo como isError=true
		// para que el LLM lo vea y corrija su comportamiento.
		return CallToolResult{
			Content: []Content{{Type: "text", Text: err.Error()}},
			IsError: true,
		}, nil
	}

	return CallToolResult{
		Content: []Content{{Type: "text", Text: out}},
		IsError: false,
	}, nil
}
