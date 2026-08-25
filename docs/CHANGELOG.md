# Changelog

Changelog history of `goulm-memory`. Format
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); the module follows
[SemVer](https://semver.org/).

## [0.4.6] — 2026-08-25

Critical memory fix in Recall + VP-Tree for vector search.

### Fixed

- **BuildGraph O(N²) fix** (`pkg/memory/graph.go`): tags appearing in
  >50 capsules no longer create synthetic edges. Reduces Recall N=1000
  from ~214MB to ~4.3MB (-98%). Explicit links and [[key]] refs are not
  affected.

### Added

- **VP-Tree** (`pkg/memory/vptree.go`): VP tree for approximate nearest
  neighbor search. Replaces brute-force for N>1000.
  Build O(N×log N), query O(log N×D), memory O(N).
- **vectorScoresVP** (`pkg/memory/embedding.go`): VP-Tree integration
  into the ranking pipeline. Automatic fallback to brute-force if the
  tree is not available.
- **vpTreeFor** (`pkg/memory/store.go`): lazy cache of the VP-Tree,
  rebuilt when the store mutates.
- **VECTOR_SEARCH.md** (`docs/VECTOR_SEARCH.md`): documentation of
  evaluated vector search methods.
- **benchmark_recall_profile_test.go**: benchmark with memprofile
  for allocation diagnostics.
- **graph_test.go**: regression tests for graph with many capsules.
- **vptree_test.go**: unit tests for the VP-Tree.

## [0.4.5] — 2026-08-24

Deep Recall optimization: levenshtein with reusable buffers and
matchQuery with pre-computed tokens.

### Improved

- **levenshtein buffers** (`pkg/memory/ranking.go`): sync.Pool for
  `prev`/`cur` arrays. Eliminates ~119K allocations per Recall with N=1000
  (51% of the previous total).
- **matchQuery tokens** (`pkg/memory/ranking.go`): uses `c.Tokens`
  pre-computed instead of re-tokenizing. Eliminates ~1K allocations per Recall.

## [0.4.4] — 2026-08-23

Memory and performance optimization in the Recall pipeline.

### Improved

- **FullText cache** (`pkg/memory/capsule.go`): `FullText()` now caches
  the result. Prevents redundant reconstruction in matchQuery and
  BM25Scores (~5000 fewer allocations per Recall with N=1000).
- **Pre-computed tokens** (`pkg/memory/capsule.go`, `memory.go`):
  `Tokens []string` field in Capsule, computed during Remember.
  BM25Scores reuses existing tokens instead of re-tokenizing
  (~3000 fewer allocations).
- **BM25 token limit** (`pkg/memory/ranking.go`): BM25Scores now
  truncates to 50 tokens per capsule (consistent with matchQuery).
- **matchTags pool** (`pkg/memory/ranking.go`): sync.Pool for the tags
  map in matchTags. Reuses memory between calls (~1000 fewer allocations).
- **RRF rankers** (`pkg/memory/ranking.go`): rankers slice moved
  outside the scoring loop. Built once, not per capsule (~1000
  fewer allocations).
- **Ambar format** (`pkg/memory/ambar.go`): serializes/deserializes
  `tokens>` field for pre-computed tokens.

### Changed

- **Capsule.Clone()**: copies `Tokens` field and `fullText` cache.
- **MergeCapsules**: recalculates tokens when Content changes.
- **ImportCapsules**: computes tokens if they don't exist in the imported capsule.

## [0.4.3] — 2026-08-23

HTTP authentication and performance benchmarks.

### Added

- **HTTP API Key auth** (`cmd/serve/main.go`): `-api-key` flag and env var
  `GOULM_API_KEY`. Header `X-API-Key` or `Authorization: Bearer <key>`.
  Timing-attack safe comparison with `crypto/subtle`. `/healthz` without auth.
  Backward compatible: no key = no auth.
- **Benchmarks** (`pkg/memory/benchmark_*_test.go`): 7 benchmark files
  covering Remember, Recall, SmartRecall, Forget, BM25Scores,
  VectorScores, BuildGraph, Centrality, concurrent writes, concurrent
  reads, and mixed workload. Tests with N=10, 100, 500, 1000.

### Documented

- **SERVER.md**: documentation of `-api-key`, `GOULM_API_KEY`, authentication
  headers, and updated examples (curl, Python, TypeScript).

## [0.4.2] — 2026-08-22

Security hardening, input validation, and recovery mode for corrupt files.
Granular review of 26 points covering compatibility, security,
resilience, reliability, robustness, and scalability.

### Fixed

- **HTTP body limit** (`cmd/serve/main.go`): `io.LimitReader` with 1 MB max.
  Prevents DoS via giant request body.
- **CORS localhost** (`cmd/serve/main.go`): default `http://localhost:*`
  instead of `*`. `-cors` flag to configure origin.
- **Content max length** (`pkg/memory/capsule.go`): 100 KB max for Content,
  10 tags max, 64 chars per tag, 128 chars per key. Prevents memory DoS.
- **MaxEntries/MaxBackups cap** (`pkg/memory/store.go`): caps of 10000 and
  100 respectively. Prevents malicious config.
- **Corrupt file recovery** (`pkg/memory/store.go`): corrupt files
  (memory.json, archive.json) are renamed to `.corrupt.<timestamp>` and
  the store opens with empty state. Previously, a single corrupt byte
  blocked the entire store.
- **Import content validation** (`pkg/memory/memory.go`): `ImportCapsules`
  rejects capsules with empty content.
- **Import duplicate ID dedupe** (`pkg/memory/memory.go`): capsules with
  the same ID on import are merged instead of causing phantom keys.
- **Levenshtein bound** (`pkg/memory/ranking.go`): fuzzy match limited
  to 50 document tokens per capsule. Prevents O(N²) with long content.
- **Export timestamp check** (`pkg/memory/ledger.go`): guard before
  `ev.TS[:10]` to prevent panic with malformed timestamps.
- **secretRE bound** (`pkg/memory/health.go`): `[A-Z ]*` replaced
  with `[A-Z]{0,50}`. Prevents theoretical ReDoS.
- **MaxArchive** (`pkg/memory/store.go`, `pkg/memory/merge.go`): archive
  limited to 500 capsules by default. Consolidate performs automatic
  quality-based pruning.

### Documented

- **Path traversal warning** (`docs/ADVANCED.md`): `File` and `PathScope`
  fields are stored unsanitized. Consumer is responsible.
- **One store per directory** (`docs/ADVANCED.md`): two stores in the
  same directory cause lockfile contention.
- **Flush volatile metadata** (`docs/ADVANCED.md`): access bumps are
  lost if Flush is not called before exiting.
- **TTL expiration behavior** (`docs/ADVANCED.md`): expired capsules
  don't appear in Recall but remain in the store.

## [0.4.1] — 2026-08-22

Production robustness fixes: cross-platform file locking,
deadlock protection, and performance optimizations at scale.

### Fixed

- **Dirty flag bug** (`pkg/memory/store.go`): `Flush()` now sets `dirty=false`
  after `persistLocked()` returns success, not before. Prevents silent
  data loss if persistence fails (disk full, lock timeout).
- **PID recycling** (`pkg/memory/store.go`): lock file now includes UUID v4
  in addition to PID and timestamp. Prevents a PID reused by the OS from
  retaining the lock unnecessarily.
- **Stale timeout with large files** (`pkg/memory/store.go`): added
  refresh heartbeat every 5s during `persistLocked`. Prevents another
  process from stealing the lock while the holder is writing a large file.
- **Thundering herd** (`pkg/memory/store.go`): random jitter (100-150ms)
  on lock contention sleep. Reduces simultaneous retry collisions.
- **Clock backward** (`pkg/memory/store.go`): locks with future timestamp
  (>5s) are treated as stale. Protects against NTP jumps.
- **Embed() timeout** (`pkg/memory/embedding.go`): `EmbeddingProvider`
  interface now accepts `context.Context`. 5s timeout in `VectorScores`.
  Prevents store deadlock if the embedding provider hangs.
- **Dimension validation** (`pkg/memory/embedding.go`): `VectorScores` validates
  that the query dimension matches the provider's dimension. Capsules with
  incorrect dimension are skipped (graceful degradation).
- **Stored dimension metadata** (`pkg/memory/capsule.go`): `EmbeddingDim`
  field in `Capsule` records the provider's dimension on storage. Enables
  diagnosing incompatibility between old embeddings and current provider.

### Changed

- **EmbeddingProvider** (`pkg/memory/embedding.go`): interface changes from
  `Embed(text)` to `Embed(ctx, text)`. Breaking change justified by
  early project stage (no external implementers).
- **BuildGraph** (`pkg/memory/graph.go`): shared tags now use
  inverted index tag→keys instead of pair loop. O(N*T) instead of O(N²).
- **Consolidate Phase 3** (`pkg/memory/merge.go`): Jaccard comparisons
  limited to 500 per execution. Prevents blocking with large collections.
- **matchQuery** (`pkg/memory/ranking.go`): `FullText()` called once
  instead of twice per capsule. Eliminates unnecessary duplication.
- **Goroutine safety** (`pkg/memory/embedding.go`): documented that
  `EmbeddingProvider` implementations must be safe for concurrent use.

### Infrastructure

- **Orphaned tmp cleanup** (`pkg/memory/store.go`): `NewStore` removes
  orphaned `.tmp-*` files from previous crashes.
- **Lock file format**: changed from `<PID> <TS>` to `<PID> <TS> <UUID>`.
  Backward compatible: old locks without UUID are cleaned up automatically.

## [0.4.0] — 2026-08-21

Growth foundations: embeddings for semantic search and HTTP server
for multi-language clients. No new dependencies; everything via
interfaces and stdlib.

### Added

- **EmbeddingProvider interface** (`pkg/memory/embedding.go`): minimal
  interface for embedding providers (OpenAI, Cohere, local models).
  `Embed(ctx, text) ([]float64, error)` + `Dimension() int`. Support
  for cancellation via context.Context.
- **VectorScores**: cosine similarity function that integrates embeddings
  into the ranking pipeline. Fixed weight 0.3 in linear combination; with
  RRF it's added as an additional ranker.
- **Embedding field in Capsule**: `[]float64` with `omitempty`, compatible
  with existing JSON and Ambar files. Serialized as `embedding>` in
  Ambar format.
- **Embedding in RememberOptions**: pre-calculated embedding to avoid
  HTTP calls inside the `Remember` lock.
- **SetEmbedder/Embedder**: methods to configure the provider in the store.
- **HTTP server** (`cmd/serve`): minimal server exposing 12 JSON
  endpoints (remember, recall, stats, health, forget, resolve, pin, backup,
  archive, consolidate, capsules). Flags `-addr` and `-dir`. Basic CORS.
- **Tests**: `TestVectorScores`, `TestCosineSim`, `TestCapsuleEmbedding*`,
  `TestAmbarEmbeddingRoundtrip`, `TestRememberWithEmbedding`, `TestSetEmbedder`.
- **docs/EMBEDDINGS.md**: integration guide with OpenAI and ollama examples.
- **docs/SERVER.md**: HTTP server documentation with Python and
  TypeScript examples.

### Changed

- **Capsule.Clone()**: deep-copy of the Embedding slice.
- **MergeCapsules**: prioritizes incoming embedding.
- **Rank pipeline**: vecScores integration with nil guard (no provider =
  identical behavior to v0.3.x).
- **docs/ADVANCED.md**: Embeddings and HTTP Server sections.
- **docs/API.md**: EmbeddingProvider interface in "Full API".

## [0.3.0] — 2026-08-21

Documentation surface restructuring to reduce the barrier to
entry. No code changes; documentation is simply reorganized into
levels: essential (general consumer), complete (reference), and advanced
(sessions, ledger, graph, and tools integration).

### Added

- **docs/QUICKSTART.md**: step-by-step guide with 3 examples (remember,
  search, view status). 1-page entry for new consumers.
- **docs/ADVANCED.md**: advanced integration guide with sessions, ledger,
  link graph, tools, and a complete agent integration example.
- **docs/TOOLS.md**: table of the 13 tools with parameters, extracted from
  the README to decouple technical reference from presentation.

### Changed

- **README.md**: rewritten (~50 lines vs previous 189). Focused on
  what it is, minimal 10-line example, and links to documentation. Removed
  concurrency, security, and tools table sections (now documented
  in dedicated files).
- **docs/API.md**: reorganized into "Essential API" (types, Config, Remember,
  Recall, Stats, Render, metadata) and "Full API" (scoring, graph,
  sessions, ledger, reports, git, ambar, tools). New consumers only
  need the first section.

Quality review and inconsistency fixes between source code and
documentation, plus enabling half-implemented functionality and removing
dead code. All existing tests continue passing.

### Added

- **SupersededOn active**: `Forget(key, hard=false)` now sets the
  `SupersededOn` field with the supersession date. Until this version the field
  existed in the structure but was never written, which prevented the
  `AsOf` temporal view logic from working correctly. `Resolve(key)`
  clears `SupersededOn` when restoring the capsule.
- **New tests**: `TestSupersededOnSetByForget`,
  `TestSupersededOnClearedByResolve`, `TestAsOfViewWithSupersededOn`,
  `TestPersistSortedByKey`, `TestNoStatusDraft`.

### Fixed

- **Deterministic persistence**: `writeLocked()` and `ExportJSON()` now
  sort capsules by `Key` before serializing. Previously the order
  depended on Go's map iteration, which produced rewrites with
  spurious diffs every time `Flush()` was run after a recall. JSON and
  Ambar files are now stable and genuinely diff-friendly.
- **Unified `Resolve` contract**: the documentation (README, API.md,
  ARCHITECTURE.md) and the `memory_resolve` tool described different
  behaviors. All three documents were corrected to reflect the actual
  code behavior: `Resolve` **restores** a soft-deleted capsule to `active`,
  reverting `memory_forget`.
- **CompactNow simplified** (`pkg/memory/rotate.go`): removed dead
  code (two redundant `return nil` branches) and the unused `before`
  variable.
- **LedgerHook.Close() idempotent** (`pkg/tools/ledger_hook.go`): added
  `sync.Once` to prevent a panic from multiple close calls on the `done`
  channel.
- **Documentation synchronized**:
  - `docs/ARCHITECTURE.md`: corrected the ranking pipeline description
    to reflect that RRF is opt-in and the default is linear combination.
  - `docs/API.md`: `StatsView` and `HealthReport` definitions now
    match the actual code structs. Removed `StatusDraft` from
    documented types.
  - `docs/FORMATS.md`: corrected the active ledger file extension
    from `ledger.json` to `ledger.jsonl` (JSON-lines).
  - `pkg/tools/memory_tools.go`: corrected the RegisterMemoryTools
    comment from "14 tools" to "11 tools".

### Removed

- **StatusDraft**: removed the `StatusDraft` constant and its references in
  `ValidStatus` and `IsVisible` (`pkg/memory/capsule.go`). There was no
  write path that assigned this state, so it was dead code that
  could cause confusion.

### Internal Changes

- `InferTags` (`pkg/memory/tags.go`): replaced manual bubble sort with
  `sort.Strings` for stable ordering and more concise code.
- `Registry.Names()` (`pkg/tools/tool.go`): now returns keys
  sorted alphabetically.
- `EventBranch` and `EventCheckout` constants (`pkg/memory/ledger.go`):
  marked as reserved for future git integration.

## [0.1.0] — 2026-08-20

Initial version. Extracted from the memory subsystem of
[Goulm](https://github.com/LRGolden/goulm) as a standalone Go module
(MIT), with the tools layer adapted to a standalone `Registry`.

### Added

- **`pkg/memory`** (100% stdlib, no external dependencies):
  - `MemoryStore`: load/persistence with multi-process file lock,
    atomic writes, external write detection (`fileStamp`),
    JSON / Ambar format migration.
  - Capsule model (`Capsule`): categories, origin with confidence,
    TTL, priority (pin), path-scope, soft/hard delete, temporal
    visibility (`AsOf`).
  - `Remember` with key fusion; `Recall`, `SmartRecall`, `Suggest`
    via hybrid ranking pipeline (BM25 + keywords + RRF + ego-subgraph
    + recency/frequency) with `Render` by budget.
  - Link graph (`BuildGraph`, `Centrality`, `EgoExpand`,
    `ShortestPath`) with daily cache.
  - Sessions (`SessionTracker`): heartbeats, 10 min TTL, file
    conflicts between sessions, `SessionFiles` for path-scope filtering.
  - Ledger JSON-lines v2: 10 `Append*` methods, rotation/compaction by
    window and size, `Tail`, `Stats`, `Export`, `Summary` by
    day/week/month, secret masking, disableable via environment
    (`GOULM_LEDGER=off`).
  - Maintenance: `Stats`, `Diff`, `Backup` (with pruning), `Primer`,
    `Health`, `Consolidate` (Jaccard + merge), `ImportCapsules`/`ExportJSON`.
  - Git utilities: `CurrentBranch`, `ProjectID`, `HasGitDir`, reflog
    (`CurrentHead`, `ReflogNew`).
  - Tag inference with project vocabulary (`InferTags`,
    `ExtractProjectDeps` for `go.mod`, `package.json`, `requirements.txt`).
  - **Ambar** format: diff-friendly plain text serialization.
- **`pkg/tools`**:
  - `Registry` with Goulm agent defaults (30s timeout, default risk/category)
    and derived metadata.
  - 13 registerable tools: 11 memory (`memory_remember`, `memory_recall`,
    `memory_suggest`, `memory_stats`, `memory_forget`, `memory_resolve`,
    `memory_archive`, `memory_pin`, `memory_backup`, `memory_consolidate`,
    `context_brief`) + 2 ledger (`ledger_tail`, `ledger_log`).
  - `LedgerHook`: observes tool execution (start/result), approvals and
    milestones with async writer (queue + drops), `Wrap` to interpose
    in an `EventSink`.
- **`cmd/demo`**: demonstration CLI (15 subcommands) that wires
  store + session tracker + ledger + Registry; memory isolated in
  `~/.goulm-memory/<ProjectID>` (or `-dir`); exit codes 0/1/2.
- **Documentation**: README, `docs/ARCHITECTURE.md`, `docs/API.md`,
  `docs/FORMATS.md`, this changelog.

### Fixed

- Tool count corrected relative to legacy Goulm documentation:
  the ecosystem exposes **13** tools (11 memory + 2 ledger), not 14/16.

### Notes

- The original Goulm repo had `gofmt` misalignments in
  `pkg/memory`; normalized with `gofmt -w` (cosmetic, no behavior change).
- When working within the Goulm repo (which uses `go.work`), compiling/testing
  this module requires `$env:GOWORK="off"`.

[0.4.6]: https://github.com/LRGolden/goulm-memory/releases/tag/v0.4.6
[0.4.5]: https://github.com/LRGolden/goulm-memory/releases/tag/v0.4.5
[0.4.4]: https://github.com/LRGolden/goulm-memory/releases/tag/v0.4.4
[0.4.3]: https://github.com/LRGolden/goulm-memory/releases/tag/v0.4.3
[0.4.2]: https://github.com/LRGolden/goulm-memory/releases/tag/v0.4.2
[0.4.1]: https://github.com/LRGolden/goulm-memory/releases/tag/v0.4.1
[0.4.0]: https://github.com/LRGolden/goulm-memory/releases/tag/v0.4.0
[0.3.0]: https://github.com/LRGolden/goulm-memory/releases/tag/v0.3.0
[0.2.0]: https://github.com/LRGolden/goulm-memory/releases/tag/v0.2.0
[0.1.0]: https://github.com/LRGolden/goulm-memory/releases/tag/v0.1.0
