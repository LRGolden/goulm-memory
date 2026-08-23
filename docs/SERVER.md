# Server HTTP

goulm-memory incluye un HTTP server minimal que expone las operaciones
esenciales del store via endpoints JSON. Diseñado para que cualquier
lenguaje (Python, TypeScript, Ruby, shell) pueda hablar con el store.

## Levantar el server

```bash
go run ./cmd/serve                          # default :8080
go run ./cmd/serve -addr :9090 -dir /path   # custom
go run ./cmd/serve -api-key "mi-clave"      # con autenticacion
```

### Flags

| Flag | Default | Descripcion |
|------|---------|-------------|
| `-addr` | `:8080` | Direccion del servidor (host:port) |
| `-dir` | `~/.goulm-memory/<proyecto>` | Directorio de persistencia |
| `-cors` | `http://localhost:*` | Origen CORS permitido (`*` para todos) |
| `-api-key` | (sin auth) | API key para autenticacion. Tambien: `GOULM_API_KEY` |

## Autenticacion

Cuando se configura `-api-key` o `GOULM_API_KEY`, todos los endpoints
(excepto `/healthz`) requieren la key en el header:

```bash
# Opcion 1: X-API-Key
curl -H "X-API-Key: mi-clave" http://localhost:8080/api/v1/stats

# Opcion 2: Authorization Bearer
curl -H "Authorization: Bearer mi-clave" http://localhost:8080/api/v1/stats
```

Si no se configura key, la auth esta deshabilitada (backward compatible).

El endpoint `/healthz` siempre funciona sin auth (liveness check).

## Endpoints

### Liveness

```
GET /healthz → {"status":"ok"}
```

### Remember

```bash
curl -X POST http://localhost:8080/api/v1/remember \
  -H "Content-Type: application/json" \
  -H "X-API-Key: mi-clave" \
  -d '{"key":"auth-jwt","category":"decision","content":"Usar JWT para auth","tags":["auth"]}'
```

### Recall

```bash
curl -X POST http://localhost:8080/api/v1/recall \
  -H "Content-Type: application/json" \
  -H "X-API-Key: mi-clave" \
  -d '{"q":"auth","limit":5}'
```

### Suggest

```bash
curl -X POST http://localhost:8080/api/v1/suggest \
  -H "Content-Type: application/json" \
  -H "X-API-Key: mi-clave" \
  -d '{"context":"estamos hablando de login","limit":3}'
```

### Stats

```bash
curl -H "X-API-Key: mi-clave" http://localhost:8080/api/v1/stats
```

### Health

```bash
curl -H "X-API-Key: mi-clave" http://localhost:8080/api/v1/health
```

### Forget

```bash
curl -X POST http://localhost:8080/api/v1/forget \
  -H "Content-Type: application/json" \
  -H "X-API-Key: mi-clave" \
  -d '{"key":"auth-jwt","hard":false}'
```

### Resolve

```bash
curl -X POST http://localhost:8080/api/v1/resolve \
  -H "Content-Type: application/json" \
  -H "X-API-Key: mi-clave" \
  -d '{"key":"auth-jwt"}'
```

### Pin

```bash
curl -X POST http://localhost:8080/api/v1/pin \
  -H "Content-Type: application/json" \
  -H "X-API-Key: mi-clave" \
  -d '{"key":"auth-jwt","priority":5}'
```

### Backup

```bash
curl -X POST http://localhost:8080/api/v1/backup \
  -H "X-API-Key: mi-clave"
```

### Archive

```bash
curl -X POST http://localhost:8080/api/v1/archive \
  -H "X-API-Key: mi-clave"
```

### Consolidate

```bash
curl -X POST http://localhost:8080/api/v1/consolidate \
  -H "X-API-Key: mi-clave"
```

### List capsules

```bash
curl -H "X-API-Key: mi-clave" http://localhost:8080/api/v1/capsules
```

## Respuesta

Todos los endpoints devuelven JSON:

```json
{"result": "..."}
```

En caso de error:

```json
{"error": "descripcion del error"}
```

## Ejemplo: Python

```python
import requests

BASE = "http://localhost:8080"
HEADERS = {"X-API-Key": "mi-clave"}

# Recordar
requests.post(f"{BASE}/api/v1/remember", headers=HEADERS, json={
    "key": "auth-jwt",
    "category": "decision",
    "content": "Usar JWT para auth"
})

# Buscar
resp = requests.post(f"{BASE}/api/v1/recall", headers=HEADERS, json={"q": "auth", "limit": 5})
print(resp.json()["result"])

# Estado
resp = requests.get(f"{BASE}/api/v1/stats", headers=HEADERS)
print(resp.json()["result"])
```

## Ejemplo: TypeScript

```typescript
const BASE = "http://localhost:8080";
const HEADERS = { "Content-Type": "application/json", "X-API-Key": "mi-clave" };

// Recordar
await fetch(`${BASE}/api/v1/remember`, {
  method: "POST",
  headers: HEADERS,
  body: JSON.stringify({
    key: "auth-jwt",
    category: "decision",
    content: "Usar JWT para auth"
  })
});

// Buscar
const resp = await fetch(`${BASE}/api/v1/recall`, {
  method: "POST",
  headers: HEADERS,
  body: JSON.stringify({ q: "auth", limit: 5 })
});
const { result } = await resp.json();
console.log(result);
```

## CORS

El server incluye headers CORS basicos (`Access-Control-Allow-Origin: *`).
Para produccion, configura un reverse proxy (nginx, caddy) con CORS
especificos.

## Mas informacion

- [QUICKSTART.md](QUICKSTART.md) — Primeros pasos
- [ADVANCED.md](ADVANCED.md) — Integracion avanzada
