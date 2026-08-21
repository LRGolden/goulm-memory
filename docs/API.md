# Referencia de API

Referencia pública de `github.com/LRGolden/goulm-memory`. Módulo Go
independiente; requiere `go 1.26` o superior.

> Nota: si se trabaja dentro del repo de Goulm (que tiene un `go.work`),
> compilar/testear este módulo requiere `$env:GOWORK="off"`.

```go
import (
	"github.com/LRGolden/goulm-memory/pkg/memory"
	"github.com/LRGolden/goulm-memory/pkg/tools"
)
```

---

## `pkg/memory`

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

### `Capsule`

```go
type Capsule struct {
	ID           string   // único (hex, 4 bytes aleatorios)
	Category     Category
	Key          string   // clave corta y estable
	Content      string
	File         string   `json:"file,omitempty"`
	Tags         []string
	Date         string   // YYYY-MM-DD
	TTL          string   // "30d" relativo o "YYYY-MM-DD" absoluto
	Accessed     int      // contador de accesos
	Links        []string // claves enlazadas (grafo)
	Quality      float64  // [0,1]
	Confidence   float64  // [0,1]
	LastAccessed string   // ISO, opcional
	Priority     int      // 0-5
	PathScope    string   // glob de ámbito
	Origin       Origin
	Status       Status
	SupersededOn string   // fecha de soft-delete, opcional
}

func NewCapsule(cat Category, key, content string) (*Capsule, error)
func ValidCategory(c Category) bool
func ValidStatus(s Status) bool
func ValidOrigin(o Origin) bool
func ConfidenceFor(o Origin) float64   // human=0.95, agent=0.75, inferred=0.5
func NewID() string

func (c *Capsule) IsExpired(now time.Time) bool
func (c *Capsule) IsVisible(now time.Time, asOf string) bool
func (c *Capsule) FullText() string
func (c *Capsule) BumpAccess(now time.Time)
func (c *Capsule) Normalized() string
func (c *Capsule) Clone() *Capsule
func (c *Capsule) ApplyTTL(ttl string, now time.Time)
func ResolveTTL(ttl string, now time.Time) string
```

`NewCapsule` valida categoría, clave (`keyRE`) y contenido no vacío. El ID y
la fecha (`Date`) se generan automáticamente.

### `Config` / `MemoryStore`

```go
type Config struct {
	Dir        string // ~/.goulm/memory/<proyecto-id>
	Format     Format // json (default) | ambar
	Project    string // nombre declarado en los archivos
	MaxEntries int    // límite de cápsulas activas (default 100)
	MaxBackups int    // backups a conservar (default 10)
}

func NewStore(cfg Config) (*MemoryStore, error)
```

#### Escritura / ciclo de vida

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
	Created bool     // true = cápsula nueva
	Merged  bool     // true = se fusionó con una existente
}

func (s *MemoryStore) Remember(o RememberOptions) (RememberResult, error)
func (s *MemoryStore) Forget(key string, hard bool) (bool, error)
func (s *MemoryStore) Resolve(key string) (bool, error)
func (s *MemoryStore) Pin(key string, priority int) (bool, error)
func (s *MemoryStore) ArchiveOld() (int, error)
func (s *MemoryStore) Clear() (int, error)
func (s *MemoryStore) ImportCapsules(capsules []*Capsule) (int, error)
func (s *MemoryStore) ExportJSON() ([]byte, error)
func (s *MemoryStore) SetFormat(f Format) error
func (s *MemoryStore) Flush() error
```

- `Forget`: `hard=false` marca `obsolete` y registra `SupersededOn` (soft delete); `hard=true` elimina.
- `Resolve`: restaura una cápsula soft-deleted a `active` y limpia `SupersededOn` (revierte `memory_forget`).
- `Pin`: establece `Priority` (0-5); `0` quita el pin.
- `ImportCapsules`: importa y **fusiona** contra las claves existentes.

#### Consulta

```go
type Query struct {
	Text         string          // texto o intento a buscar
	Category     Category        // filtro opcional
	Tags         []string        // filtro AND: todos presentes
	FromDate     string          // YYYY-MM-DD
	ToDate       string          // YYYY-MM-DD
	PathScope    string          // glob
	AsOf         string          // vista temporal YYYY-MM-DD
	Limit        int             // default 6
	Graph        bool            // expandir ego-subgraph
	Hops         int             // 1 o 2 (default 1)
	RRF          bool            // fusión de rangos en vez de score lineal
	SessionFiles map[string]bool // archivos tocados por la sesión actual
}

