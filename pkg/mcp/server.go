package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

// Server implementa un servidor JSON-RPC 2.0 sobre stdio para el protocolo MCP.
type Server struct {
	in       io.Reader
	out      io.Writer
	mu       sync.Mutex
	handlers map[string]func(context.Context, json.RawMessage) (interface{}, error)
}

// NewServer crea un nuevo servidor MCP.
func NewServer(in io.Reader, out io.Writer) *Server {
	return &Server{
		in:       in,
		out:      out,
		handlers: make(map[string]func(context.Context, json.RawMessage) (interface{}, error)),
	}
}

// Handle registra un manejador para un método JSON-RPC.
func (s *Server) Handle(method string, handler func(context.Context, json.RawMessage) (interface{}, error)) {
	s.handlers[method] = handler
}

// Run inicia el bucle principal de lectura y procesamiento.
func (s *Server) Run(ctx context.Context) error {
	scanner := bufio.NewScanner(s.in)
	// Los mensajes MCP por stdio no tienen límite estricto, pero pueden ser grandes
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			s.sendError(nil, ParseError, "Parse error: "+err.Error())
			continue
		}

		if req.JSONRPC != JSONRPCVersion {
			s.sendError(req.ID, InvalidRequest, "Invalid JSON-RPC version")
			continue
		}

		// Si no tiene ID, es una notificación (ej: notifications/initialized). Ignoramos.
		if req.ID == nil {
			continue
		}

		handler, ok := s.handlers[req.Method]
		if !ok {
			s.sendError(req.ID, MethodNotFound, fmt.Sprintf("Method not found: %s", req.Method))
			continue
		}

		result, err := handler(ctx, req.Params)
		if err != nil {
			s.sendError(req.ID, InternalError, err.Error())
			continue
		}

		s.sendResult(req.ID, result)
	}

	return scanner.Err()
}

func (s *Server) sendResult(id interface{}, result interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	resp := JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Result:  result,
	}
	enc := json.NewEncoder(s.out)
	_ = enc.Encode(resp)
}

func (s *Server) sendError(id interface{}, code int, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	resp := JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: message,
		},
	}
	enc := json.NewEncoder(s.out)
	_ = enc.Encode(resp)
}
