# goulm-memory

Ecosistema de memoria persistente de Goulm, como módulo Go independiente (MIT).

Arranca como una copia del subsistema de memoria de
[Goulm](https://github.com/LRGolden/goulm): el store (`pkg/memory`, 100%
stdlib) y la capa de herramientas (`pkg/tools`) que lo exponen. A partir de
aquí evoluciona por separado.

## Qué ofrece

- **Store de memoria persistente** (`pkg/memory`): cápsulas (entradas de
  memoria) con categorías, origen, calidad, confianza, TTL y grafo de enlaces.
  Cero dependencias externas (solo stdlib).
- **Búsqueda híbrida**: BM25 + coincidencia de palabras clave + fusión de
  rangos (RRF) + expansión por grafo (ego-subgraph) + recencia/frecuencia.
- **Formato doble**: JSON legible o [Ámbar](docs/FORMATS.md) (texto plano
  orientado a líneas, diff-friendly).
- **Sesiones**: rastreador de sesiones concurrentes (heartbeats, conflictos de
  archivos, archivos tocados por sesión).
- **Ledger**: registro de actividad en JSON-lines con rotación/compactación,
  resumen diario/semanal/mensual y detección de raíz de proyecto.
- **Herramientas**: 13 tools (`memory_*`, `context_brief`, `ledger_*`)
  registrables en un `Registry` standalone, más un `LedgerHook` que observa la
  ejecución y registra sucesos.
- **Demo CLI** (`cmd/demo`): secuencia guionizada y subcomandos para probar el
  ecosistema de punta a punta.

## Estructura

```
pkg/memory/   # Store de memoria persistente: cápsulas, BM25, grafo, sessions,
              # ledger, reflog, health, ámbar, consolidación, backups.
              # Sin dependencias externas (solo stdlib).
pkg/tools/    # Capa de tools adaptada a un Registry standalone:
              #   11 tools memory_* + context_brief + ledger_tail/ledger_log
              #   + LedgerHook (observa ejecución y registra sucesos).
cmd/demo/     # CLI de demostración que cablea store + ledger + las 13 tools.
docs/         # Documentación: ARCHITECTURE.md, API.md, FORMATS.md, CHANGELOG.md.
```

## Requisitos

- Go 1.26 o superior (el módulo declara `go 1.26.5`).
- Sin dependencias de terceros: `go get` no es necesario.

## Uso del demo

```bash
cd goulm-memory
go run ./cmd/demo demo                 # secuencia guionizada
go run ./cmd/demo remember -key auth-jwt -category decision "Usar JWT para auth"
go run ./cmd/demo recall -q "autenticación"
go run ./cmd/demo stats --health
go run ./cmd/demo suggest -context "quiero autenticación"
go run ./cmd/demo brief
go run ./cmd/demo pin -key auth-jwt -priority 5
go run ./cmd/demo ledger-tail
go run ./cmd/demo tools                # lista las 13 tools registradas
```

Por defecto la memoria se guarda en `~/.goulm-memory/<Proyecto>`; usa
`-dir <ruta>` para cambiarlo. El ledger del demo se aísla dentro de ese
directorio (`<dir>/ledger`). Exit codes: 0 éxito, 1 error, 2 uso incorrecto.

Subcomandos: `demo`, `remember`, `recall`, `stats`, `suggest`, `brief`, `pin`,
`forget`, `resolve`, `backup`, `archive`, `consolidate`, `ledger-tail`,
`ledger-log`, `tools`, `help`. Ejecuta las tools vía `Registry` (misma ruta
que un agente real).

## Librería

### Store + herramientas

