# Referencia de API

Referencia de `github.com/LRGolden/goulm-memory`. Requiere Go 1.26+.

```go
import "github.com/LRGolden/goulm-memory/pkg/memory"
```

---

## API esencial

Lo que necesitas para recordar, buscar y mantener capsulas.

### Tipos base

```go
type Category string
const (
    CategoryDecision  Category = "decision"
    CategoryPattern   Category = "pattern"
    CategoryBug       Category = "bug"
    CategoryKnowledge Category = "knowledge"
)

type Status string
const (
    StatusActive   Status = "active"
    StatusObsolete Status = "obsolete"
)

type Origin string
const (
    OriginHuman    Origin = "human"
    OriginAgent    Origin = "agent"
    OriginInferred Origin = "inferred"
)

type Format string
const (
    FormatJSON  Format = "json"
    FormatAmbar Format = "ambar"
)
```

### Capsule

```go
type Capsule struct {
    ID           string   `json:"id"`
    Category     Category `json:"category"`
    Key          string   `json:"key"`
    Content      string   `json:"content"`
    File         string   `json:"file,omitempty"`
    Tags         []string `json:"tags,omitempty"`
    Date         string   `json:"date"`
    TTL          string   `json:"ttl,omitempty"`
    Accessed     int      `json:"accessed"`
    Links        []string `json:"links,omitempty"`
    Quality      float64  `json:"quality"`
    Confidence   float64  `json:"confidence"`
    LastAccessed string   `json:"last_accessed,omitempty"`
    Priority     int      `json:"priority"`
    PathScope    string   `json:"path_scope,omitempty"`
    Origin       Origin   `json:"origin"`
    Status       Status   `json:"status"`
    SupersededOn string   `json:"superseded_on,omitempty"`
}

func NewCapsule(cat Category, key, content string) (*Capsule, error)
func NewID() string
func (c *Capsule) Clone() *Capsule
```

### Config / NewStore

```go
type Config struct {
    Dir        string // directorio de persistencia
    Format     Format // json (default) | ambar
    Project    string // nombre del proyecto
    MaxEntries int    // limite de capsulas activas (default 100)
    MaxBackups int    // backups a conservar (default 10)
}

func NewStore(cfg Config) (*MemoryStore, error)
```

### Escritura

```go
type RememberOptions struct {
    Key       string
    Category  Category
    Content   string
    Tags      []string
    Links     []string
    Origin    Origin
    Priority  int      // 0-5
    TTL       string
    PathScope string
    Verbatim  bool     // true: sin inferir tags ni recalcular calidad
}

type RememberResult struct {
    Capsule *Capsule
    Created bool
    Merged  bool
}

func (s *MemoryStore) Remember(o RememberOptions) (RememberResult, error)
func (s *MemoryStore) Forget(key string, hard bool) (bool, error)
func (s *MemoryStore) Resolve(key string) (bool, error)
func (s *MemoryStore) Pin(key string, priority int) (bool, error)
func (s *MemoryStore) Flush() error
```

- `Forget(key, false)`: soft delete, marca `obsolete` y registra `SupersededOn`.
- `Forget(key, true)`: hard delete, elimina permanentemente.
- `Resolve(key)`: restaura soft-deleted a `active` y limpia `SupersededOn`.

### Consulta

```go
type Query struct {
    Text         string
    Category     Category
    Tags         []string
    FromDate     string          // YYYY-MM-DD
    ToDate       string          // YYYY-MM-DD
    PathScope    string          // glob
    AsOf         string          // vista temporal YYYY-MM-DD
    Limit        int             // default 6
    Graph        bool            // expandir ego-subgraph
    Hops         int             // 1 o 2 (default 1)
    RRF          bool            // fusion de rangos
    SessionFiles map[string]bool // archivos tocados por la sesion
}

type Ranked struct {
    Capsule *Capsule
    Score   float64
    IsSeed  bool
    Dist    int
}

func (s *MemoryStore) Recall(query string, opts *Query) ([]Ranked, error)
func (s *MemoryStore) SmartRecall(intent string, limit int, sessionFiles ...map[string]bool) ([]Ranked, error)
func (s *MemoryStore) Suggest(context string, limit int, sessionFiles ...map[string]bool) ([]Ranked, error)
func (s *MemoryStore) ListActive(limit int) []*Capsule
```

### Render

