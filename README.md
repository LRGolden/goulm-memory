# goulm-memory

[![GoDoc](https://pkg.go.dev/badge/github.com/LRGolden/goulm-memory)](https://pkg.go.dev/github.com/LRGolden/goulm-memory)
[![Go Version](https://img.shields.io/github/go-mod/go-version/LRGolden/goulm-memory)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Persistent memory for AI agents — a standalone Go module with zero dependencies.

Store knowledge capsules (decisions, patterns, bugs, facts) with metadata, search them with a hybrid pipeline (BM25 + graph + embeddings), and persist atomically across processes. No database, no server, no external dependencies.

```go
store, _ := memory.NewStore(memory.Config{
    Dir:     filepath.Join(home, ".goulm-memory", "my-app"),
    Project: "my-app",
})

store.Remember(memory.RememberOptions{
    Key:      "auth-jwt",
    Category: memory.CategoryDecision,
    Content:  "Use JWT for authentication",
    Tags:     []string{"auth", "security"},
})

ranked, _ := store.Recall("authentication", &memory.Query{Limit: 5})
```

## Features

- **Zero dependencies** — 100% Go stdlib, no Qdrant, Neo4j, or Postgres
- **Hybrid search** — BM25 text + graph expansion + VP-Tree vector similarity
- **Multi-process safe** — atomic file persistence with advisory locks
- **Temporal** — TTL, priority decay, supersession timestamps
- **Graph** — tag-based edges, wiki-style links, ego expansion, centrality
- **Embeddings** — pluggable provider interface, automatic VP-Tree indexing
- **HTTP server** — expose the store via JSON endpoints for Python/TypeScript clients
- **Ledger** — JSON-lines audit trail of all agent operations
- **Ambar format** — human-readable, diff-friendly persistence alternative
- **Simple** — `go get` and you're done, no infrastructure to set up

## Quick Start

```bash
go get github.com/LRGolden/goulm-memory
```

```go
package main

import (
    "fmt"
    "path/filepath"

    "github.com/LRGolden/goulm-memory/pkg/memory"
)

func main() {
    home, _ := os.UserHomeDir()
    store, err := memory.NewStore(memory.Config{
        Dir:     filepath.Join(home, ".goulm-memory", "my-app"),
        Project: "my-app",
    })
    if err != nil {
        log.Fatal(err)
    }
    defer store.Flush()

    // Store a decision
    store.Remember(memory.RememberOptions{
        Key:      "auth-jwt",
        Category: memory.CategoryDecision,
        Content:  "Use JWT for authentication with 24h expiry",
        Tags:     []string{"auth", "security"},
    })

    // Search it back
    ranked, _ := store.Recall("authentication", &memory.Query{Limit: 5})
    for _, r := range ranked {
        fmt.Printf("[%.2f] %s: %s\n", r.Score, r.Capsule.Key, r.Capsule.Content)
    }
}
```

See [docs/QUICKSTART.md](docs/QUICKSTART.md) for a complete step-by-step guide.

## Why goulm-memory?

| Need | goulm-memory | Traditional solutions |
|------|-------------|----------------------|
| No infrastructure | File-based, zero setup | Needs Qdrant/Neo4j/Postgres |
| Zero dependencies | 100% Go stdlib | 8-50+ packages |
| Go native | First-class Go module | Python-first, Go afterthought |
| Multi-process | Atomic writes with locks | Via server/DB |
| No LLM required | BM25 + graph search | Fact extraction with LLM |
| No cost | Free forever | $249+/mo (managed services) |

## How It Works

goulm-memory stores knowledge as **capsules** — structured JSON entries with content, metadata, tags, and links. When you search, a hybrid pipeline combines multiple signals:

1. **BM25** — full-text search with TF-IDF scoring
2. **Graph expansion** — ego-subgraph traversal via tag-based edges
3. **VP-Tree** — approximate nearest-neighbor vector search (when embeddings are configured)
4. **RRF fusion** — combines all scores into a single ranking

The store persists to disk as JSON or Ambar files with atomic writes and advisory file locks, making it safe for multiple processes accessing the same directory.

```
pkg/memory/   # Core: capsules, BM25, graph, embeddings, sessions,
              # ledger, health, Ambar, consolidation, backups.
              # 100% stdlib, zero external dependencies.
pkg/tools/    # Registerable tools (memory_*, context_brief, ledger_*)
              # + LedgerHook for execution observability.
cmd/demo/     # Demo CLI (15 subcommands).
cmd/serve/    # HTTP server for multi-language clients.
docs/         # Documentation.
```

## Performance

Metrics for hybrid search (BM25 + graph + embeddings), measured with `go test -benchmem`. The `allocs/op` and `B/op` values are hardware-independent and represent the real cost per operation.

| N capsules | allocs/op | B/op | Note |
|------------|-----------|------|------|
| 10 | 281 | 40 KB | Default (Limit=6) |
| 100 | 2,468 | 416 KB | Default |
| 500 | 12,105 | 2.16 MB | Default |
| 1000 | 24,171 | 4.3 MB | Default |

**BuildGraph** with shared tags:

| N capsules | allocs/op | B/op | Note |
|------------|-----------|------|------|
| 100 | 239 | 49 KB | Tags >50 capsules skip edges |
| 500 | 1,055 | 276 KB | Tags >50 capsules skip edges |

**BM25Scores** (text search):

| N capsules | allocs/op | B/op |
|------------|-----------|------|
| 100 | 618 | 96 KB |
| 1000 | 6,029 | 998 KB |

To reproduce:

```bash
go test ./pkg/memory/ -run "^$" -bench "BenchmarkRecall" -benchmem
```

See [docs/VECTOR_SEARCH.md](docs/VECTOR_SEARCH.md) for VP-Tree details and vector search methods.

## Documentation

**Getting started:**

| Document | What it's for |
|----------|--------------|
| [QUICKSTART.md](docs/QUICKSTART.md) | First steps: store, search, status |
| [API.md](docs/API.md) | Complete API reference |
| [ADVANCED.md](docs/ADVANCED.md) | Sessions, ledger, graph, embeddings, server |

**Deep dives:**

| Document | What it's for |
|----------|--------------|
| [EMBEDDINGS.md](docs/EMBEDDINGS.md) | How to integrate an embedding provider |
| [SERVER.md](docs/SERVER.md) | HTTP server for Python/TypeScript clients |
| [TOOLS.md](docs/TOOLS.md) | Registerable tools table |
| [ARCHITECTURE.md](docs/ARCHITECTURE.md) | Internal architecture and design |
| [FORMATS.md](docs/FORMATS.md) | Persistence formats (JSON and Ambar) |
| [VECTOR_SEARCH.md](docs/VECTOR_SEARCH.md) | Vector search methods (VP-Tree) |
| [CHANGELOG.md](docs/CHANGELOG.md) | Release history |

**En español:** La documentación también está disponible en [docs/es/](docs/es/).

## Contributing

Contributions are welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

```bash
# Clone and run tests
git clone https://github.com/LRGolden/goulm-memory.git
cd goulm-memory
go test ./pkg/memory/ -v
```

## License

MIT — see [LICENSE](LICENSE). Copyright (c) 2026 LRGolden.
