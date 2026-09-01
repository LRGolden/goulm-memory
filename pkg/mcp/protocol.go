package mcp

import (
	"encoding/json"
)

const (
	JSONRPCVersion = "2.0"
)

// JSONRPCRequest representa una petición genérica JSON-RPC 2.0.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"` // Puede ser int o string
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse representa una respuesta genérica JSON-RPC 2.0.
type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

// RPCError representa un error en JSON-RPC 2.0.
type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Códigos de error estándar JSON-RPC
const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
)

// ServerCapabilities define lo que soporta el servidor MCP.
type ServerCapabilities struct {
	Tools map[string]interface{} `json:"tools"`
}

// InitializeResult es el contenido de Result para el método "initialize".
type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      ServerInfo         `json:"serverInfo"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Tool representa una herramienta en el formato MCP.
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

// ListToolsResult es la respuesta para "tools/list".
type ListToolsResult struct {
	Tools []Tool `json:"tools"`
}

// CallToolParams son los parámetros enviados en "tools/call".
type CallToolParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// CallToolResult es la respuesta para "tools/call".
type CallToolResult struct {
	Content []Content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

// Content representa un bloque de contenido en MCP (texto, imagen, etc).
type Content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}
