# Formatos de archivo

Descripción de los formatos que escribe `goulm-memory` en disco. Útil para
inspección manual, debugging, migraciones y herramientas externas.

## Layout de directorio del store

`MemoryStore` escribe en un directorio (por defecto
`~/.goulm-memory/<ProjectID>` o el `Dir` de `Config`). Dependiendo del
formato (`json` o `ambar`), la extensión de los archivos de datos cambia
(`.json` vs `.amb`):

```
<dir>/
├── memory.json        # cápsulas activas (JSON) — o memory.amb (Ámbar)
├── archive.json       # cápsulas archivadas — o archive.amb
├── config.json        # metadatos del store (formato, proyecto, vocabulario)
├── memory.lock        # lock de escritura (pid + timestamp)
├── backups/           # backups del store (memory-<stamp>.json)
└── sessions/          # heartbeats de sesión (<session-id>.json)
```

## `config.json`

```json
{
  "format": "json",
  "project": "goulm-cli",
  "max_entries": 100,
  "vocab": { "golang": ["go", "go.mod"], "react": ["react", "tsx"] }
}
```

- `format`: `"json"` o `"ambar"` (formato de los archivos de datos).
- `project`: nombre declarado del proyecto.
- `max_entries`: límite de cápsulas activas (podado al arrancar/guardar).
- `vocab`: vocabulario del proyecto para inferencia de tags
  (`map[tag]palabras`); lo construye `ExtractProjectDeps`.

## Formato JSON

Archivos `memory.json` y `archive.json` con este esquema (indentado 2
espacios):

```json
{
  "version": 1,
  "project": "goulm-cli",
  "updated": "2026-08-20T10:30:00Z",
  "capsules": [
    {
      "id": "a1b2c3d4",
      "category": "decision",
      "key": "auth-jwt",
      "content": "Usar JWT para autenticación",
      "file": "cmd/server/main.go",
      "tags": ["auth", "seguridad"],
      "date": "2026-08-19",
      "ttl": "30d",
      "accessed": 3,
      "links": ["auth-session"],
      "quality": 0.72,
      "confidence": 0.95,
      "last_accessed": "2026-08-20T09:00:00Z",
      "priority": 3,
      "path_scope": "cmd/**",
      "origin": "human",
      "status": "active"
    }
  ]
}
```

Campos con `omitempty` (ausentes si vienen a cero): `file`, `tags`, `ttl`,
`links`, `last_accessed`, `path_scope`, `superseded_on`. `id`, `category`,
`key`, `content`, `date`, `quality`, `confidence`, `origin` y `status`
siempre presentes.

## Formato Ámbar