```go
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/LRGolden/goulm-memory/pkg/memory"
	"github.com/LRGolden/goulm-memory/pkg/tools"
)

func main() {
	cwd, _ := os.Getwd()
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".goulm-memory", memory.ProjectID(cwd))

	store, err := memory.NewStore(memory.Config{
		Dir:        dir,
		Format:     memory.FormatJSON,
		Project:    memory.ProjectID(cwd),
		MaxEntries: 100,
		MaxBackups: 10,
	})
	if err != nil {
		panic(err)
	}
	store.SetVocab(memory.ExtractProjectDeps(cwd))

	// Sesión opcional (para recall por archivos tocados).
	tracker, _ := store.Sessions("mi-agente")
	tracker.SetRoot("")

	// Registrar las 11 tools de memoria + 2 de ledger.
	reg := tools.NewRegistry()
	tools.RegisterMemoryTools(reg, store, tracker)
	if ledger, err := memory.NewLedger(cwd); err == nil {
		hook := tools.NewLedgerHook(ledger)
		defer hook.Close()
		tools.RegisterLedgerTools(reg, hook)
	}

	fmt.Println("tools registradas:", reg.Count())
}
```

### Uso directo del store

```go
// Recordar
res, _ := store.Remember(memory.RememberOptions{
	Key:      "auth-jwt",
	Category: memory.CategoryDecision,
	Content:  "Usar JWT para autenticación",
	Tags:     []string{"auth", "seguridad"},
	Origin:   memory.OriginHuman,
	Priority: 3,
})
fmt.Println("creada:", res.Created, "| fusionada:", res.Merged)

// Recuperar
ranked, _ := store.Recall("autenticación", &memory.Query{Limit: 5})
for _, r := range ranked {
	fmt.Printf("%.3f  %s\n", r.Score, r.Capsule.Key)
}

// Sugerencias sobre un contexto (sin consulta explícita)
sugs, _ := store.Suggest("estamos hablando de login", 3)

// Estado y mantenimiento
stats, _ := store.Stats()
backup, _ := store.Backup()          // backup/ <archivo>
rep, _ := store.Consolidate()        // merge de casi-duplicados
store.Flush()
```

Ver [`docs/API.md`](docs/API.md) para la referencia completa y
[`docs/FORMATS.md`](docs/FORMATS.md) para los formatos de los archivos.

## Herramientas registrables

| Tool | Descripción |
|------|-------------|
| `memory_remember` | Crea o fusiona una cápsula (key, category, content, tags, priority, ttl, origin, path_scope) |
| `memory_recall` | Búsqueda híbrida (q, category, tags, path_scope, graph, hops, rrf, limit) |
| `memory_suggest` | Sugiere cápsulas relevantes a un contexto |
| `memory_stats` | Estadísticas del store (+ `--health`) |
| `memory_forget` | Olvida por clave (soft `obsolete` con `SupersededOn` o `hard` = borrado) |
| `memory_resolve` | Restaura una cápsula soft-deleted (revierte memory_forget) |
| `memory_archive` | Archiva cápsulas por antigüedad (`older_than`: 24h/7d/30d) |
| `memory_pin` | Fija prioridad de una cápsula (0-5) |
| `memory_backup` | Copia el store a `backups/` |
| `memory_consolidate` | Merge de casi-duplicados por clave y Jaccard |
| `context_brief` | Resumen contextual del store (categorías, recientes, sugerencias) |
| `ledger_tail` | Últimos N eventos del ledger |
| `ledger_log` | Registra un suceso arbitrario (milestone) |

## Concurrencia

El store está protegido por mutex y por un **lock de archivo** (`memory.lock`):
los escritores bloquean el archivo durante persistencia; los lectores validan
la marca del archivo (`fileStamp`) para re-cargar si otro proceso lo cambió.
Lock stale tras 15 s, espera máxima 10 s. En Windows se manejan las
`sharing violation` como lock ocupado.

## Seguridad

- Archivos de memoria y ledger con permisos `0600`; directorios `0700`.
- Los directorios se crean solo si son escribibles; el ledger se deshabilita
  (con `Reason`) si no hay permisos.
- Escrituras atómicas (archivo temporal + `rename`).

## Licencia

MIT — ver `LICENSE`. Copyright (c) 2026 LRGolden.