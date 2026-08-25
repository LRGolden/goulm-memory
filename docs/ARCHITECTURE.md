# Architecture

Overview of how `goulm-memory` is built, what components make it up, and
how data flows. For the API reference, see
[`API.md`](API.md); for file formats, [`FORMATS.md`](FORMATS.md).

## Package diagram

```
 cmd/demo/        Demo CLI
    │  wires store + tracker + ledger + Registry + LedgerHook
    ▼
 pkg/tools/       Tool layer adapted to a standalone Registry
    │  memory_tools.go (11 tools)  ledger_tools.go (2 tools)
    │  ledger_hook.go  tool.go  types.go  events.go
    ▼
 pkg/memory/      Core of the store (100% stdlib, no dependencies)
    ├─ store.go        MemoryStore: load/persistence, lock, atomic-write
    ├─ memory.go       Remember / Recall / SmartRecall / Suggest / ListActive
    ├─ capsule.go      Capsule model + validation + TTL + visibility
    ├─ ranking.go      Search pipeline: BM25, match, graph, RRF, Render
    ├─ scoring.go      QualityScore / Importance
    ├─ graph.go        Link graph: BuildGraph, Centrality, EgoExpand
    ├─ sessions.go     SessionTracker: heartbeats, conflicts, files per session
    ├─ ledger.go       Ledger: JSON-lines registry + rotation + summary
    ├─ rotate.go       Compaction of active ledger into archives/
    ├─ summary.go      Ledger aggregation by day/week/month
    ├─ reflog.go       Git reflog reading (to detect branch changes)
    ├─ gitutil.go      CurrentBranch, HasGitDir, ProjectID
    ├─ primer.go       Primer, Stats, Diff, Backup + renderers
    ├─ merge.go        Consolidate / MergeCapsules / Jaccard
    ├─ health.go       Health(cwd) + RenderHealth
    ├─ tags.go         InferTags + ExtractProjectDeps (vocabulary)
    ├─ ambar.go        Amber format (plain text) + Format
    ├─ scoring.go / ranking.go / ... (details below)
    └─ pidalive_*.go   Live PID detection (Windows/Unix)
```

## The capsule (memory unit)

All memory consists of **capsules** (`Capsule`), with the following fields:

| Field | Description |
|-------|-------------|
| `ID` | Unique identifier (4 random bytes, hex). |
| `Category` | `decision`, `pattern`, `bug`, `knowledge`. |
| `Key` | Short, stable key for identification/collision. |
| `Content` | Body of the memory (text). |
| `File` | Associated project file (optional). |
| `Tags` | Labels (AND filter search). |
| `Date` | Creation date `YYYY-MM-DD` (ISO). |
| `TTL` | Expiration: `30d` (relative) or `YYYY-MM-DD` (absolute). |
| `Accessed` | Access counter (for recency/frequency). |
| `Links` | Linked keys (graph). |
| `Quality` | Calculated quality [0-1] (see scoring). |
| `Confidence` | Confidence based on origin. |
| `LastAccessed` | Last access ISO (if any). |
| `Priority` | 0-5; 1-5 boosts the capsule above pure BM25. |
| `PathScope` | Path scope glob (session filter). |
| `Origin` | `human`, `agent`, `inferred`. |
| `Status` | `active`, `obsolete`. |
| `SupersededOn` | Date when it was superseded (soft-delete). |

**Origin → confidence** (`ConfidenceFor`): `human` = 1.0, `agent` = 0.8,
`inferred` = 0.6.

**Lifecycle**:

1. `Remember` creates a new capsule or **merges** with an existing one of the
   same key (see `MergeCapsules` in merge.go).
2. The capsule is visible while `Status == active`, not expired
   (`TTL`) and, in queries with a temporal view, its `Date` does not exceed `asOf`.
3. `Forget(key, hard=false)` marks it `obsolete` (soft); `hard=true` removes
   it from the store.
4. `Resolve(key)` restores it to `active` and clears `SupersededOn` (reverts `Forget`).
5. `ArchiveOld` moves to `archive` those that exceed an age threshold.
6. Accessing a capsule via `Rank`/`Recall` increments `Accessed` and
   refreshes `LastAccessed` (bump); pending bumps are persisted with
   `Flush` or on the next write.

## Persistence and concurrency

- `NewStore(Config)` opens (or creates) the directory with the structure of the
  `FileSet` table: `memory.<ext>`, `archive.<ext>`, `config.json`,
  `memory.lock`, `backups/`, `sessions/`.
- Loads into memory: active capsules in `s.entries` (indexed by ID and by
  key) and archived in `s.archive`.
- **File lock** (`memory.lock`): writers write
  `pid + timestamp` and block the file during persistence; if the lock
  is held by a dead or stale PID (15 s) it is taken (stolen). Max wait
  10 s. On Windows, `sharing violation` errors are treated as an occupied lock.
- **Atomic writes**: temp + `rename`. Permissions `0600`.
- Readers compare the `fileStamp` (mtime+size) of the memory file
  against the last loaded: if another process wrote, they reload
  (`loadFile`/`adoptForeignLocked`). This allows **multiple processes**
  sharing the same memory directory.
- The `vocab` (project) and `config.json` are rewritten with `writeMetaLocked`.
- Format: `json` (default) or `ambar`; `SetFormat` migrates files.

## Search pipeline (`Rank`)

```
 filter       -> match        -> (graph)    -> scoring     -> order     -> limit
 visible      -> tokens       -> ego       -> BM25 +      -> RRF or    -> top N
 (status,     -> keywords     -> subgraph  -> match       -> linear
  ttl, asOf,    + tags AND     + seeds    -> recency     + bumped
  dates,        + path glob              + frequency
  pathscope)
```

Steps:

1. **Visibility**: only active, non-expired capsules within the temporal
   view (`AsOf`) and date filters.
2. **Match**: if there is a query, `matchQuery` (BM25 does not filter; prior
   filtering uses tokens + keywords with accent normalization and `splitCamel`).
   `matchTags` applies AND label filtering; `pathMatch` applies the
   `PathScope` glob against `SessionFiles`.
3. **Graph** (`Graph: true`): `EgoExpand` on seeds to include neighbors
   at `Hops` (1 or 2) as candidates marked with `Dist`.
4. **Scoring**: by default uses a **linear combination** of BM25 +
   centrality + importance; with `RRF: true` rankings of BM25,
   keyword match, and frequency/recency are merged by rank. Capsules with
   `Priority > 0` are moved to the front. Seeds (direct match)
   retain `IsSeed: true`.
5. **Bump**: returned results increment their access counter.

Relevant functions: `BM25Scores` (k1=1.5, b=0.75), `rrfScore`, `rankOf`,
`Render(rs, budget)` with budgets `tiny`/`normal`/`deep`.

### Scoring (`scoring.go`)

- `QualityScore(c, now)`: weighted blend of
  - content length (0.30),
  - tags present (0.30),
  - links (0.15),
  - access frequency (0.10, capped),
  - recency (0.10),
  - specificity/origin (0.10),
  - with a **stagnation cap** (0.20) if not accessed in 90 days.
- `Importance(c, now) = recency*0.6 + frequency*0.4` (both normalized to
  [0,1]).

## Link graph (`graph.go`)

- `BuildGraph` connects capsules whose `Links` point to other keys present,
  and also links by **shared tags** (edge weight `sharedTagCount`).
- `LinkKey` normalizes a key for use as a node.
- `Centrality` = normalized degree; cached per day (`cachedCentral`) and
  invalidated with `bumpGraph` on store mutation.
- `EgoExpand(seeds, hops, visible)` performs BFS up to `hops` hops and returns
  `map[key]dist` for the pipeline expansion step.
- `ShortestPath(a, b)` (BFS) to answer "how are two memories related".

## Sessions (`sessions.go`)

The `SessionTracker` maintains a **heartbeat file per session** in
`<dir>/sessions/<id>.json`:

```json
{ "id": "...", "agent": "...", "pid": 1234, "branch": "main",
  "started_at": "...", "last_seen": "...", "files": {"path": "iso"}, "ended": false }
```

- `Touch(file)` / `Heartbeat(file, ended)` update the state (cap of 200
  files per session; after that the oldest are pruned).
- `ActiveSessions()` lists live sessions (10 min TTL: if
  `last_seen` is older and the PID is dead, it is considered ended).
- `Conflicts()` detects **files touched by two or more live sessions**
  (possible concurrent editing).
- `SessionFiles()` returns the set of files touched by *this* session,
  which the search pipeline uses to filter by `PathScope`.
- `Prune()` removes orphaned heartbeats.

## Ledger (`ledger.go`, `rotate.go`, `summary.go`)

The ledger is a **JSON-lines** activity log (v2, `V:2`) with automatic
rotation:

- `NewLedger(cwd, ...)` locates the project root (`DetectRoot`, up to 10
  levels up searching for `.git`/`go.mod`/etc.) and chooses
  `~/.goulm/ledger/<Project>/` unless `WithHome` is used to isolate it.
- `Append*`: `AppendTool`, `AppendEdit`, `AppendCommit`, `AppendError`,
  `AppendMemory`, `AppendSessionStart/End`, `AppendMilestone`,
  `AppendApproval`. Each event carries `TS`, `Type`, `Action`, `Session`,
  and optional fields (`Path`, `Detail`, `Hash`, `Risk`, `Status`,
  `Approved`, `Tokens`, `CostUSD`, `DurationMs`, `Turn`, `Test`).
- **Rotation**: on write, if the active file exceeds `Window` (200 events
  by default) or 48 KiB, `CompactNow` moves the excess to
  `archives/YYYY-MM.json` grouped by month.
- `Tail(n, type, includeHistory)` returns the last n events (active and,
  optionally, historical). `Stats()` summarizes the state.
- `Summary()` aggregates by day/week/month: commits, edits (unique
  files), errors, tests, memories by category, milestones, cost, and
  duration; respects `SummaryBudget`.
- `Export(since, to)` returns the range as text.
- Sanitization: `sanitizeDetail` masks secrets and truncates to 300 characters.
- Environment variable masking: `GOULM_LEDGER=off` disables;
  the session is obtained from `GOULM_SESSION_ID` if defined.

`LedgerHook` (in `pkg/tools`) is the bridge between the tools and the ledger:
it observes `OnToolStart`/`OnToolResult` (via `EventSink` or wrapping `Execute`)
and logs `tool`/`approval`/`session`/`milestone` to an **async writer**
with queue and counted drops (`Stats()`).

## Tool layer (`pkg/tools`)

`Registry` is a flat registry with the same defaults as the Goulm agent
(timeout 30 s, category `inspect`, risk `low` or `high` depending on
`RequiresApproval`). `RegisterMemoryTools` registers 11 memory tools and
`RegisterLedgerTools` the 2 ledger tools. Each tool receives `(ctx, input string)`
and returns `(string, error)`, where `input` is a JSON of arguments. See
`API.md` for each tool's parameters.

## Demo flow (`cmd/demo`)

1. Resolves `cwd` and `-dir` (default `~/.goulm-memory/<ProjectID>`).
2. Opens the store (JSON), injects project vocabulary, opens a
   `SessionTracker`, creates the isolated ledger and `LedgerHook`.
3. Registers the 13 tools in a `Registry`.
4. Runs the requested subcommand through the Registry (same path as an
   agent), with exit codes 0/1/2.
