# Changelog

Historial de cambios de `goulm-memory`. Formato
[Keep a Changelog](https://keepachangelog.com/es/1.1.0/); el módulo sigue
[SemVer](https://semver.org/).

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

[0.3.0]: https://github.com/LRGolden/goulm-memory/releases/tag/v0.3.0
[0.2.0]: https://github.com/LRGolden/goulm-memory/releases/tag/v0.2.0
[0.1.0]: https://github.com/LRGolden/goulm-memory/releases/tag/v0.1.0
