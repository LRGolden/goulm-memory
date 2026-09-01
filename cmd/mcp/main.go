// Command mcp levanta un servidor stdio compatible con el Model Context Protocol.
//
// Uso en configuración del IDE (ej. Cursor/Windsurf):
//
//	{
//	  "mcpServers": {
//	    "goulm-memory": {
//	      "command": "goulm-memory-mcp", // o "go run ./cmd/mcp"
//	      "args": ["-dir", "/path/to/memory", "-project", "mi-proyecto"]
//	    }
//	  }
//	}
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/LRGolden/goulm-memory/pkg/mcp"
	"github.com/LRGolden/goulm-memory/pkg/memory"
	"github.com/LRGolden/goulm-memory/pkg/tools"
)

func main() {
	// IMPORTANTE: En stdio mode, logs y fmt.Print rompen el protocolo JSON-RPC.
	// Solo debemos escribir a stdout en formato JSON válido.
	// Errores graves se envían a stderr.

	dir := flag.String("dir", "", "Directorio base para la memoria y el ledger")
	project := flag.String("project", "default", "Nombre del proyecto/workspace")
	sessionID := flag.String("session", "mcp-client", "ID de sesión persistente")
	flag.Parse()

	if *dir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error obteniendo cwd: %v\n", err)
			os.Exit(1)
		}
		*dir = cwd
	}

	// 1. Inicializar Store
	store, err := memory.NewStore(memory.Config{
		Dir:     *dir,
		Project: *project,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error iniciando store: %v\n", err)
		os.Exit(1)
	}

	// 2. Inicializar Tracker de sesiones
	tracker, err := store.Sessions(*sessionID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error iniciando sesiones: %v\n", err)
		os.Exit(1)
	}
	tracker.SetRoot(*dir)
	tracker.Heartbeat("MCP Client Connected", false)

	// 3. Inicializar Ledger
	ledger, err := memory.NewLedger(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error iniciando ledger: %v\n", err)
		os.Exit(1)
	}
	hook := tools.NewLedgerHook(ledger)
	defer hook.Close()
	hook.StartSession(*sessionID)

	// 4. Registrar Tools
	reg := tools.NewRegistry()
	tools.RegisterMemoryTools(reg, store, tracker)
	tools.RegisterLedgerTools(reg, hook)

	// 5. Levantar el servidor MCP
	server := mcp.NewServer(os.Stdin, os.Stdout)
	adapter := mcp.NewAdapter(reg)
	adapter.RegisterHandlers(server)

	ctx := context.Background()
	if err := server.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Servidor MCP abortado: %v\n", err)
		os.Exit(1)
	}
}
