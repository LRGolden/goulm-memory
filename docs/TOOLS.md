# Tools registrables

Tools que exponen las operaciones de memoria y ledger para integracion
con agentes IA. Cada tool recibe JSON de argumentos y devuelve texto.

## Tools de memoria (11)

| Tool | Parametros | Descripcion |
|------|-----------|-------------|
| `memory_remember` | `key`, `category`, `content`, `tags[]`, `priority`, `ttl`, `origin`, `path_scope` | Crea o fusiona una capsula. |
| `memory_recall` | `q`, `category`, `tags[]`, `path_scope`, `graph`, `hops`, `rrf`, `limit` | Busqueda hibrida. |
| `memory_suggest` | `context`, `limit` | Sugerencias sobre un contexto. |
| `memory_stats` | `format` (`json`/`text`), `health` (bool) | Estadisticas (+ health check). |
| `memory_forget` | `key`, `hard` (bool) | Olvida (soft/hard). Soft establece `SupersededOn`. |
| `memory_resolve` | `key` | Restaura una capsula soft-deleted. |
| `memory_archive` | `older_than` (`24h`/`7d`/`30d`) | Archiva por antiguedad. |
| `memory_pin` | `key`, `priority` | Fija prioridad (0-5). |
| `memory_backup` | — | Backup a `backups/`. |
| `memory_consolidate` | — | Merge de casi-duplicados por clave y Jaccard. |
| `context_brief` | `limit` | Resumen contextual (categorias, recientes, sugerencias). |

## Tools de ledger (2)

| Tool | Parametros | Descripcion |
|------|-----------|-------------|
| `ledger_tail` | `n`, `type`, `history` | Ultimos n eventos (por tipo, opcional historia). |
| `ledger_log` | `action`, `detail` | Registra un milestone arbitrario. |

## Registro

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

Observa la ejecucion de tools y registra sucesos en el ledger:

```go
hook := tools.NewLedgerHook(ledger)
hook.StartSession("mi-agente")
defer hook.EndSession()

// Interponer en un EventSink
sink := hook.Wrap(originalSink)
```

El writer es asincrono (cola interna). `Close()` drena y cierra.

## Mas informacion

- [ADVANCED.md](ADVANCED.md) — Integracion avanzada
- [API.md](API.md) — Referencia completa de `pkg/tools`