```go
type Budget string
const (
    BudgetTiny   Budget = "tiny"
    BudgetNormal Budget = "normal"
    BudgetDeep   Budget = "deep"
)

func Render(rs []Ranked, budget Budget) string
```

### Estado

```go
type StatsView struct {
    Total       int            `json:"total"`
    ByCategory  map[string]int `json:"by_category"`
    ByOrigin    map[string]int `json:"by_origin"`
    ByStatus    map[string]int `json:"by_status"`
    Archived    int            `json:"archived"`
    Expired     int            `json:"expired"`
    Pinned      int            `json:"pinned"`
    AvgQuality  float64        `json:"avg_quality"`
    FileSizeKB  float64        `json:"file_size_kb"`
    LastUpdated string         `json:"last_updated"`
}

func (s *MemoryStore) Stats() (StatsView, error)
func RenderStats(st StatsView) string
```

### Metadatos

```go
func (s *MemoryStore) Dir() string
func (s *MemoryStore) Project() string
func (s *MemoryStore) Format() Format
func (s *MemoryStore) SetVocab(v map[string][]string) error
func (s *MemoryStore) Vocab() map[string][]string
```

---

## API completa

Funciones adicionales para uso avanzado. Ver [ADVANCED.md](ADVANCED.md)
para guia de integracion.

### Escritura extendida

```go
func (s *MemoryStore) ArchiveOld() (int, error)
func (s *MemoryStore) Clear() (int, error)
func (s *MemoryStore) ImportCapsules(capsules []*Capsule) (int, error)
func (s *MemoryStore) ExportJSON() ([]byte, error)
func (s *MemoryStore) SetFormat(f Format) error
```

### Consulta avanzada

```go
type RankOptions struct {
    Query
    Now time.Time
}

func (s *MemoryStore) Rank(opts RankOptions) ([]Ranked, error)
```

### Scoring

```go
func QualityScore(c *Capsule, now time.Time) float64
func Importance(c *Capsule, now time.Time) float64
func BM25Scores(query string, docs []*Capsule) map[string]float64
```

### Grafo

```go
type Graph struct { /* interno */ }
func BuildGraph(capsules []*Capsule) *Graph
func LinkKey(token string) string
func (g *Graph) Neighbors(key string) []string
func (g *Graph) HasKey(key string) bool
func (g *Graph) Node(key string) *Capsule
func (g *Graph) Degree(key string) int
func (g *Graph) Centrality() map[string]float64
func (g *Graph) EgoExpand(seeds []string, hops int, visible func(*Capsule) bool) map[string]int
func (g *Graph) ShortestPath(a, b string) []string
```

### Sesiones

```go
func (s *MemoryStore) Sessions(agent string) (*SessionTracker, error)

type ActiveSession struct {
    ID        string
    Agent     string
    Branch    string
    StartedAt string
    LastSeen  time.Time
    Files     []string
    IsSelf    bool
    Ended     bool
}

type FileConflict struct {
    File     string
    Sessions []string
    Conflict bool
}

func NewSessionTracker(dir, agent string) (*SessionTracker, error)
func (t *SessionTracker) SetRoot(root string)
func (t *SessionTracker) SelfID() string
func (t *SessionTracker) Touch(file string) error
func (t *SessionTracker) Heartbeat(file string, ended bool) error
func (t *SessionTracker) End() error
func (t *SessionTracker) ActiveSessions() ([]ActiveSession, error)
func (t *SessionTracker) Conflicts() ([]FileConflict, error)
func (t *SessionTracker) Prune() (int, error)
func (t *SessionTracker) SessionFiles() map[string]bool
func RenderSessions(sessions []ActiveSession, conflicts []FileConflict, conflictsOnly bool) string
```

### Ledger

