# Uso avanzado

Guia para integraciones avanzadas: sesiones concurrentes, ledger de actividad,
grafo de enlaces y tools registrables.

## Sesiones

El `SessionTracker` coordina multiples instancias del mismo agente:

```go
tracker, err := store.Sessions("mi-agente")
tracker.SetRoot("") // usa el cwd para detectar rama git

// Heartbeat periodico (cada ~60s)
tracker.Heartbeat("src/main.go", false)

// Archivos tocados en la sesion
tracker.Touch("src/auth.go")
tracker.Touch("src/db.go")

// Consultar sesiones activas
sessions, _ := tracker.ActiveSessions()
conflicts, _ := tracker.Conflicts()
fmt.Println(memory.RenderSessions(sessions, conflicts, false))

// Finalizar
tracker.End()
```

### Coordinacion multi-proceso

- Heartbeat TTL: 10 minutos
- Lock de archivo con stale detection (15s)
- Conflictos de archivos entre sesiones detectados por `Conflicts()`
- `SessionFiles()` devuelve los archivos tocados por la sesion actual,
  utiles para el sesgo de sesion en `Query.SessionFiles`

## Ledger

Registro de actividad en JSON-lines con rotacion y compactacion:

```go
ledger, err := memory.NewLedger(cwd)
// Se deshabilita automaticamente si no hay permisos de escritura
// GOULM_LEDGER=off lo deshabilita desde entorno

// Registrar eventos
ledger.AppendTool("read_file", "src/main.go", "ok", "low", 250, session, false)
ledger.AppendEdit("edit", "src/main.go", "cambio de auth", session, false)
ledger.AppendCommit("abc123", "feat: auth", "main", session)
ledger.AppendMilestone("v1.0 publicado", session)

// Consultar
events := ledger.Tail(10, "", false)
stats := ledger.Stats()
summary := ledger.Summary()

// Exportar y compactar
export, _ := ledger.Export("2026-08-01", "2026-08-31")
ledger.CompactNow()
```

### Formato de eventos

```go
type LedgerEvent struct {
    V          int      `json:"v"`
    TS         string   `json:"ts"`         // RFC3339
    Type       string   `json:"type"`       // tool, edit, commit, error, milestone, etc.
    Action     string   `json:"action,omitempty"`
    Session    string   `json:"session,omitempty"`
    Path       string   `json:"path,omitempty"`
    Detail     string   `json:"detail,omitempty"`
    Hash       string   `json:"hash,omitempty"`
    Risk       string   `json:"risk,omitempty"`
    Status     string   `json:"status,omitempty"`
    DurationMs int64    `json:"duration_ms,omitempty"`
}
```

### Formateo de eventos

```go
// Linea corta (para tail)
fmt.Println(memory.FormatEvent(ev))

// Linea completa (con fecha ISO)
fmt.Println(memory.FormatEventFull(ev))
```

## Grafo de enlaces

Construye un grafo a partir de links explicitos, referencias `[[wiki-style]]`
y co-ocurrencia de tags (>=2 tags compartidos):

```go
graph := memory.BuildGraph(capsules)

// Vecinos directos
neighbors := graph.Neighbors("auth-jwt")

// Expansion ego-subgraph (BFS)
dist := graph.EgoExpand([]string{"auth-jwt"}, 2, nil)

// Camino mas corto
path := graph.ShortestPath("auth-jwt", "db-schema")

// Centralidad (betweenness simplificada)
centrality := graph.Centrality()

// En busquedas: Graph=true, Hops=1 o 2
ranked, _ := store.Recall("auth", &memory.Query{
    Graph: true,
    Hops:  2,
    RRF:   true, // fusion de rangos
})
```

### LinkKey

Normaliza un token de link para usarlo como nodo del grafo:

```go
memory.LinkKey("supersedes:engine-arch") // "engine-arch"
memory.LinkKey("engine-arch")            // "engine-arch"
```

## Tags e inferencia

```go
// Inferir tags desde el contenido
tags := memory.InferTags("Usar JWT para autenticacion", "auth-jwt", vocab)

// Extraer vocabulario del proyecto (go.mod, package.json, requirements.txt)
vocab := memory.ExtractProjectDeps("/path/to/project")
store.SetVocab(vocab)
```

