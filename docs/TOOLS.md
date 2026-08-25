# Registrable Tools

Tools that expose memory and ledger operations for integration
with AI agents. Each tool receives JSON arguments and returns text.

## Memory Tools (11)

| Tool | Parameters | Description |
|------|-----------|-------------|
| `memory_remember` | `key`, `category`, `content`, `tags[]`, `priority`, `ttl`, `origin`, `path_scope` | Creates or merges a capsule. |
| `memory_recall` | `q`, `category`, `tags[]`, `path_scope`, `graph`, `hops`, `rrf`, `limit` | Hybrid search. |
| `memory_suggest` | `context`, `limit` | Suggestions on a context. |
| `memory_stats` | `format` (`json`/`text`), `health` (bool) | Statistics (+ health check). |
| `memory_forget` | `key`, `hard` (bool) | Forgets (soft/hard). Soft sets `SupersededOn`. |
| `memory_resolve` | `key` | Restores a soft-deleted capsule. |
| `memory_archive` | `older_than` (`24h`/`7d`/`30d`) | Archives by age. |
| `memory_pin` | `key`, `priority` | Sets priority (0-5). |
| `memory_backup` | — | Backup to `backups/`. |
| `memory_consolidate` | — | Merges near-duplicates by key and Jaccard. |
| `context_brief` | `limit` | Contextual summary (categories, recent, suggestions). |

## Ledger Tools (2)

| Tool | Parameters | Description |
|------|-----------|-------------|
| `ledger_tail` | `n`, `type`, `history` | Last n events (by type, optional history). |
| `ledger_log` | `action`, `detail` | Records an arbitrary milestone. |

## Registration

```go
import (
    "github.com/LRGolden/goulm-memory/pkg/memory"
    "github.com/LRGolden/goulm-memory/pkg/tools"
)

reg := tools.NewRegistry()
tools.RegisterMemoryTools(reg, store, tracker)

if ledger, err := memory.NewLedger(cwd); err == nil {
    hook := tools.NewLedgerHook(ledger)
    defer hook.Close()
    tools.RegisterLedgerTools(reg, hook)
}

fmt.Println("tools:", reg.Count()) // 13
```

## LedgerHook

Observes tool execution and records events in the ledger:

```go
hook := tools.NewLedgerHook(ledger)
hook.StartSession("mi-agente")
defer hook.EndSession()

// Intercept into an EventSink
sink := hook.Wrap(originalSink)
```

The writer is asynchronous (internal queue). `Close()` drains and closes.

## More Information

- [ADVANCED.md](ADVANCED.md) — Advanced integration
- [API.md](API.md) — Complete reference for `pkg/tools`
