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

## Installation

```bash
go get github.com/LRGolden/goulm-memory
```

## Quick Start

```go
package main

import (
    "fmt"
    "path/filepath"
    "os"
    "github.com/LRGolden/goulm-memory/pkg/memory"
)

func main() {
    home, _ := os.UserHomeDir()
    store, _ := memory.NewStore(memory.Config{
        Dir:     filepath.Join(home, ".goulm-memory", "my-app"),
        Project: "my-app",
    })
    defer store.Flush()

    // Store context
    store.Remember(memory.RememberOptions{
        Key:      "auth-jwt",
        Category: memory.CategoryDecision,
        Content:  "Use JWT for authentication with 24h expiry",
        Tags:     []string{"auth", "security"},
    })

    // Retrieve context
    ranked, _ := store.Recall("authentication", &memory.Query{Limit: 5})
    for _, r := range ranked {
        fmt.Printf("[%.2f] %s\n", r.Score, r.Capsule.Content)
    }
}
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
| [API.md](docs/API.md) | Complete public methods reference |
| [METHODOLOGY.md](docs/METHODOLOGY.md) | **Best Practices:** Context quality, chunking, and noise isolation |
| [ADVANCED.md](docs/ADVANCED.md) | Sessions, ledgers, and advanced topics |
| [MCP_INTEGRATION.md](docs/MCP_INTEGRATION.md) | Guide to connect with Cursor, Windsurf, Claude |
| [EMBEDDINGS.md](docs/EMBEDDINGS.md) | Guide to inject AI models (Vector Search) |
| [CHANGELOG.md](docs/CHANGELOG.md) | Release history and breaking changes |

> **Note:** Advanced uses and performance metrics are delegated to their respective markdown files. Spanish documentation is available in `docs/es/`.

## License

MIT — see [LICENSE](LICENSE). Copyright (c) 2026 LRGolden.
