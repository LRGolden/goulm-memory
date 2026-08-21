package tools

// ToolCall representa una invocación de herramienta observada por el
// LedgerHook. Es el tipo mínimo necesario (sin dependencia del agente).
type ToolCall struct {
	ID        string
	Name      string
	Arguments string
}

// EventSink agrupa callbacks de observación opcionales (portado del agente
// de Goulm, recortado a los dos eventos que usa el ledger). Permiten a una UI
// recibir eventos tipados de la ejecución de tools sin acoplarse al store.
type EventSink struct {
	// OnToolStart se llama justo antes de ejecutar una herramienta.
	OnToolStart func(call *ToolCall)
	// OnToolResult se llama después de ejecutar una herramienta.
	// result es la salida de la herramienta; err su error (si hubo).
	OnToolResult func(call *ToolCall, result string, err error)
}