Formato de texto plano orientado a líneas (extensión `.amb`), pensado para
ser diff-friendly y legible en terminal. Escapa `\`, `|`, `\n` y `\r` con
backslash.

```
v:1|project:goulm-cli|updated:2026-08-20T10:30:00Z|count:1
~
id:a1b2c3d4|key:auth-jwt|cat:decision|date:2026-08-19|tags:auth;seguridad|ttl:30d|acc:3|q:0.72|c:0.95|origin:human|status:active|pri:3
content>Usar JWT para autenticación
file>cmd/server/main.go
links>auth-session
scope>cmd/**
last>2026-08-20T09:00:00Z
```

- **Cabecera** (línea 1): `v:<version>|project:<id>|updated:<ISO>|count:<n>`.
- Cada cápsula empieza con una línea `~`.
- Línea de atributos: pares `clave:valor` separados por `|`. Claves: `id`,
  `key`, `cat`, `date`, `tags` (`;`-separadas), `ttl`, `acc`, `q`, `c`,
  `origin`, `status`, `pri`.
- Líneas opcionales de cuerpo: `content>`, `file>`, `links>` (`;`-separadas),
  `scope>`, `last>`, `superseded>`.
- El parser (`UnmarshalAmbar`) es **tolerante**: campos desconocidos se
  ignoran y los faltantes usan defaults (`knowledge`/`agent`/`active`).
- Atributos con valor por defecto se omiten al serializar (`origin:agent`,
  `status:active`, `pri:0`, `acc:0`).

## Formato del ledger

El ledger es **JSON-lines** (una línea = un evento), con rotación automática.
Versión del evento: `v:2` (el parser acepta v1 y v2).

```
<ledger-home>/<Proyecto>/
├── ledger.jsonl       # archivo activo (ventana de eventos)
└── archives/
    └── 2026-08.jsonl  # eventos antiguos agrupados por mes
```

### Evento

```json
{"v":2,"ts":"2026-08-20T09:00:00Z","type":"tool","action":"memory_remember","session":"s-abc","path":"auth-jwt","detail":"decision","risk":"low","status":"ok","duration_ms":12,"test":false}
```

Campos:

| Campo | Obligatorio | Descripción |
|-------|-------------|-------------|
| `v` | sí | Versión del formato (1 o 2). |
| `ts` | sí | Timestamp RFC3339. |
| `type` | sí | `tool`, `edit`, `commit`, `error`, `memory`, `session`, `milestone`, `approval`, `branch`, `checkout`, `test`, `system`. |
| `action` | no | Acción concreta (p. ej. `memory_remember`, `start`, `end`, `mark`). |
| `session` | no | ID de sesión. |
| `path` | no | Ruta (normalizada relativa a la raíz del proyecto) o clave de memoria. |
| `detail` | no | Detalle (cortado a 300 runas y con secretos enmascarados). |
| `hash` | no | Hash de commit (8+ caracteres hex). |
| `risk` | no | `low`/`medium`/`high`/`critical`. |
| `status` | no | `ok`, `error`, `denied`, `blocked`. |
| `approved` | no | `yes`, `no`, `na`. |
| `tokens` | no | Uso de tokens. |
| `cost_usd` | no | Coste en USD. |
| `duration_ms` | no | Duración en ms. |
| `turn` | no | Número de turno. |
| `test` | no | Marca sesión de test. |
| `id` | no | Identificador adicional. |

### Rotación

- `DefaultLedgerWindow` = 200 eventos en el archivo activo (configurable con
  `WithWindow`).
- Al escribir, si el activo supera la ventana **o** 48 KiB
  (`defaultCompactSizeHint`), se compacta: los eventos excedentes se agrupan
  por mes y se anexan a `archives/YYYY-MM.json`.
- `GOULM_LEDGER=off` deshabilita el ledger por entorno.

## Formato de sesiones

Cada sesión viva es un archivo JSON en `<dir>/sessions/<id>.json`:

```json
{
  "id": "s-abc",
  "agent": "goulm",
  "pid": 1234,
  "branch": "main",
  "started_at": "2026-08-20T09:00:00Z",
  "last_seen": "2026-08-20T09:05:00Z",
  "files": { "cmd/server/main.go": "2026-08-20T09:04:00Z" },
  "ended": false
}
```

- `id`: de `GOULM_SESSION_ID` o autogenerado.
- `files`: mapa `path → ISO de último toque`; tope de 200 archivos por
  sesión (`maxHeartbeatFiles`), podando los más antiguos al exceder.
- TTL de sesión: 10 min (`SessionTTL`). Una sesión se considera terminada si
  su `last_seen` supera el TTL **y** su PID no está vivo.
- `Prune` elimina heartbeats huérfanos.

## Backups

`memory_backup`/`Backup()` copian el estado completo del store a
`<dir>/backups/memory-<stamp>.json` (o `.amb`), con `stamp` en
`YYYY-MM-DDTHH-MM-SS` UTC. Se conservan hasta `MaxBackups` (default 10),
podando los más antiguos.

## Migración de formato

`SetFormat(FormatAmbar)` (o `FormatJSON`) reescribe `memory` y `archive` en el
nuevo formato (la extensión cambia a `.amb`/`.json`) y actualiza `config.json`.
No hay datos compartidos entre formatos: se migra la carga actual completa.