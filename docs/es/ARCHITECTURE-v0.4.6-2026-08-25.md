# Arquitectura

Vista general de cómo está construido `goulm-memory`, qué piezas lo componen y
cómo fluyen los datos. Para la referencia de la API, ver
[`API.md`](API.md); para los formatos de archivo, [`FORMATS.md`](FORMATS.md).

## Diagrama de paquetes

```
 cmd/demo/        CLI de demostración
    │  cablea store + tracker + ledger + Registry + LedgerHook
    ▼
 pkg/tools/       Capa de herramientas adaptada a un Registry standalone
    │  memory_tools.go (11 tools)  ledger_tools.go (2 tools)
    │  ledger_hook.go  tool.go  types.go  events.go
    ▼
 pkg/memory/      Núcleo del store (100% stdlib, sin dependencias)
    ├─ store.go        MemoryStore: carga/persistencia, lock, atomic-write
    ├─ memory.go       Remember / Recall / SmartRecall / Suggest / ListActive
    ├─ capsule.go      Modelo de cápsula + validación + TTL + visibilidad
    ├─ ranking.go      Pipeline de búsqueda: BM25, match, grafo, RRF, Render
    ├─ scoring.go      QualityScore / Importance
    ├─ graph.go        Grafo de enlaces: BuildGraph, Centrality, EgoExpand
    ├─ sessions.go     SessionTracker: heartbeats, conflictos, archivos por sesión
    ├─ ledger.go       Ledger: registro JSON-lines + rotación + resumen
    ├─ rotate.go       Compactación del ledger activo hacia archives/
    ├─ summary.go      Agregación del ledger por día/semana/mes
    ├─ reflog.go       Lectura de reflog git (para detectar cambios de rama)
    ├─ gitutil.go      CurrentBranch, HasGitDir, ProjectID
    ├─ primer.go       Primer, Stats, Diff, Backup + renderizadores
    ├─ merge.go        Consolidate / MergeCapsules / Jaccard
    ├─ health.go       Health(cwd) + RenderHealth
    ├─ tags.go         InferTags + ExtractProjectDeps (vocabulario)
    ├─ ambar.go        Formato Ámbar (texto plano) + Format
    ├─ scoring.go / ranking.go / ... (detalle abajo)
    └─ pidalive_*.go   Detección de PID vivo (Windows/Unix)
```

## La cápsula (unidad de memoria)

Toda la memoria son **cápsulas** (`Capsule`), con campos:

| Campo | Descripción |
|-------|-------------|
| `ID` | Identificador único (4 bytes aleatorios, hex). |
| `Category` | `decision`, `pattern`, `bug`, `knowledge`. |
| `Key` | Clave corta y estable para identificar/colisionar. |
| `Content` | Cuerpo de la memoria (texto). |
| `File` | Archivo del proyecto asociado (opcional). |
| `Tags` | Etiquetas (búsqueda por filtro AND). |
| `Date` | Fecha de creación `YYYY-MM-DD` (ISO). |
| `TTL` | Caducidad: `30d` (relativo) o `YYYY-MM-DD` (absoluto). |
| `Accessed` | Contador de accesos (para recencia/frecuencia). |
| `Links` | Claves enlazadas (grafo). |
| `Quality` | Calidad [0-1] calculada (ver scoring). |
| `Confidence` | Confianza según origen. |
| `LastAccessed` | Último acceso ISO (si alguno). |
| `Priority` | 0-5; 1-5 eleva la cápsula por encima del BM25 puro. |
| `PathScope` | Glob de ámbito de rutas (filtro de sesión). |
| `Origin` | `human`, `agent`, `inferred`. |
| `Status` | `active`, `obsolete`. |
| `SupersededOn` | Fecha en que fue superada (soft-delete). |

**Origen → confianza** (`ConfidenceFor`): `human` = 1.0, `agent` = 0.8,
`inferred` = 0.6.

**Ciclo de vida**:

1. `Remember` crea una cápsula nueva o **fusiona** con una existente de la
   misma clave (ver `MergeCapsules` en merge.go).
2. La cápsula es visible mientras `Status == active`, no está caducada
   (`TTL`) y, en consultas con vista temporal, su `Date` no supera el `asOf`.
3. `Forget(key, hard=false)` la marca `obsolete` (soft); `hard=true` la
   elimina del store.