type RankOptions struct {
	Query
	Now time.Time
}

type Ranked struct {
	Capsule *Capsule
	Score   float64
	IsSeed  bool   // coincidencia directa
	Dist    int    // 0 = seed; 1, 2 = vecino a N saltos
}

func (s *MemoryStore) Recall(query string, opts *Query) ([]Ranked, error)
func (s *MemoryStore) SmartRecall(intent string, limit int, sessionFiles ...map[string]bool) ([]Ranked, error)
func (s *MemoryStore) Suggest(context string, limit int, sessionFiles ...map[string]bool) ([]Ranked, error)
func (s *MemoryStore) Rank(opts RankOptions) ([]Ranked, error)
func (s *MemoryStore) ListActive(limit int) []*Capsule
```

- `Recall` equivale a `Rank` con `Now=time.Now()` (y `Limit` 6 si no se pasa
  en el `Query`).
- `SmartRecall` = `Rank` con `Graph:true, Hops:1` (intento semántico).
- `Suggest` = `Rank` sobre el contexto sin keywords estrictas.

#### Presupuestos de render

```go
type Budget string
const (
	BudgetTiny   Budget = "tiny"
	BudgetNormal Budget = "normal"
	BudgetDeep   Budget = "deep"
)
func Render(rs []Ranked, budget Budget) string
```

#### Metadatos / vocabulario

```go
func (s *MemoryStore) Dir() string
func (s *MemoryStore) Project() string
func (s *MemoryStore) Format() Format
func (s *MemoryStore) SetVocab(v map[string][]string) error
func (s *MemoryStore) Vocab() map[string][]string
func (s *MemoryStore) Sessions(agent string) (*SessionTracker, error)
```

### Scoring y grafo

```go
func QualityScore(c *Capsule, now time.Time) float64
func Importance(c *Capsule, now time.Time) float64

func BM25Scores(query string, docs []*Capsule) map[string]float64

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
const SessionTTL = 10 * time.Minute

type Heartbeat struct {
	ID        string            `json:"id"`
	Agent     string            `json:"agent"`
	PID       int               `json:"pid"`
	Branch    string            `json:"branch"`
	StartedAt string            `json:"started_at"`
	LastSeen  string            `json:"last_seen"`
	Files     map[string]string `json:"files"` // path -> ISO de último toque
	Ended     bool              `json:"ended"`
}