## Consolidacion

Merge automatico de duplicados y near-duplicates:

```go
report, err := store.Consolidate()
// report.Merged = capsulas fusionadas por clave
// report.NearDuplicates = near-duplicates por Jaccard
// report.Removed = duplicados exactos eliminados
```

## Backup y mantenimiento

```go
// Backup con poda
path, _ := store.Backup()

// Archivar capsulas viejas (>30 dias)
archived, _ := store.ArchiveOld()

// Diff contra un timestamp
diff, _ := store.Diff("2026-08-01")

// Health check
health, _ := store.Health(".")
fmt.Println(memory.RenderHealth(health))
```

## Formato Ambar

Alternativa a JSON, orientado a lineas y diff-friendly:

```go
// Serializar
data := memory.MarshalAmbar("my-project", capsules)

// Deserializar
project, capsules, err := memory.UnmarshalAmbar(data)
```

## Integracion con un agente

```go
// 1. Crear store
store, _ := memory.NewStore(memory.Config{
    Dir:     dir,
    Project: projectID,
})
store.SetVocab(memory.ExtractProjectDeps(cwd))

// 2. Sesion (opcional)
tracker, _ := store.Sessions("agente-1")
tracker.SetRoot("")

// 3. Ledger (opcional)
ledger, _ := memory.NewLedger(cwd)
hook := tools.NewLedgerHook(ledger)
defer hook.Close()
hook.StartSession("agente-1")
defer hook.EndSession()

// 4. Registrar tools
reg := tools.NewRegistry()
tools.RegisterMemoryTools(reg, store, tracker)
tools.RegisterLedgerTools(reg, hook)

// 5. Ejecutar tools desde el agente
tool, ok := reg.Get("memory_recall")
if ok {
    result, err := tool.Execute(ctx, `{"q": "auth", "limit": 5}`)
    // ...
}
```

## Embeddings

Busqueda semantica via embeddings. Ver [EMBEDDINGS.md](EMBEDDINGS.md) para
guia completa con ejemplos de OpenAI y modelos locales.

```go
// Configurar provider
store.SetEmbedder(&MiProvider{apiKey: "..."})

// Pre-calcul (recomendado, evita lock)
ctx := context.Background()
emb, _ := embedder.Embed(ctx, "texto")
store.Remember(memory.RememberOptions{
    Key: "mi-clave", Category: memory.CategoryDecision,
    Content: "texto", Embedding: emb,
})

// Busqueda automatica (BM25 + vector)
ranked, _ := store.Recall("query", &memory.Query{Limit: 5})
```

## Server HTTP

Ver [SERVER.md](SERVER.md) para el server HTTP que expone el store via
endpoints JSON, util para clientes Python, TypeScript y otros lenguajes.

## Notas de comportamiento

### Flush y metadata volatile

Los bumps de acceso (`Accessed`, `LastAccessed`) se marcan en memoria pero
solo se persisten cuando `Flush()` es llamado. Si el proceso termina sin
llamar `Flush`, estos cambios se pierden. Esto es intencional: el recall
nunca falla por errores de disco.

Para garantizar persistencia, llamar `Flush()` al final del turno o usar
`defer store.Flush()`.

### Un store por directorio por proceso

No crear dos `MemoryStore` apuntando al mismo directorio dentro de un
mismo proceso. Cada store tiene su propio mutex; dos stores = dos mutexes
independientes que contendran por el lockfile (timeout a 10s).

### TTL y expiracion

Las capsulas con TTL expirado no aparecen en `Recall` pero siguen en el
store. `Consolidate` no las elimina. Para limpiar, usar `Forget` manual
o configurar un TTL generoso.

### Campos sanitizados

Los campos `File` y `PathScope` se almacenan tal cual. El consumidor es
responsable de validarlos si los usa para abrir archivos. No confiar en
estos campos como rutas seguras.

## Mas informacion

- [API.md](API.md) — Referencia completa de todos los tipos y funciones
- [TOOLS.md](TOOLS.md) — Tabla de tools y parametros
- [ARCHITECTURE.md](ARCHITECTURE.md) — Arquitectura interna