4. `Resolve(key)` la restaura a `active` y limpia `SupersededOn` (revierte `Forget`).
5. `ArchiveOld` mueve a `archive` las que superan una antigüedad.
6. El acceso a una cápsula vía `Rank`/`Recall` incrementa `Accessed` y
   refresca `LastAccessed` (bump); los bumps pendientes se persisten con
   `Flush` o en la siguiente escritura.

## Persistencia y concurrencia

- `NewStore(Config)` abre (o crea) el directorio con la estructura de la
  tabla `FileSet`: `memory.<ext>`, `archive.<ext>`, `config.json`,
  `memory.lock`, `backups/`, `sessions/`.
- Carga en memoria: cápsulas activas en `s.entries` (indexadas por ID y por
  clave) y archivadas en `s.archive`.
- **Lock de archivo** (`memory.lock`): los escritores escriben
  `pid + timestamp` y bloquean el archivo durante la persistencia; si el lock
  está ocupado por un PID muerto o stale (15 s) se toma (robo). Espera máx.
  10 s. En Windows las `sharing violation` se tratan como lock ocupado.
- **Escrituras atómicas**: temporal + `rename`. Permisos `0600`.
- Los lectores comparan la `fileStamp` (mtime+size) del archivo de memoria
  contra la última cargada: si otro proceso escribió, recargan
  (`loadFile`/`adoptForeignLocked`). Esto permite **múltiples procesos**
  compartiendo el mismo directorio de memoria.
- El `vocab` (proyecto) y el `config.json` se reescriben con `writeMetaLocked`.
- Formato: `json` (por defecto) o `ambar`; `SetFormat` migra los archivos.

## Pipeline de búsqueda (`Rank`)

```
 filtro       -> match        -> (grafo)   -> puntuación -> orden -> límite
 visible      -> tokens       -> ego       -> BM25 +    -> RRF o  -> top N
 (status,     -> keywords     -> subgraph  -> match     -> linear
  ttl, asOf,     + tags AND     + seeds    -> recencia  + bumped
  dates,        + path glob              + frecuencia
  pathscope)
```

Pasos:

1. **Visibilidad**: solo cápsulas activas, no caducadas y dentro de la vista
   temporal (`AsOf`) y filtros de fecha.
2. **Match**: si hay query, `matchQuery` (BM25 no filtra; el filtrado previo
   usa tokens + keywords con normalización de acentos y `splitCamel`).
   `matchTags` aplica filtro AND de etiquetas; `pathMatch` aplica el glob de
   `PathScope` contra `SessionFiles`.
3. **Grafo** (`Graph: true`): `EgoExpand` sobre las seeds para incluir vecinos
   a `Hops` (1 o 2) como candidatos marcados con `Dist`.
4. **Puntuación**: por defecto se usa **combinación lineal** de BM25 +
   centralidad + importancia; con `RRF: true` se fusionan por rangos los
   rankings de BM25, match de keywords y frecuencia/recencia. Las cápsulas con
   `Priority > 0` se mueven al frente. Los seed (coincidencia directa)
   conservan `IsSeed: true`.
5. **Bump**: los resultados devueltos incrementan su contador de acceso.

Funciones relevantes: `BM25Scores` (k1=1.5, b=0.75), `rrfScore`, `rankOf`,
`Render(rs, budget)` con presupuestos `tiny`/`normal`/`deep`.

### Scoring (`scoring.go`)

- `QualityScore(c, now)`: mezcla ponderada de
  - longitud del contenido (0.30),
  - tags presentes (0.30),
  - enlaces (0.15),
  - frecuencia de acceso (0.10, tope),
  - recencia (0.10),
  - especificidad/origen (0.10),
  - con un **cap de estancamiento** (0.20) si no se ha accedido en 90 días.
- `Importance(c, now) = recencia*0.6 + frecuencia*0.4` (ambas normalizadas a
  [0,1]).

## Grafo de enlaces (`graph.go`)

- `BuildGraph` conecta cápsulas cuyos `Links` apuntan a otras claves presentes,
  y además enlaza por **tags compartidos** (arista con peso `sharedTagCount`).
- `LinkKey` normaliza una clave para usarla como nodo.
- `Centrality` = grado normalizado; se cachea por día (`cachedCentral`) y se
  invalida con `bumpGraph` al mutar el store.
