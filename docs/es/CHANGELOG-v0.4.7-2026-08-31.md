# Changelog

Historial de cambios de `goulm-memory`. Formato
[Keep a Changelog](https://keepachangelog.com/es/1.1.0/); el módulo sigue
[SemVer](https://semver.org/).

## [0.4.7] — 2026-08-31

Optimizacion de grado industrial para rendimiento de busqueda vectorial en dispositivos edge.

### Mejorado

- **Pruning VP-Tree** (`pkg/memory/vptree.go`): El radio de busqueda ahora se reduce dinamicamente a medida que se encuentran k-vecinos mas cercanos. Restaura el verdadero rendimiento de busqueda `O(log N)` en lugar de degradarse a escaneo lineal.
- **Build VP-Tree** (`pkg/memory/vptree.go`): Se reemplazo `sort.Slice` con un algoritmo `Quickselect` in-place para encontrar la mediana. El tiempo de construccion es ahora estrictamente `O(N×log N)`.
- **SIMD de distancias vectoriales** (`pkg/memory/vptree.go`, `pkg/memory/embedding.go`): Loop unrolling implementado para `euclideanDist` y `cosineSim` (bloques de 4) para permitir autovectorizacion del compilador.

## [0.4.6] — 2026-08-25

Fix critico de memoria en Recall + VP-Tree para vector search.

### Corregido

- **BuildGraph O(N²) fix** (`pkg/memory/graph.go`): tags que aparecen
  en >50 cápsulas ya no crean edges sintéticos. Reduce Recall N=1000
  de ~214MB a ~4.3MB (-98%). Links explícitos y refs [[clave]] no se
  afectan.

### Agregado

- **VP-Tree** (`pkg/memory/vptree.go`): árbol VP para búsqueda de
  nearest neighbors aproximada. Reemplaza brute-force para N>1000.
  Build O(N×log N), query O(log N×D), memoria O(N).
- **vectorScoresVP** (`pkg/memory/embedding.go`): integración del
  VP-Tree en el pipeline de Ranking. Fallback automático a brute-force
  si el tree no está disponible.
- **vpTreeFor** (`pkg/memory/store.go`): cache lazy del VP-Tree,
  reconstruido cuando el store muta.
- **VECTOR_SEARCH.md** (`docs/VECTOR_SEARCH.md`): documentación de
  métodos de busqueda vectorial evaluados.
- **benchmark_recall_profile_test.go**: benchmark con memprofile
  para diagnóstico de allocs.
- **graph_test.go**: tests de regresión para grafo con muchas cápsulas.
- **vptree_test.go**: tests unitarios del VP-Tree.

## [0.4.5] — 2026-08-24

Optimizacion profunda de Recall: levenshtein con buffers reusables y
matchQuery con tokens pre-computed.

### Mejorado

- **levenshtein buffers** (`pkg/memory/ranking.go`): sync.Pool para
  `prev`/`cur` arrays. Elimina ~119K allocs por Recall con N=1000
  (51% del total anterior).
- **matchQuery tokens** (`pkg/memory/ranking.go`): usa `c.Tokens`
  pre-computed en vez de re-tokenizar. Elimina ~1K allocs por Recall.

## [0.4.4] — 2026-08-23

Optimizacion de memoria y performance en el pipeline de Recall.

### Mejorado

- **FullText cache** (`pkg/memory/capsule.go`): `FullText()` ahora cachea
  el resultado. Previene reconstruccion redundante en matchQuery y
  BM25Scores (~5000 allocs menos por Recall con N=1000).
- **Tokens pre-computed** (`pkg/memory/capsule.go`, `memory.go`):
  Campo `Tokens []string` en Capsule, computado durante Remember.
  BM25Scores reutiliza tokens existentes en vez de tokenizar de nuevo
  (~3000 allocs menos).
- **BM25 token limit** (`pkg/memory/ranking.go`): BM25Scores ahora
  trunca a 50 tokens por capsula (consistente con matchQuery).
- **matchTags pool** (`pkg/memory/ranking.go`): sync.Pool para el map
  de tags en matchTags. Reusa memoria entre llamadas (~1000 allocs menos).
- **RRF rankers** (`pkg/memory/ranking.go`): Slice de rankers movido
  fuera del scoring loop. Se construye una vez, no por capsula (~1000
  allocs menos).
- **Ambar format** (`pkg/memory/ambar.go`): Serializa/deserializa
  campo `tokens>` para pre-computed tokens.

### Cambio

- **Capsule.Clone()**: Copia campo `Tokens` y cache `fullText`.
- **MergeCapsules**: Recalcula tokens cuando Content cambia.
- **ImportCapsules**: Computa tokens si no existen en la capsula importada.

## [0.4.3] — 2026-08-23

Autenticacion HTTP y benchmarks de performance.

### Agregado

- **HTTP API Key auth** (`cmd/serve/main.go`): flag `-api-key` y env var
  `GOULM_API_KEY`. Header `X-API-Key` o `Authorization: Bearer <key>`.
  Comparacion timing-attack safe con `crypto/subtle`. `/healthz` sin auth.
  Backward compatible: sin key = sin auth.
- **Benchmarks** (`pkg/memory/benchmark_*_test.go`): 7 archivos de
  benchmark cubriendo Remember, Recall, SmartRecall, Forget, BM25Scores,
  VectorScores, BuildGraph, Centrality, concurrent writes, concurrent
  reads, y mixed workload. Tests con N=10, 100, 500, 1000.

### Documentado

- **SERVER.md**: documentacion de `-api-key`, `GOULM_API_KEY`, headers
  de autenticacion, y ejemplos actualizados (curl, Python, TypeScript).

## [0.4.2] — 2026-08-22

Hardening de seguridad, validacion de inputs y recovery mode para archivos
corruptos. Revision granular de 26 puntos de compatibilidad, seguridad,
resiliencia, confiabilidad, robustez y escalabilidad.

### Corregido

- **HTTP body limit** (`cmd/serve/main.go`): `io.LimitReader` de 1 MB max.
  Previene DoS por request body gigante.
- **CORS localhost** (`cmd/serve/main.go`): default `http://localhost:*`
  en vez de `*`. Flag `-cors` para configurar origen.
- **Content max length** (`pkg/memory/capsule.go`): 100 KB max en Content,
  10 tags max, 64 chars por tag, 128 chars por key. Previene memory DoS.
- **MaxEntries/MaxBackups cap** (`pkg/memory/store.go`): caps de 10000 y
  100 respectivamente. Previene config maliciosa.
- **Corrupt file recovery** (`pkg/memory/store.go`): archivos corruptos
  (memory.json, archive.json) se renombran como `.corrupt.<timestamp>` y
  el store se abre con estado vacio. Antes, un byte corrupto bloqueaba
  todo el store.
- **Import content validation** (`pkg/memory/memory.go`): `ImportCapsules`
  rechaza capsulas con contenido vacio.
- **Import duplicate ID dedupe** (`pkg/memory/memory.go`): capsulas con
  mismo ID en import se fusionan en vez de causar phantom keys.
- **Levenshtein bound** (`pkg/memory/ranking.go`): fuzzy match limitado
  a 50 doc tokens por capsula. Previene O(N²) con contenido largo.
- **Export timestamp check** (`pkg/memory/ledger.go`): guard antes de
  `ev.TS[:10]` para prevenir panic con timestamps malformados.
- **secretRE bound** (`pkg/memory/health.go`): `[A-Z ]*` reemplazado
  por `[A-Z]{0,50}`. Previene ReDoS teorico.
- **MaxArchive** (`pkg/memory/store.go`, `pkg/memory/merge.go`): archive
  limitado a 500 capsulas por defecto. Consolidate hace prune automatico
  por calidad.

### Documentado

- **Path traversal warning** (`docs/ADVANCED.md`): campos `File` y
  `PathScope` se almacenan sin sanitizar. Consumidor es responsable.
- **Un store por directorio** (`docs/ADVANCED.md`): dos stores en el
  mismo directorio causan contention de lockfile.
- **Flush volatile metadata** (`docs/ADVANCED.md`): access bumps se
  pierden si Flush no se llama antes de terminar.
- **TTL expiration behavior** (`docs/ADVANCED.md`): capsulas expiradas
  no aparecen en Recall pero siguen en el store.

## [0.4.1] — 2026-08-22

Correcciones de robustez para produccion: file locking multi-plataforma,
proteccion contra deadlocks, y optimizaciones de rendimiento a escala.

### Corregido

- **Dirty flag bug** (`pkg/memory/store.go`): `Flush()` ahora pone `dirty=false`
  despues de que `persistLocked()` retorna exito, no antes. Previene perdida
  silenciosa de datos si la persistencia falla (disco lleno, lock timeout).
- **PID recycling** (`pkg/memory/store.go`): lock file ahora incluye UUID v4
  ademas de PID y timestamp. Previene que un PID reusado por el OS retenga
  el lock innecesariamente.
- **Stale timeout con archivos grandes** (`pkg/memory/store.go`): agregado
  heartbeat de refresh cada 5s durante `persistLocked`. Previene que otro
  proceso robe el lock mientras el holder esta escribiendo un archivo grande.
- **Thundering herd** (`pkg/memory/store.go`): jitter aleatorio (100-150ms)
  en el sleep de lock contention. Reduce colision de retries simultaneos.
- **Clock backward** (`pkg/memory/store.go`): locks con timestamp futuro
  (>5s) se tratan como stale. Protege contra saltos de NTP.
- **Embed() timeout** (`pkg/memory/embedding.go`): interfaz `EmbeddingProvider`
  ahora acepta `context.Context`. Timeout de 5s en `VectorScores`. Previene
  deadlock del store si el provider de embeddings cuelga.
- **Dimension validation** (`pkg/memory/embedding.go`): `VectorScores` valida
  que la dimension del query coincida con la del provider. Capsulas con
  dimension incorrecta se saltan (degradation graceful).
- **Stored dimension metadata** (`pkg/memory/capsule.go`): campo `EmbeddingDim`
  en `Capsule` registra la dimension del provider al almacenar. Permite
  diagnosticar incompatibilidad entre embeddings viejos y provider actual.

### Cambiado

- **EmbeddingProvider** (`pkg/memory/embedding.go`): interfaz cambia de
  `Embed(text)` a `Embed(ctx, text)`. Breaking change justificado por
  etapa temprana del proyecto (sin implementadores externos).
- **BuildGraph** (`pkg/memory/graph.go`): tags compartidas ahora usan
  indice invertido tag→keys en vez de loop de pares. O(N*T) en vez de O(N²).
- **Consolidate Phase 3** (`pkg/memory/merge.go`): comparaciones Jaccard
  limitadas a 500 por ejecucion. Previene bloqueo con colecciones grandes.
- **matchQuery** (`pkg/memory/ranking.go`): `FullText()` se llama una vez
  en vez de dos por capsula. Elimina duplicacion innecesaria.
- **Goroutine safety** (`pkg/memory/embedding.go`): documentado que las
  implementaciones de `EmbeddingProvider` deben ser seguras para uso
  concurrente.

### Infraestructura

- **Orphaned tmp cleanup** (`pkg/memory/store.go`): `NewStore` elimina
  archivos `.tmp-*` huérfanos de crashes anteriores.
- **Lock file format**: de `<PID> <TS>` a `<PID> <TS> <UUID>`. Backward
  compatible: locks viejos sin UUID se limpian automaticamente.

## [0.4.0] — 2026-08-21

Fundaciones para crecimiento: embeddings para busqueda semantica y server
HTTP para clientes multi-lenguaje. Sin dependencias nuevas; todo via
interfaces y stdlib.

### Anadido

- **EmbeddingProvider interface** (`pkg/memory/embedding.go`): interfaz
  minima para proveedores de embeddings (OpenAI, Cohere, modelos locales).
  `Embed(ctx, text) ([]float64, error)` + `Dimension() int`. Soporte
  para cancellation via context.Context.
- **VectorScores**: funcion de similitud coseno que integra embeddings
  en el pipeline de ranking. Peso fijo 0.3 en combinacion lineal; con
  RRF se agrega como ranker adicional.
- **Campo Embedding en Capsule**: `[]float64` con `omitempty`, compatible
  con archivos JSON y Ambar existentes. Serializado como `embedding>` en
  formato Ambar.
- **Embedding en RememberOptions**: embedding pre-calculado para evitar
  llamadas HTTP dentro del lock de `Remember`.
- **SetEmbedder/Embedder**: metodos para configurar el provider en el store.
- **HTTP server** (`cmd/serve`): server minimal que expone 12 endpoints
  JSON (remember, recall, stats, health, forget, resolve, pin, backup,
  archive, consolidate, capsules). Flags `-addr` y `-dir`. CORS basico.
- **Tests**: `TestVectorScores`, `TestCosineSim`, `TestCapsuleEmbedding*`,
  `TestAmbarEmbeddingRoundtrip`, `TestRememberWithEmbedding`, `TestSetEmbedder`.
- **docs/EMBEDDINGS.md**: guia de integracion con ejemplos OpenAI y ollama.
- **docs/SERVER.md**: documentacion del server HTTP con ejemplos Python y
  TypeScript.

### Cambiado

- **Capsule.Clone()**: deep-copy del slice Embedding.
- **MergeCapsules**: prioriza embedding del incoming.
- **Rank pipeline**: integracion de vecScores con nil guard (sin provider =
  comportamiento identico a v0.3.x).
- **docs/ADVANCED.md**: secciones de Embeddings y Server HTTP.
- **docs/API.md**: interfaz EmbeddingProvider en "API completa".

## [0.3.0] — 2026-08-21

Reestructuracion de la superficie documental para reducir la barrera de
entrada. El codigo no cambia; solo se reorganiza la documentacion en
niveles: esencial (consumidor general), completa (referencia) y avanzada
(integracion con sesiones, ledger, grafo y tools).

### Anadido

- **docs/QUICKSTART.md**: guia paso a paso con 3 ejemplos (recordar,
  buscar, ver estado). Entrada de 1 pagina para consumidores nuevos.
- **docs/ADVANCED.md**: guia de integracion avanzada con sesiones, ledger,
  grafo de enlaces, tools y ejemplo completo de integracion con un agente.
- **docs/TOOLS.md**: tabla de las 13 tools con parametros, extraida del
  README para desacoplar la referencia tecnica de la presentacion.

### Cambiado

- **README.md**: reescrito (~50 lineas vs 189 anteriores). Enfocado en
  que es, ejemplo minimo de 10 lineas y links a documentacion. Eliminadas
  las secciones de concurrencia, seguridad y tabla de tools (ya documentadas
  en archivos dedicados).
- **docs/API.md**: reorganizado en "API esencial" (tipos, Config, Remember,
  Recall, Stats, Render, metadatos) y "API completa" (scoring, grafo,
  sesiones, ledger, reportes, git, ambar, tools). El consumidor nuevo solo
  necesita la primera seccion.

Revisión de calidad y corrección de inconsistencias entre el código fuente y
la documentación, además de activar funcionalidad半implementada y eliminar
código muerto. Todos los tests existentes continúan pasando.

### Añadido

- **SupersededOn activo**: `Forget(key, hard=false)` ahora establece el campo
  `SupersededOn` con la fecha de supersedencia. Hasta esta versión el campo
  existía en la estructura pero nunca se escribía, lo que impedía que la
  lógica de vista temporal `AsOf` funcionara correctamente. `Resolve(key)`
  limpia `SupersededOn` al restaurar la cápsula.
- **Tests nuevos**: `TestSupersededOnSetByForget`,
  `TestSupersededOnClearedByResolve`, `TestAsOfViewWithSupersededOn`,
  `TestPersistSortedByKey`, `TestNoStatusDraft`.

### Corregido

- **Persistencia determinista**: `writeLocked()` y `ExportJSON()` ahora
  ordenan las cápsulas por `Key` antes de serializar. Anteriormente el
  orden dependía de la iteración de mapa de Go, lo que producía reescrituras
  con diferencias espurias cada vez que se ejecutaba `Flush()` tras un
  recall. Los archivos JSON y Ámbar son ahora estables y genuinamente
  diff-friendly.
- **Contrato de `Resolve` unificado**: la documentación (README, API.md,
  ARCHITECTURE.md) y la herramienta `memory_resolve` describían comportamientos
  distintos. Se corrigieron los tres documentos para reflejar el comportamiento
  real del código: `Resolve` **restaura** una cápsula soft-deleted a `active`,
  revirtiendo `memory_forget`.
- **CompactNow simplificado** (`pkg/memory/rotate.go`): se eliminó código
  muerto (dos ramas `return nil` redundantes) y la variable `before` no
  utilizada.
- **LedgerHook.Close() idempotente** (`pkg/tools/ledger_hook.go`): se añadió
  `sync.Once` para evitar un panic por cierre múltiple del canal `done`.
- **Documentación sincronizada**:
  - `docs/ARCHITECTURE.md`: corregida la descripción del pipeline de ranking
    para reflejar que RRF es opt-in y el default es combinación lineal.
  - `docs/API.md`: las definiciones de `StatsView` y `HealthReport` ahora
    coinciden con las structs reales del código. Se eliminó `StatusDraft` de
    los tipos documentados.
  - `docs/FORMATS.md`: corregida la extensión del archivo activo del ledger
    de `ledger.json` a `ledger.jsonl` (JSON-lines).
  - `pkg/tools/memory_tools.go`: corregido el comentario de RegisterMemoryTools
    de "14 herramientas" a "11 herramientas".

### Eliminado

- **StatusDraft**: se eliminó la constante `StatusDraft` y sus referencias en
  `ValidStatus` e `IsVisible` (`pkg/memory/capsule.go`). No existía ninguna
  ruta de escritura que asignara este estado, por lo que era código muerto que
  podía confundir.

### Cambios internos

- `InferTags` (`pkg/memory/tags.go`): reemplazado el bubble sort manual por
  `sort.Strings` para orden estable y código más conciso.
- `Registry.Names()` (`pkg/tools/tool.go`): ahora devuelve las claves
  ordenadas alfabéticamente.
- Constantes `EventBranch` y `EventCheckout` (`pkg/memory/ledger.go`):
  marcadas como reservadas para futura integración con git.

## [0.1.0] — 2026-08-20

Versión inicial. Extraído del subsistema de memoria de
[Goulm](https://github.com/LRGolden/goulm) como módulo Go independiente
(MIT), con la capa de tools adaptada a un `Registry` standalone.

### Añadido

- **`pkg/memory`** (100% stdlib, sin dependencias externas):
  - `MemoryStore`: carga/persistencia con lock de archivo multi-proceso,
    escrituras atómicas, detección de escrituras externas (`fileStamp`),
    migración de formato JSON / Ámbar.
  - Modelo de cápsula (`Capsule`): categorías, origen con confianza,
    TTL, prioridad (pin), path-scope, soft/hard delete, visibilidad
    temporal (`AsOf`).
  - `Remember` con fusión por clave; `Recall`, `SmartRecall`, `Suggest`
    vía pipeline de ranking híbrido (BM25 + keywords + RRF + ego-subgraph
    + recencia/frecuencia) con `Render` por presupuesto.
  - Grafo de enlaces (`BuildGraph`, `Centrality`, `EgoExpand`,
    `ShortestPath`) con caché por día.
  - Sesiones (`SessionTracker`): heartbeats, TTL 10 min, conflictos de
    archivos entre sesiones, `SessionFiles` para filtrado por path-scope.
  - Ledger JSON-lines v2: 10 métodos `Append*`, rotación/compactación por
    ventana y tamaño, `Tail`, `Stats`, `Export`, `Summary` por
    día/semana/mes, enmascarado de secretos, deshabilitable por entorno
    (`GOULM_LEDGER=off`).
  - Mantenimiento: `Stats`, `Diff`, `Backup` (con poda), `Primer`,
    `Health`, `Consolidate` (Jaccard + merge), `ImportCapsules`/`ExportJSON`.
  - Utilidades git: `CurrentBranch`, `ProjectID`, `HasGitDir`, reflog
    (`CurrentHead`, `ReflogNew`).
  - Inferencia de tags con vocabulario del proyecto (`InferTags`,
    `ExtractProjectDeps` para `go.mod`, `package.json`, `requirements.txt`).
  - Formato **Ámbar**: serialización de texto plano diff-friendly.
- **`pkg/tools`**:
  - `Registry` con defaults del agente de Goulm (timeout 30 s, riesgo/categoría
    por defecto) y metadata derivada.
  - 13 tools registrables: 11 de memoria (`memory_remember`, `memory_recall`,
    `memory_suggest`, `memory_stats`, `memory_forget`, `memory_resolve`,
    `memory_archive`, `memory_pin`, `memory_backup`, `memory_consolidate`,
    `context_brief`) + 2 de ledger (`ledger_tail`, `ledger_log`).
  - `LedgerHook`: observa ejecución de tools (start/result), aprobaciones y
    milestones con writer asíncrono (cola + drops), `Wrap` para interponerse
    en un `EventSink`.
- **`cmd/demo`**: CLI de demostración (15 subcomandos) que cablea
  store + session tracker + ledger + Registry; memoria aislada en
  `~/.goulm-memory/<ProjectID>` (o `-dir`); exit codes 0/1/2.
- **Documentación**: README, `docs/ARCHITECTURE.md`, `docs/API.md`,
  `docs/FORMATS.md`, este changelog.

### Corregido

- Conteo de tools corregido respecto a la documentación heredada de Goulm:
  el ecosistema expone **13** tools (11 memoria + 2 ledger), no 14/16.

### Notas

- El repo original de Goulm presentaba desalineaciones de `gofmt` en
  `pkg/memory`; se normalizó con `gofmt -w` (cosmético, sin cambio de
  comportamiento).
- Al trabajar dentro del repo de Goulm (que usa `go.work`), compilar/testear
  este módulo requiere `$env:GOWORK="off"`.

[0.4.7]: https://github.com/LRGolden/goulm-memory/releases/tag/v0.4.7
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
