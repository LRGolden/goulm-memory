# goulm-memory

[![Build Status](https://github.com/LRGolden/goulm-memory/actions/workflows/test.yml/badge.svg)](https://github.com/LRGolden/goulm-memory/actions)
[![Release](https://img.shields.io/github/v/release/LRGolden/goulm-memory)](https://github.com/LRGolden/goulm-memory/releases/latest)
[![GoDoc](https://pkg.go.dev/badge/github.com/LRGolden/goulm-memory)](https://pkg.go.dev/github.com/LRGolden/goulm-memory)
[![Go Version](https://img.shields.io/github/go-mod/go-version/LRGolden/goulm-memory)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**The embedded, deterministic memory engine for autonomous AI agents. Pure Go. Zero dependencies.**

Unlike memory solutions that use background LLMs to auto-summarize conversation history, `goulm-memory` is designed as an explicit knowledge vault. The agent decides exactly what to remember and recall using function calling.

### Design Philosophy
* **Explicit Storage:** No background LLM costs or automatic summarization noise. Agents invoke `memory_remember` explicitly for technical decisions, facts, or patterns.
* **Hybrid Retrieval (BM25 + Vectors + RRF):** Pure vector databases often fail to retrieve exact terms (like error codes or IP addresses). This engine implements a hybrid pipeline, using BM25 for lexical precision and VP-Trees for semantic similarity, merged via Reciprocal Rank Fusion (RRF).
* **Embedded & Zero Dependencies:** Compiles directly into your binary. No external databases (Postgres, Qdrant) or Docker containers required. Ideal for Local-First, Edge AI, and highly private environments.
* **Append-Only Ledger:** Memory state is saved locally as an immutable log, making agent decisions fully auditable.
* **MCP Support:** Ships with a standalone Model Context Protocol server (`cmd/mcp`) for instant integration with AI IDEs.

> **Note on Benchmarks:** While `goulm-memory`'s hybrid search pipeline has shown exceptional precision in our internal testing (particularly at isolating exact keyword matches vs semantic noise), we do not treat these internal results as publicly endorsed benchmarks. We welcome and encourage third-party benchmarking to formally validate these results against other memory architectures.

## 🚀 How to use (3 ways)

`goulm-memory` is designed as a universal tool. While it's written in Go, you don't need to write Go to use it. Choose the integration that fits your stack:

### 1. The No-Code Way: AI IDEs (Cursor, Windsurf, Claude)
Download the standalone **MCP binary** from the Releases tab and plug it directly into your AI assistant. It exposes all memory tools instantly via the Model Context Protocol.
```json
// Example Cursor mcp.json configuration
{
  "mcpServers": {
    "goulm-memory": {
      "command": "/path/to/goulm-memory-mcp",
      "args": ["-dir", "./vault", "-project", "my-app"]
    }
  }
}
```
*📖 See the full [MCP Integration Guide](docs/MCP_INTEGRATION.md).*

### 2. The Python/TS Way: REST API Server
Building agents in LangChain or LlamaIndex? Skip Docker and heavy databases. Download the **Serve binary** and run it locally. It acts as an ultra-lightweight memory microservice.
```bash
./goulm-memory-serve -addr :8080 -dir ./vault
```
```python
# Call it directly from Python or TypeScript
requests.post("http://localhost:8080/api/v1/remember", json={
    "key": "arch-decision", "category": "decision", "content": "Use Redis for caching."
})
```

### 3. The Go Way: Native Embedded Library
For absolute maximum performance and zero network latency, embed the engine directly into your Go backend.
```bash
go get github.com/LRGolden/goulm-memory
```
```go
import "github.com/LRGolden/goulm-memory/pkg/memory"

store, _ := memory.NewStore(memory.Config{Dir: "./vault", Project: "my-app"})
defer store.Flush()

store.Remember(memory.RememberOptions{
    Key: "fact-01", Category: memory.CategoryFact, Content: "Server IP is 10.0.0.5",
})
```

## Project Structure

```text
pkg/memory/   # Core engine: capsules, BM25, graphs, embeddings, Ambar. 100% stdlib.
pkg/tools/    # Registerable tools for agents (memory_*, context_brief).
cmd/serve/    # Optional HTTP server for Python/TypeScript clients.
cmd/demo/     # Interactive demo CLI.
cmd/mcp/      # Standard JSON-RPC stdio server for MCP (Model Context Protocol).
docs/         # Extended technical documentation.
```

## Documentation (docs/)

| Document | Content |
|----------|---------|
| [QUICKSTART.md](docs/QUICKSTART.md) | First steps to integrate the library |
| [PROMPTING.md](docs/PROMPTING.md) | **System Prompts:** How to instruct agents to use the memory vault |
| [API.md](docs/API.md) | Complete public methods reference |
| [METHODOLOGY.md](docs/METHODOLOGY.md) | **Best Practices:** Context quality, chunking, and noise isolation |
| [ADVANCED.md](docs/ADVANCED.md) | Sessions, ledgers, and advanced topics |
| [MCP_INTEGRATION.md](docs/MCP_INTEGRATION.md) | Guide to connect with Cursor, Windsurf, Claude |
| [EMBEDDINGS.md](docs/EMBEDDINGS.md) | Guide to inject AI models (Vector Search) |
| [CHANGELOG.md](docs/CHANGELOG.md) | Release history and breaking changes |

> **Note:** Advanced uses and performance metrics are delegated to their respective markdown files. Spanish documentation is available in `docs/es/`.

## License

MIT — see [LICENSE](LICENSE). Copyright (c) 2026 LRGolden.
