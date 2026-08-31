# goulm-memory

[![GoDoc](https://pkg.go.dev/badge/github.com/LRGolden/goulm-memory)](https://pkg.go.dev/github.com/LRGolden/goulm-memory)
[![Go Version](https://img.shields.io/github/go-mod/go-version/LRGolden/goulm-memory)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Persistent memory for AI agents — a standalone Go module with zero dependencies.

Store knowledge capsules (decisions, patterns, bugs, facts) with metadata, search them using a hybrid pipeline (BM25 + graph + VP-Tree vector similarity), and persist them atomically to local disk. No databases, no servers.

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
docs/         # Extended technical documentation.
```

## Documentation (docs/)

| Document | Content |
|----------|---------|
| [QUICKSTART.md](docs/QUICKSTART.md) | First steps to integrate the library |
| [API.md](docs/API.md) | Complete public methods reference |
| [METHODOLOGY.md](docs/METHODOLOGY.md) | **Best Practices:** Context quality, chunking, and noise isolation |
| [ADVANCED.md](docs/ADVANCED.md) | Sessions, ledgers, and advanced topics |
| [EMBEDDINGS.md](docs/EMBEDDINGS.md) | Guide to inject AI models (Vector Search) |
| [CHANGELOG.md](docs/CHANGELOG.md) | Release history and breaking changes |

> **Note:** Advanced uses and performance metrics are delegated to their respective markdown files. Spanish documentation is available in `docs/es/`.

## License

MIT — see [LICENSE](LICENSE). Copyright (c) 2026 LRGolden.
