# Model Context Protocol (MCP) Integration

`goulm-memory` provides a native, zero-dependency implementation of the [Model Context Protocol (MCP)](https://modelcontextprotocol.io). This allows seamless integration with modern AI IDEs (such as Cursor, Windsurf, or Claude Desktop) natively exposing all memory tools.

## The `cmd/mcp` Server

The `cmd/mcp` package runs a strict JSON-RPC 2.0 server over standard input/output (`stdio`). This means the IDE orchestrates the process execution locally without requiring any network configuration or HTTP overhead.

## IDE Configuration

To enable `goulm-memory` in your IDE, point the MCP configuration to the compiled binary or run it directly via `go run`.

### Cursor / Windsurf (`mcp.json`)

Add the following to your IDE's MCP configuration:

```json
{
  "mcpServers": {
    "goulm-memory": {
      "command": "go",
      "args": [
        "run", 
        "./cmd/mcp", 
        "-dir", "/absolute/path/to/your/workspace", 
        "-project", "my-project-name"
      ]
    }
  }
}
```

### Pre-compiled Binary

If you prefer to distribute a single binary without Go installed on the host machine:

```bash
go build -o goulm-memory-mcp ./cmd/mcp
```

```json
{
  "mcpServers": {
    "goulm-memory": {
      "command": "/absolute/path/to/goulm-memory-mcp",
      "args": ["-dir", "/absolute/path/to/your/workspace"]
    }
  }
}
```

## Available Tools

Once connected, the MCP server automatically exposes the `tools.Registry`, injecting the following tools directly into the AI's context:

*   **`memory_remember`**: Store facts, bugs, decisions, or architectural notes.
*   **`memory_recall`**: Semantic search across the vault using the hybrid BM25+VPTree pipeline.
*   **`memory_stats`**: View repository health, broken files, and orphan links.
*   **`memory_forget` / `memory_resolve`**: Manage capsule lifecycles.
*   **`ledger_append`**: Maintain an immutable changelog (if the Ledger hook is used).

## Diagnostics & Troubleshooting

Since MCP uses `stdio` for protocol messages, **you must never print debug logs to `stdout`**. The `cmd/mcp` server strictly directs all standard logs and errors to `stderr`.

If your IDE fails to connect:
1. Ensure the `-dir` path is absolute and has read/write permissions.
2. Check the IDE's MCP Output/Console tab for `stderr` traces.
3. The server responds to the `"ping"` JSON-RPC method to prevent aggressive client timeouts.
