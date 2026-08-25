# goulm-memory

Memoria persistente para agentes IA, como modulo Go independiente (MIT).

Capsulas de conocimiento con busqueda hibrida (BM25 + grafo + embeddings),
persistencia atomica multi-proceso y cero dependencias externas.

```go
store, _ := memory.NewStore(memory.Config{
    Dir:     filepath.Join(home, ".goulm-memory", "my-app"),
    Project: "my-app",
})

store.Remember(memory.RememberOptions{
    Key:      "auth-jwt",
    Category: memory.CategoryDecision,
    Content:  "Usar JWT para autenticacion",
    Tags:     []string{"auth", "seguridad"},
})

ranked, _ := store.Recall("autenticacion", &memory.Query{Limit: 5})
```

## Que es

Una libreria Go que almacena fragments de conocimiento (capsulas) con
metadatos (tags, TTL, prioridad, origen, grafo de enlaces) y los busca
con un pipeline hibrido: BM25 + expansion por grafo + fusion de rangos +
similitud vectorial (via embeddings).

Disenada para agentes IA que necesitan recordar decisiones, patrones,
bugs y conocimiento entre sesiones, en el contexto de un proyecto.

## Instalacion

```bash
go get github.com/LRGolden/goulm-memory
```

Requiere Go 1.26+. Sin dependencias de terceros.

## Uso basico

Ver [docs/QUICKSTART.md](docs/QUICKSTART.md) para guia paso a paso.

## Documentacion

| Documento | Para que |
|-----------|----------|
| [QUICKSTART.md](docs/QUICKSTART.md) | Primeros pasos: recordar, buscar, ver estado |
| [API.md](docs/API.md) | Referencia completa de la API |
| [ADVANCED.md](docs/ADVANCED.md) | Sessions, ledger, graph, embeddings, server, integracion |
| [EMBEDDINGS.md](docs/EMBEDDINGS.md) | Como integrar un provider de embeddings |
| [SERVER.md](docs/SERVER.md) | Server HTTP para clientes Python/TypeScript |
| [TOOLS.md](docs/TOOLS.md) | Tabla de tools registrables |
| [ARCHITECTURE.md](docs/ARCHITECTURE.md) | Arquitectura interna y diseno |
| [FORMATS.md](docs/FORMATS.md) | Formatos de persistencia (JSON y Ambar) |
| [VECTOR_SEARCH.md](docs/VECTOR_SEARCH.md) | Metodos de busqueda vectorial (VP-Tree) |
| [CHANGELOG.md](docs/CHANGELOG.md) | Historial de cambios |

## Estructura

```
pkg/memory/   # Store de memoria: capsulas, BM25, grafo, embeddings,
              # sessions, ledger, health, ambar, consolidacion, backups.
              # 100% stdlib, cero dependencias externas.
pkg/tools/    # Tools registrables (memory_*, context_brief, ledger_*)
              # + LedgerHook para observar ejecucion.
cmd/demo/     # CLI de demostracion (15 subcomandos).
cmd/serve/    # HTTP server para clientes multi-lenguaje.
docs/         # Documentacion.
```

## Performance

Metricas de Recall con busqueda hibrida (BM25 + grafo + embeddings),
medidas en `go test -benchmem`. Los valores de allocs/op y B/op son
independientes del hardware y representan el costo real por operacion.

| N capsules | allocs/op | B/op | Nota |
|------------|-----------|------|------|
| 10 | 281 | 40 KB | Default (Limit=6) |
| 100 | 2,468 | 416 KB | Default |
| 500 | 12,105 | 2.16 MB | Default |
| 1000 | 24,171 | 4.3 MB | Default |

**BuildGraph** con tags compartidos:

| N capsules | allocs/op | B/op | Nota |
|------------|-----------|------|------|
| 100 | 239 | 49 KB | Tags >50 capsules skip edges |
| 500 | 1,055 | 276 KB | Tags >50 capsules skip edges |

**BM25Scores** (busqueda textual):

| N capsules | allocs/op | B/op |
|------------|-----------|------|
| 100 | 618 | 96 KB |
| 1000 | 6,029 | 998 KB |

Para reproducir:
```bash
go test ./pkg/memory/ -run "^$" -bench "BenchmarkRecall" -benchmem
```

Ver [VECTOR_SEARCH.md](docs/VECTOR_SEARCH.md) para detalles del VP-Tree
y metodos de busqueda vectorial.

## Licencia

MIT — ver LICENSE. Copyright (c) 2026 LRGolden.
