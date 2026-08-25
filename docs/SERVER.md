# HTTP Server

goulm-memory includes a minimal HTTP server that exposes the essential
store operations via JSON endpoints. Designed so that any language
(Python, TypeScript, Ruby, shell) can talk to the store.

## Starting the server

```bash
go run ./cmd/serve                          # default :8080
go run ./cmd/serve -addr :9090 -dir /path   # custom
go run ./cmd/serve -api-key "my-key"      # with authentication
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-addr` | `:8080` | Server address (host:port) |
| `-dir` | `~/.goulm-memory/<project>` | Persistence directory |
| `-cors` | `http://localhost:*` | Allowed CORS origin (`*` for all) |
| `-api-key` | (no auth) | API key for authentication. Also: `GOULM_API_KEY` |

## Authentication

When `-api-key` or `GOULM_API_KEY` is configured, all endpoints
(except `/healthz`) require the key in the header:

```bash
# Option 1: X-API-Key
curl -H "X-API-Key: my-key" http://localhost:8080/api/v1/stats

# Option 2: Authorization Bearer
curl -H "Authorization: Bearer my-key" http://localhost:8080/api/v1/stats
```

If no key is configured, authentication is disabled (backward compatible).

The `/healthz` endpoint always works without auth (liveness check).

## Endpoints

### Liveness

```
GET /healthz → {"status":"ok"}
```

### Remember

```bash
curl -X POST http://localhost:8080/api/v1/remember \
  -H "Content-Type: application/json" \
  -H "X-API-Key: my-key" \
  -d '{"key":"auth-jwt","category":"decision","content":"Use JWT for auth","tags":["auth"]}'
```

### Recall

```bash
curl -X POST http://localhost:8080/api/v1/recall \
  -H "Content-Type: application/json" \
  -H "X-API-Key: my-key" \
  -d '{"q":"auth","limit":5}'
```

### Suggest

```bash
curl -X POST http://localhost:8080/api/v1/suggest \
  -H "Content-Type: application/json" \
  -H "X-API-Key: my-key" \
  -d '{"context":"we are talking about login","limit":3}'
```

### Stats

```bash
curl -H "X-API-Key: my-key" http://localhost:8080/api/v1/stats
```

### Health

```bash
curl -H "X-API-Key: my-key" http://localhost:8080/api/v1/health
```

### Forget

```bash
curl -X POST http://localhost:8080/api/v1/forget \
  -H "Content-Type: application/json" \
  -H "X-API-Key: my-key" \
  -d '{"key":"auth-jwt","hard":false}'
```

### Resolve

```bash
curl -X POST http://localhost:8080/api/v1/resolve \
  -H "Content-Type: application/json" \
  -H "X-API-Key: my-key" \
  -d '{"key":"auth-jwt"}'
```

### Pin

```bash
curl -X POST http://localhost:8080/api/v1/pin \
  -H "Content-Type: application/json" \
  -H "X-API-Key: my-key" \
  -d '{"key":"auth-jwt","priority":5}'
```

### Backup

```bash
curl -X POST http://localhost:8080/api/v1/backup \
  -H "X-API-Key: my-key"
```

### Archive

```bash
curl -X POST http://localhost:8080/api/v1/archive \
  -H "X-API-Key: my-key"
```

### Consolidate

```bash
curl -X POST http://localhost:8080/api/v1/consolidate \
  -H "X-API-Key: my-key"
```

### List capsules

```bash
curl -H "X-API-Key: my-key" http://localhost:8080/api/v1/capsules
```

## Response

All endpoints return JSON:

```json
{"result": "..."}
```

In case of error:

```json
{"error": "error description"}
```

## Example: Python

```python
import requests

BASE = "http://localhost:8080"
HEADERS = {"X-API-Key": "my-key"}

# Remember
requests.post(f"{BASE}/api/v1/remember", headers=HEADERS, json={
    "key": "auth-jwt",
    "category": "decision",
    "content": "Use JWT for auth"
})

# Search
resp = requests.post(f"{BASE}/api/v1/recall", headers=HEADERS, json={"q": "auth", "limit": 5})
print(resp.json()["result"])

# Stats
resp = requests.get(f"{BASE}/api/v1/stats", headers=HEADERS)
print(resp.json()["result"])
```

## Example: TypeScript

```typescript
const BASE = "http://localhost:8080";
const HEADERS = { "Content-Type": "application/json", "X-API-Key": "my-key" };

// Remember
await fetch(`${BASE}/api/v1/remember`, {
  method: "POST",
  headers: HEADERS,
  body: JSON.stringify({
    key: "auth-jwt",
    category: "decision",
    content: "Use JWT for auth"
  })
});

// Search
const resp = await fetch(`${BASE}/api/v1/recall`, {
  method: "POST",
  headers: HEADERS,
  body: JSON.stringify({ q: "auth", limit: 5 })
});
const { result } = await resp.json();
console.log(result);
```

## CORS

The server includes basic CORS headers (`Access-Control-Allow-Origin: *`).
For production, configure a reverse proxy (nginx, caddy) with specific
CORS settings.

## More information

- [QUICKSTART.md](QUICKSTART.md) — Getting started
- [ADVANCED.md](ADVANCED.md) — Advanced integration