type ActiveSession struct {
	ID       string
	Agent    string
	Branch   string
	StartedAt string
	LastSeen time.Time
	Files    []string
	IsSelf   bool
	Ended    bool
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

El `SessionTracker` usa `GOULM_SESSION_ID` como ID de sesión si está definido;
si no, genera uno propio. `SetRoot("")` deja la rama al cwd; `SetRoot(ruta)`
usa el repo de `ruta` (`CurrentBranch`).

### Ledger

```go
type Ledger struct {
	Dir     string
	Root    string
	Project string
	Active  string
	Window  int
	Enabled bool
	Reason  string // por qué está deshabilitado (si aplica)
	Lock    string
}

type LedgerEvent struct {
	V          int    `json:"v"`
	TS         string `json:"ts"` // RFC3339
	Type       string `json:"type"`
	Action     string `json:"action,omitempty"`
	Session    string `json:"session,omitempty"`
	Path       string `json:"path,omitempty"`
	Detail     string `json:"detail,omitempty"`
	Hash       string `json:"hash,omitempty"`
	Risk       string `json:"risk,omitempty"`
	Status     string `json:"status,omitempty"`
	Approved   string `json:"approved,omitempty"`
	Tokens     int    `json:"tokens,omitempty"`
	CostUSD    float64 `json:"cost_usd,omitempty"`
	DurationMs int64  `json:"duration_ms,omitempty"`
	Turn       int    `json:"turn,omitempty"`
	Test       bool   `json:"test,omitempty"`
	ID         string `json:"id,omitempty"`
}
```

Constantes:

```go
const DefaultLedgerWindow = 200

const (
	EventApproval  = "approval"
	EventBranch    = "branch"
	EventCheckout  = "checkout"
	EventCommit    = "commit"
	EventEdit      = "edit"
	EventError     = "error"
	EventMemory    = "memory"
	EventMilestone = "milestone"
	EventSession   = "session"
	EventSystem    = "system"
	EventTest      = "test"
	EventTool      = "tool"
)

const (
	StatusOK      = "ok"
	StatusError   = "error"
	StatusDenied  = "denied"
	StatusBlocked = "blocked"
)

const (
	ApprovedYes = "yes"
	ApprovedNo  = "no"
	ApprovedNA  = "na"
)
```

```go
func NewLedger(cwd string, opts ...Option) (*Ledger, error)
func WithHome(dir string) Option   // aislar el ledger en dir
func WithWindow(n int) Option
func WithMaxDepth(d int) Option    // búsqueda de raíz (default 10)

func DetectRoot(cwd string, maxDepth int) string
func ProjectID(cwd string) string  // id de proyecto (ver gitutil)

func FormatEvent(ev LedgerEvent) string
func FormatEventFull(ev LedgerEvent) string

func (l *Ledger) Append(ev LedgerEvent) error
func (l *Ledger) AppendTool(action, path, status, risk string, durationMs int64, session string, isTest bool) error
func (l *Ledger) AppendEdit(action, path, detail, session string, isTest bool) error
func (l *Ledger) AppendCommit(hash, subject, branch, session string) error
func (l *Ledger) AppendError(action, detail, session string, isTest bool) error
func (l *Ledger) AppendMemory(action, key, category, session string) error
func (l *Ledger) AppendSessionStart(session string) error
func (l *Ledger) AppendSessionEnd(session string) error
func (l *Ledger) AppendMilestone(msg, session string) error
func (l *Ledger) AppendApproval(action, approved, session string, isTest bool) error
func (l *Ledger) Tail(n int, typ string, includeHistory bool) []LedgerEvent
func (l *Ledger) Stats() LedgerStats
func (l *Ledger) Export(since, to string) (string, error)
func (l *Ledger) Summary() string
func (l *Ledger) CompactNow() error
```

`NewLedger` se **deshabilita** (sin error) si no hay permisos de escritura:
`Enabled=false` con `Reason` explicativo. `GOULM_LEDGER=off` lo deshabilita
desde entorno.

```go
type LedgerStats struct {
	Enabled      bool
	Dir          string
	Project      string
	Total        int
	ActiveLines  int
	ArchiveFiles int
	ArchiveLines int
	ByType       map[string]int
}
```

### Reportes y mantenimiento

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

type DiffReport struct {
	Since   string      `json:"since"`
	New     []*Capsule  `json:"new"`
	Updated []*Capsule  `json:"updated"`
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

func (s *MemoryStore) Stats() (StatsView, error)
func (s *MemoryStore) Diff(since string) (DiffReport, error)
func (s *MemoryStore) Backup() (string, error)
func (s *MemoryStore) Primer(limit int) (string, error)
func (s *MemoryStore) Health(cwd string) (HealthReport, error)
func (s *MemoryStore) Consolidate() (ConsolidateReport, error)

func RenderStats(st StatsView) string
func RenderDiff(rep DiffReport) string
func RenderHealth(rep HealthReport) string

func MergeCapsules(existing, incoming *Capsule) *Capsule
func Jaccard(a, b string) float64
```

### Git / reflog / tags

```go
func CurrentBranch(repoDir string) string
func HasGitDir(repoDir string) bool
func ProjectID(cwd string) string

type ReflogEntry struct {
	Hash    string
	Subject string
	TS      string
}
func CurrentHead(repoDir string) string
func ReflogNew(repoDir, fromHash string) []ReflogEntry

func InferTags(content, key string, projectVocab map[string][]string) []string
func ExtractProjectDeps(dir string) map[string][]string
```

`ExtractProjectDeps` lee `go.mod`, `package.json` y `requirements.txt` para
construir el vocabulario del proyecto (p. ej. `{"golang": ["go", "go.mod"]}`).
`InferTags` combina vocabulario integrado + vocabulario del proyecto.

### Formato Ámbar

```go
func MarshalAmbar(project string, capsules []*Capsule) string
func UnmarshalAmbar(data string) (project string, capsules []*Capsule, err error)
```

Ver [`FORMATS.md`](FORMATS.md).

---

## `pkg/tools`

### Tipos

```go
type RiskLevel int
const (
	RiskLow      RiskLevel = iota + 1
	RiskMedium
	RiskHigh
	RiskCritical
)
func (r RiskLevel) String() string
func (r RiskLevel) Color() string

type ToolCategory string

type ToolMetadata struct {
	Name             string
	Description      string
	Category         ToolCategory
	RiskLevel        RiskLevel
	RequiresApproval bool
	Timeout          time.Duration
	Tags             []string
}

type Tool struct {
	Name             string
	Description      string
	Parameters       interface{}
	Execute          func(ctx context.Context, input string) (string, error)
	RequiresApproval bool
	IsReadOnly       bool
	Metadata         ToolMetadata
	RiskLevel        RiskLevel
	Category         ToolCategory
	Timeout          time.Duration
	Tags             []string
}

type ToolCall struct {
	ID        string
	Name      string
	Arguments string
}

type EventSink struct {
	OnToolStart  func(call *ToolCall)
	OnToolResult func(call *ToolCall, result string, err error)
}
```

### Registry

```go
func NewRegistry() *Registry
func (r *Registry) Register(tool Tool)
func (r *Registry) Get(name string) (Tool, bool)
func (r *Registry) List() []Tool
func (r *Registry) Names() []string
func (r *Registry) Count() int
```

`Register` aplica defaults si el campo viene a cero: timeout 30 s, categoría
`inspect`, riesgo `low` (o `high` si `RequiresApproval`).

### Registro de tools

```go
func RegisterMemoryTools(r *Registry, store *memory.MemoryStore, tracker *memory.SessionTracker)
func RegisterLedgerTools(r *Registry, hook *LedgerHook)
```

#### Tools de memoria (11)

Cada tool recibe `input` como **JSON de argumentos** y devuelve texto (o JSON).

| Tool | Parámetros (`input`) | Descripción |
|------|----------------------|-------------|
| `memory_remember` | `key`, `category`, `content`, `tags[]`, `priority`, `ttl`, `origin`, `path_scope` | Crea/fusiona una cápsula. |
| `memory_recall` | `q`, `category`, `tags[]`, `path_scope`, `graph`, `hops`, `rrf`, `limit` | Búsqueda híbrida. |
| `memory_suggest` | `context`, `limit` | Sugerencias sobre un contexto. |
| `memory_stats` | `format` (`json`/`text`), `health` (bool) | Estadísticas (+ health check). |
| `memory_forget` | `key`, `hard` (bool) | Olvida (soft/hard). |
| `memory_resolve` | `key` | Marca resuelta (soft delete). |
| `memory_archive` | `older_than` (`24h`/`7d`/`30d`) | Archiva por antigüedad. |
| `memory_pin` | `key`, `priority` | Fija prioridad (0-5). |
| `memory_backup` | — | Backup a `backups/`. |
| `memory_consolidate` | — | Merge de casi-duplicados. |
| `context_brief` | `limit` | Resumen contextual (categorías + recientes + sugerencias). |

#### Tools de ledger (2)

| Tool | Parámetros | Descripción |
|------|-----------|-------------|
| `ledger_tail` | `n`, `type`, `history` | Últimos n eventos (por tipo, opcional historia). |
| `ledger_log` | `action`, `detail` | Registra un milestone arbitrario. |

### LedgerHook

```go
func NewLedgerHook(ledger *memory.Ledger) *LedgerHook
func (h *LedgerHook) Ledger() *memory.Ledger
func (h *LedgerHook) StartSession(session string)
func (h *LedgerHook) EndSession()
func (h *LedgerHook) Milestone(msg string)
func (h *LedgerHook) OnToolStart(call *ToolCall)
func (h *LedgerHook) OnToolResult(call *ToolCall, result string, err error)
func (h *LedgerHook) Approval(call *ToolCall, approved, action string)
func (h *LedgerHook) Wrap(sink *EventSink) *EventSink
func (h *LedgerHook) Stats() (drops, writes int64)
func (h *LedgerHook) Close()
```

- El writer es **asíncrono** (cola interna); `Close()` drena y cierra.
- `Wrap` interpone el hook entre un `EventSink` externo y las ejecuciones.
- `Stats()` devuelve escrituras realizadas y drops por cola llena.
- Si el ledger está deshabilitado, el hook no registra nada y no bloquea.

---

## Demo (`cmd/demo`)

```bash
go run ./cmd/demo [subcomando] [-dir <ruta>]
```

- `demo`, `remember`, `recall`, `stats`, `suggest`, `brief`, `pin`, `forget`,
  `resolve`, `backup`, `archive`, `consolidate`, `ledger-tail`, `ledger-log`,
  `tools`, `help`.
- Por defecto: `~/.goulm-memory/<ProjectID>`; ledger aislado en
  `<dir>/ledger`.
- Exit codes: `0` éxito, `1` error, `2` uso incorrecto.