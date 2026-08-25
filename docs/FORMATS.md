# File formats

Description of the formats that `goulm-memory` writes to disk. Useful for
manual inspection, debugging, migrations and external tools.

## Store directory layout

`MemoryStore` writes to a directory (by default
`~/.goulm-memory/<ProjectID>` or the `Dir` of `Config`). Depending on the
format (`json` or `amber`), the data file extension changes
(`.json` vs `.amb`):

```
<dir>/
├── memory.json        # active capsules (JSON) — or memory.amb (Amber)
├── archive.json       # archived capsules — or archive.amb
├── config.json        # store metadata (format, project, vocabulary)
├── memory.lock        # write lock (pid + timestamp)
├── backups/           # store backups (memory-<stamp>.json)
└── sessions/          # session heartbeats (<session-id>.json)
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

- `format`: `"json"` or `"amber"` (format of the data files).
- `project`: declared project name.
- `max_entries`: active capsule limit (pruned on start/save).
- `vocab`: project vocabulary for tag inference
  (`map[tag]words`); built by `ExtractProjectDeps`.

## JSON format

Files `memory.json` and `archive.json` with this schema (indented 2
spaces):

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
      "content": "Use JWT for authentication",
      "file": "cmd/server/main.go",
      "tags": ["auth", "security"],
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

Fields with `omitempty` (absent if zero): `file`, `tags`, `ttl`,
`links`, `last_accessed`, `path_scope`, `superseded_on`. `id`, `category`,
`key`, `content`, `date`, `quality`, `confidence`, `origin` and `status`
always present.

## Amber format

Line-oriented plain text format (extension `.amb`), designed to be
diff-friendly and terminal-readable. Escapes `\`, `|`, `\n` and `\r` with
backslash.

```
v:1|project:goulm-cli|updated:2026-08-20T10:30:00Z|count:1
~
id:a1b2c3d4|key:auth-jwt|cat:decision|date:2026-08-19|tags:auth;security|ttl:30d|acc:3|q:0.72|c:0.95|origin:human|status:active|pri:3
content>Use JWT for authentication
file>cmd/server/main.go
links>auth-session
scope>cmd/**
last>2026-08-20T09:00:00Z
```

- **Header** (line 1): `v:<version>|project:<id>|updated:<ISO>|count:<n>`.
- Each capsule starts with a `~` line.
- Attributes line: `key:value` pairs separated by `|`. Keys: `id`,
  `key`, `cat`, `date`, `tags` (`;`-separated), `ttl`, `acc`, `q`, `c`,
  `origin`, `status`, `pri`.
- Optional body lines: `content>`, `file>`, `links>` (`;`-separated),
  `scope>`, `last>`, `superseded>`.
- The parser (`UnmarshalAmbar`) is **tolerant**: unknown fields are
  ignored and missing fields use defaults (`knowledge`/`agent`/`active`).
- Default-value attributes are omitted when serializing (`origin:agent`,
  `status:active`, `pri:0`, `acc:0`).

## Ledger format

The ledger is **JSON-lines** (one line = one event), with automatic rotation.
Event version: `v:2` (parser accepts v1 and v2).

```
<ledger-home>/<Project>/
├── ledger.jsonl       # active file (event window)
└── archives/
    └── 2026-08.jsonl  # old events grouped by month
```

### Event

```json
{"v":2,"ts":"2026-08-20T09:00:00Z","type":"tool","action":"memory_remember","session":"s-abc","path":"auth-jwt","detail":"decision","risk":"low","status":"ok","duration_ms":12,"test":false}
```

Fields:

| Field | Required | Description |
|-------|----------|-------------|
| `v` | yes | Format version (1 or 2). |
| `ts` | yes | RFC3339 timestamp. |
| `type` | yes | `tool`, `edit`, `commit`, `error`, `memory`, `session`, `milestone`, `approval`, `branch`, `checkout`, `test`, `system`. |
| `action` | no | Concrete action (e.g. `memory_remember`, `start`, `end`, `mark`). |
| `session` | no | Session ID. |
| `path` | no | Path (normalized relative to project root) or memory key. |
| `detail` | no | Detail (truncated to 300 runes with secrets masked). |
| `hash` | no | Commit hash (8+ hex chars). |
| `risk` | no | `low`/`medium`/`high`/`critical`. |
| `status` | no | `ok`, `error`, `denied`, `blocked`. |
| `approved` | no | `yes`, `no`, `na`. |
| `tokens` | no | Token usage. |
| `cost_usd` | no | Cost in USD. |
| `duration_ms` | no | Duration in ms. |
| `turn` | no | Turn number. |
| `test` | no | Mark test session. |
| `id` | no | Additional identifier. |

### Rotation

- `DefaultLedgerWindow` = 200 events in the active file (configurable with
  `WithWindow`).
- On write, if the active file exceeds the window **or** 48 KiB
  (`defaultCompactSizeHint`), it is compacted: excess events are grouped
  by month and appended to `archives/YYYY-MM.json`.
- `GOULM_LEDGER=off` disables the ledger via environment variable.

## Session format

Each live session is a JSON file at `<dir>/sessions/<id>.json`:

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

- `id`: from `GOULM_SESSION_ID` or auto-generated.
- `files`: map of `path → ISO of last touch`; capped at 200 files per
  session (`maxHeartbeatFiles`), pruning the oldest when exceeded.
- Session TTL: 10 min (`SessionTTL`). A session is considered terminated if
  its `last_seen` exceeds the TTL **and** its PID is not alive.
- `Prune` removes orphaned heartbeats.

## Backups

`memory_backup`/`Backup()` copy the full store state to
`<dir>/backups/memory-<stamp>.json` (or `.amb`), with `stamp` in
`YYYY-MM-DDTHH-MM-SS` UTC. Kept up to `MaxBackups` (default 10),
pruning the oldest.

## Format migration

`SetFormat(FormatAmbar)` (or `FormatJSON`) rewrites `memory` and `archive` in the
new format (extension changes to `.amb`/`.json`) and updates `config.json`.
No data is shared between formats: the entire current load is migrated.