```go
func NewLedger(cwd string, opts ...Option) (*Ledger, error)

type LedgerEvent struct {
    V          int     `json:"v"`
    TS         string  `json:"ts"`
    Type       string  `json:"type"`
    Action     string  `json:"action,omitempty"`
    Session    string  `json:"session,omitempty"`
    Path       string  `json:"path,omitempty"`
    Detail     string  `json:"detail,omitempty"`
    Hash       string  `json:"hash,omitempty"`
    Risk       string  `json:"risk,omitempty"`
    Status     string  `json:"status,omitempty"`
    DurationMs int64   `json:"duration_ms,omitempty"`
}

func FormatEvent(ev LedgerEvent) string
func FormatEventFull(ev LedgerEvent) string

func (l *Ledger) Append(ev LedgerEvent) error
func (l *Ledger) AppendTool(action, path, status, risk string, durationMs int64, session string, isTest bool) error
func (l *Ledger) AppendEdit(action, path, detail, session string, isTest bool) error
func (l *Ledger) AppendCommit(hash, subject, branch, session string) error
func (l *Ledger) AppendError(action, detail, session string, isTest bool) error
func (l *Ledger) AppendMilestone(msg, session string) error
func (l *Ledger) Tail(n int, typ string, includeHistory bool) []LedgerEvent
func (l *Ledger) Stats() LedgerStats
func (l *Ledger) Summary() string
func (l *Ledger) CompactNow() error
```

### Reportes y mantenimiento

```go
type DiffReport struct {
    Since   string     `json:"since"`
    New     []*Capsule `json:"new"`
    Updated []*Capsule `json:"updated"`
}

type ConsolidateReport struct {
    Before         int `json:"before"`
    After          int `json:"after"`
    Merged         int `json:"merged"`
    NearDuplicates int `json:"near_duplicates"`
    Removed        int `json:"removed"`
}

type HealthReport struct {
    Score           int      `json:"score"`
    Entries         int      `json:"entries"`
    AvgQuality      float64  `json:"avg_quality"`
    OrphanLinks     []string `json:"orphan_links"`
    ExactDuplicates int      `json:"exact_duplicates"`
    ExpiredTTL      []string `json:"expired_ttl"`
    BrokenFiles     []string `json:"broken_files"`
    MissingEvidence []string `json:"missing_evidence"`
    StaleClaims     []string `json:"stale_claims"`
    Secrets         []string `json:"secrets"`
    Warnings        int      `json:"warnings"`
}

func (s *MemoryStore) Diff(since string) (DiffReport, error)
func (s *MemoryStore) Backup() (string, error)
func (s *MemoryStore) Primer(limit int) (string, error)
func (s *MemoryStore) Health(cwd string) (HealthReport, error)
func (s *MemoryStore) Consolidate() (ConsolidateReport, error)

func RenderDiff(rep DiffReport) string
func RenderHealth(rep HealthReport) string
func MergeCapsules(existing, incoming *Capsule) *Capsule
func Jaccard(a, b string) float64
```

### Git / tags

```go
func CurrentBranch(repoDir string) string
func HasGitDir(repoDir string) bool
func ProjectID(cwd string) string
func InferTags(content, key string, projectVocab map[string][]string) []string
func ExtractProjectDeps(dir string) map[string][]string
```

### Formato Ambar

```go
func MarshalAmbar(project string, capsules []*Capsule) string
func UnmarshalAmbar(data string) (project string, capsules []*Capsule, err error)
```

Ver [FORMATS.md](FORMATS.md).

---

## pkg/tools

Ver [TOOLS.md](TOOLS.md) para tabla de tools y [ADVANCED.md](ADVANCED.md)
para guia de integracion.

### Registry

```go
func NewRegistry() *Registry
func (r *Registry) Register(tool Tool)
func (r *Registry) Get(name string) (Tool, bool)
func (r *Registry) List() []Tool
func (r *Registry) Names() []string
func (r *Registry) Count() int
```

### Registro de tools

```go
func RegisterMemoryTools(r *Registry, store *memory.MemoryStore, tracker *memory.SessionTracker)
func RegisterLedgerTools(r *Registry, hook *LedgerHook)
```

### LedgerHook

```go
func NewLedgerHook(ledger *memory.Ledger) *LedgerHook
func (h *LedgerHook) StartSession(session string)
func (h *LedgerHook) EndSession()
func (h *LedgerHook) Milestone(msg string)
func (h *LedgerHook) Wrap(sink *EventSink) *EventSink
func (h *LedgerHook) Stats() (drops, writes int64)
func (h *LedgerHook) Close()
```

---

## Demo (cmd/demo)

```bash
go run ./cmd/demo [subcomando] [-dir <ruta>]
```

Subcomandos: `demo`, `remember`, `recall`, `stats`, `suggest`, `brief`, `pin`,
`forget`, `resolve`, `backup`, `archive`, `consolidate`, `ledger-tail`,
`ledger-log`, `tools`, `help`.