- `EgoExpand(seeds, hops, visible)` recorre BFS hasta `hops` saltos y devuelve
  `map[key]dist` para el paso de expansión del pipeline.
- `ShortestPath(a, b)` (BFS) para responder "cómo se relacionan dos memorias".

## Sesiones (`sessions.go`)

El `SessionTracker` mantiene un fichero de **heartbeat por sesión** en
`<dir>/sessions/<id>.json`:

```json
{ "id": "...", "agent": "...", "pid": 1234, "branch": "main",
  "started_at": "...", "last_seen": "...", "files": {"path": "iso"}, "ended": false }
```

- `Touch(file)` / `Heartbeat(file, ended)` actualizan el estado (tope de 200
  archivos por sesión; a partir de ahí se podan los más antiguos).
- `ActiveSessions()` lista las sesiones vivas (TTL de 10 min: si
  `last_seen` es más viejo y el PID está muerto, se considera terminada).
- `Conflicts()` detecta **archivos tocados por dos o más sesiones vivas**
  (posible edición concurrente).
- `SessionFiles()` devuelve el conjunto de archivos tocados por *esta* sesión,
  que el pipeline de búsqueda usa para filtrar por `PathScope`.
- `Prune()` elimina heartbeats huérfanos.

## Ledger (`ledger.go`, `rotate.go`, `summary.go`)

El ledger es un registro de actividad **JSON-lines** (v2, `V:2`) con rotación
automática:

- `NewLedger(cwd, ...)` localiza la raíz del proyecto (`DetectRoot`, hasta 10
  niveles hacia arriba buscando `.git`/`go.mod`/etc.) y elige
  `~/.goulm/ledger/<Proyecto>/` salvo que se use `WithHome` para aislarlo.
- `Append*`: `AppendTool`, `AppendEdit`, `AppendCommit`, `AppendError`,
  `AppendMemory`, `AppendSessionStart/End`, `AppendMilestone`,
  `AppendApproval`. Cada evento lleva `TS`, `Type`, `Action`, `Session`,
  y campos opcionales (`Path`, `Detail`, `Hash`, `Risk`, `Status`,
  `Approved`, `Tokens`, `CostUSD`, `DurationMs`, `Turn`, `Test`).
- **Rotación**: al escribir, si el archivo activo supera `Window` (200 eventos
  por defecto) o 48 KiB, `CompactNow` mueve el exceso a
  `archives/YYYY-MM.json` agrupado por mes.
- `Tail(n, type, includeHistory)` devuelve los últimos n eventos (activo y,
  opcionalmente, históricos). `Stats()` resume el estado.
- `Summary()` agrega por día/semana/mes: commits, ediciones (archivos
  únicos), errores, tests, memorias por categoría, milestones, coste y
  duración; respeta `SummaryBudget`.
- `Export(since, to)` devuelve el rango como texto.
- Sanitización: `sanitizeDetail` enmascara secretos y corta a 300 caracteres.
- Enmascarado por variables de entorno: `GOULM_LEDGER=off` deshabilita;
  la sesión se obtiene de `GOULM_SESSION_ID` si está definida.

`LedgerHook` (en `pkg/tools`) es el puente entre las tools y el ledger:
observa `OnToolStart`/`OnToolResult` (vía `EventSink` o envolviendo `Execute`)
y registra `tool`/`approval`/`session`/`milestone` en un **writer asíncrono**
con cola y drops contados (`Stats()`).

## Capa de tools (`pkg/tools`)

`Registry` es un registro plano con los mismos defaults que el agente de
Goulm (timeout 30 s, categoría `inspect`, riesgo `low` o `high` según
`RequiresApproval`). `RegisterMemoryTools` registra 11 tools de memoria y
`RegisterLedgerTools` las 2 de ledger. Cada tool recibe `(ctx, input string)`
y devuelve `(string, error)`, donde `input` es un JSON de argumentos. Ver
`API.md` para los parámetros de cada tool.

## Flujo del demo (`cmd/demo`)

1. Resuelve `cwd` y `-dir` (por defecto `~/.goulm-memory/<ProjectID>`).
2. Abre el store (JSON), inyecta vocabulario del proyecto, abre un
   `SessionTracker`, crea el ledger aislado y el `LedgerHook`.
3. Registra las 13 tools en un `Registry`.
4. Ejecuta el subcomando pedido a través del Registry (misma ruta que un
   agente), con exit codes 0/1/2.