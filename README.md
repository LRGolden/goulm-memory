# goulm-memory

Memoria persistente para agentes IA, como modulo Go independiente (MIT).

Capsulas de conocimiento con busqueda hibrida (BM25 + grafo), persistencia
atomica multi-proceso y cero dependencias externas.

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
con un pipeline hibrido: BM25 + expansion por grafo + fusion de rangos.

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
| [ADVANCED.md](docs/ADVANCED.md) | Sessions, ledger, graph, tools, integracion avanzada |
| [TOOLS.md](docs/TOOLS.md) | Tabla de tools registrables |
| [ARCHITECTURE.md](docs/ARCHITECTURE.md) | Arquitectura interna y diseno |
| [FORMATS.md](docs/FORMATS.md) | Formatos de persistencia (JSON y Ambar) |
| [CHANGELOG.md](docs/CHANGELOG.md) | Historial de cambios |

## Estructura

```
pkg/memory/   # Store de memoria: capsulas, BM25, grafo, sessions,
              # ledger, health, ambar, consolidacion, backups.
              # 100% stdlib, cero dependencias externas.
pkg/tools/    # Tools registrables (memory_*, context_brief, ledger_*)
              # + LedgerHook para observar ejecucion.
cmd/demo/     # CLI de demostracion (15 subcomandos).
docs/         # Documentacion.
```

## Licencia

MIT — ver LICENSE. Copyright (c) 2026 LRGolden.
