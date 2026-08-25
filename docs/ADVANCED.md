# Advanced Usage

Guide for advanced integrations: concurrent sessions, activity ledger,
link graph and registerable tools.

## Sessions

The `SessionTracker` coordinates multiple instances of the same agent:

```go
tracker, err := store.Sessions("mi-agente")
tracker.SetRoot("") // use the cwd to detect git branch

// Periodic heartbeat (every ~60s)
tracker.Heartbeat("src/main.go", false)

// Files touched in the session
tracker.Touch("src/auth.go")
tracker.Touch("src/db.go")

// Query active sessions
sessions, _ := tracker.ActiveSessions()
conflicts, _ := tracker.Conflicts()
fmt.Println(memory.RenderSessions(sessions, conflicts, false))

// End
tracker.End()
```

### Multi-process coordination

- Heartbeat TTL: 10 minutes
- File lock with stale detection (15s)
- File conflicts between sessions detected by `Conflicts()`
- `SessionFiles()` returns the files touched by the current session,
  useful for session bias in `Query.SessionFiles`

## Ledger

Activity logging in JSON-lines with rotation and compaction:

```go
ledger, err := memory.NewLedger(cwd)
// Automatically disabled if no write permissions
// GOULM_LEDGER=off disables it from environment

// Log events
ledger.AppendTool("read_file", "src/main.go", "ok", "low", 250, session, false)
ledger.AppendEdit("edit", "src/main.go", "auth change", session, false)
ledger.AppendCommit("abc123", "feat: auth", "main", session)
ledger.AppendMilestone("v1.0 published", session)

// Query
events := ledger.Tail(10, "", false)
stats := ledger.Stats()
summary := ledger.Summary()

// Export and compact
export, _ := ledger.Export("2026-08-01", "2026-08-31")
ledger.CompactNow()
```

### Event format

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

### Event formatting

```go
// Short line (for tail)
fmt.Println(memory.FormatEvent(ev))

// Full line (with ISO date)
fmt.Println(memory.FormatEventFull(ev))
```

## Link graph

Builds a graph from explicit links, `[[wiki-style]]` references
and tag co-occurrence (>=2 shared tags):

```go
graph := memory.BuildGraph(capsules)

// Direct neighbors
neighbors := graph.Neighbors("auth-jwt")

// Ego-subgraph expansion (BFS)
dist := graph.EgoExpand([]string{"auth-jwt"}, 2, nil)

// Shortest path
path := graph.ShortestPath("auth-jwt", "db-schema")

// Centrality (simplified betweenness)
centrality := graph.Centrality()

// In searches: Graph=true, Hops=1 or 2
ranked, _ := store.Recall("auth", &memory.Query{
    Graph: true,
    Hops:  2,
    RRF:   true, // rank fusion
})
```

### LinkKey

Normalizes a link token to use as a graph node:

```go
memory.LinkKey("supersedes:engine-arch") // "engine-arch"
memory.LinkKey("engine-arch")            // "engine-arch"
```

## Tags and inference

```go
// Infer tags from content
tags := memory.InferTags("Use JWT for authentication", "auth-jwt", vocab)

// Extract project vocabulary (go.mod, package.json, requirements.txt)
vocab := memory.ExtractProjectDeps("/path/to/project")
store.SetVocab(vocab)
```

## Consolidation

Automatic merge of duplicates and near-duplicates:

```go
report, err := store.Consolidate()
// report.Merged = capsules merged by key
// report.NearDuplicates = near-duplicates by Jaccard
// report.Removed = exact duplicates removed
```

## Backup and maintenance

```go
// Backup with pruning
path, _ := store.Backup()

// Archive old capsules (>30 days)
archived, _ := store.ArchiveOld()

// Diff against a timestamp
diff, _ := store.Diff("2026-08-01")

// Health check
health, _ := store.Health(".")
fmt.Println(memory.RenderHealth(health))
```

## Amber Format

Alternative to JSON, line-oriented and diff-friendly:

```go
// Serialize
data := memory.MarshalAmbar("my-project", capsules)

// Deserialize
project, capsules, err := memory.UnmarshalAmbar(data)
```

## Agent integration

```go
// 1. Create store
store, _ := memory.NewStore(memory.Config{
    Dir:     dir,
    Project: projectID,
})
store.SetVocab(memory.ExtractProjectDeps(cwd))

// 2. Session (optional)
tracker, _ := store.Sessions("agente-1")
tracker.SetRoot("")

// 3. Ledger (optional)
ledger, _ := memory.NewLedger(cwd)
hook := tools.NewLedgerHook(ledger)
defer hook.Close()
hook.StartSession("agente-1")
defer hook.EndSession()

// 4. Register tools
reg := tools.NewRegistry()
tools.RegisterMemoryTools(reg, store, tracker)
tools.RegisterLedgerTools(reg, hook)

// 5. Execute tools from the agent
tool, ok := reg.Get("memory_recall")
if ok {
    result, err := tool.Execute(ctx, `{"q": "auth", "limit": 5}`)
    // ...
}
```

## Embeddings

Semantic search via embeddings. See [EMBEDDINGS.md](EMBEDDINGS.md) for a
complete guide with OpenAI and local model examples.

```go
// Configure provider
store.SetEmbedder(&MiProvider{apiKey: "..."})

// Pre-calc (recommended, avoids locks)
ctx := context.Background()
emb, _ := embedder.Embed(ctx, "text")
store.Remember(memory.RememberOptions{
    Key: "mi-clave", Category: memory.CategoryDecision,
    Content: "text", Embedding: emb,
})

// Automatic search (BM25 + vector)
ranked, _ := store.Recall("query", &memory.Query{Limit: 5})
```

Vector search uses an automatic VP-Tree (Vantage Point Tree)
when embeddings are available. The tree is cached and rebuilt on each mutation.
For N>1000, the VP-Tree reduces search time from O(N×D) to
O(log N×D). See [VECTOR_SEARCH.md](VECTOR_SEARCH.md) for details.

## HTTP Server

See [SERVER.md](SERVER.md) for the HTTP server that exposes the store via
JSON endpoints, useful for Python, TypeScript and other language clients.

## Behavior notes

### Flush and volatile metadata

Access bumps (`Accessed`, `LastAccessed`) are marked in memory but
only persisted when `Flush()` is called. If the process ends without
calling `Flush`, these changes are lost. This is intentional: recall
never fails due to disk errors.

To ensure persistence, call `Flush()` at the end of the turn or use
`defer store.Flush()`.

### One store per directory per process

Do not create two `MemoryStore` instances pointing to the same directory within
the same process. Each store has its own mutex; two stores = two independent
mutexes that will contend for the lockfile (timeout at 10s).

### TTL and expiration

Capsules with expired TTL do not appear in `Recall` but remain in the
store. `Consolidate` does not remove them. To clean up, use `Forget` manually
or configure a generous TTL.

### Sanitized fields

The `File` and `PathScope` fields are stored as-is. The consumer is
responsible for validating them if used to open files. Do not trust
these fields as safe paths.

## Further reading

- [API.md](API.md) — Complete reference of all types and functions
- [TOOLS.md](TOOLS.md) — Tools and parameters table
- [ARCHITECTURE.md](ARCHITECTURE.md) — Internal architecture
