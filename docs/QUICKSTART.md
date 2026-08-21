# Quickstart

Guia rapida para empezar a usar goulm-memory.

## Instalacion

```bash
go get github.com/LRGolden/goulm-memory
```

## 1. Crear el store

```go
import (
    "path/filepath"
    "github.com/LRGolden/goulm-memory/pkg/memory"
)

home, _ := os.UserHomeDir()
store, err := memory.NewStore(memory.Config{
    Dir:        filepath.Join(home, ".goulm-memory", "my-app"),
    Format:     memory.FormatJSON,
    Project:    "my-app",
    MaxEntries: 100,
    MaxBackups: 10,
})
```

## 2. Recordar algo

```go
res, err := store.Remember(memory.RememberOptions{
    Key:      "auth-jwt",
    Category: memory.CategoryDecision,
    Content:  "Usar JWT para autenticacion. Refresh token con TTL de 7d.",
    Tags:     []string{"auth", "seguridad", "jwt"},
    Origin:   memory.OriginHuman,
    Priority: 3,
})
// res.Created = true (nueva) o false (fusionada con existente)
```

Categorias disponibles: `CategoryDecision`, `CategoryPattern`, `CategoryBug`, `CategoryKnowledge`.

## 3. Buscar

```go
// Busqueda basica
ranked, err := store.Recall("autenticacion", &memory.Query{Limit: 5})
for _, r := range ranked {
    fmt.Printf("%.3f  %s/%s\n", r.Score, r.Capsule.Category, r.Capsule.Key)
}

// Busqueda con filtros
ranked, err := store.Recall("jwt", &memory.Query{
    Category: memory.CategoryDecision,
    Tags:     []string{"seguridad"},
    Limit:    3,
})

// Sugerencias sobre un contexto (sin query explicita)
sugs, err := store.Suggest("estamos hablando de login", 3)
```

## 4. Ver estado

```go
stats, err := store.Stats()
fmt.Printf("Total: %d capsulas\n", stats.Total)
fmt.Printf("Por categoria: %v\n", stats.ByCategory)
```

## 5. Olvidar y restaurar

```go
// Soft delete (queda como obsolete, se puede restaurar)
store.Forget("auth-jwt", false)

// Restaurar
store.Resolve("auth-jwt")

// Hard delete (eliminacion permanente)
store.Forget("auth-jwt", true)
```

## 6. Persistir

```go
store.Flush() // forzar escritura a disco
```

## Ejemplo completo

```go
package main

import (
    "fmt"
    "os"
    "path/filepath"

    "github.com/LRGolden/goulm-memory/pkg/memory"
)

func main() {
    home, _ := os.UserHomeDir()
    dir := filepath.Join(home, ".goulm-memory", "demo")

    store, err := memory.NewStore(memory.Config{
        Dir:     dir,
        Project: "demo",
    })
    if err != nil {
        panic(err)
    }

    // Recordar
    store.Remember(memory.RememberOptions{
        Key:      "auth-jwt",
        Category: memory.CategoryDecision,
        Content:  "Usar JWT para autenticacion",
        Tags:     []string{"auth"},
    })

    // Buscar
    ranked, _ := store.Recall("auth", &memory.Query{Limit: 5})
    fmt.Println(memory.Render(ranked, memory.BudgetNormal))

    // Estado
    stats, _ := store.Stats()
    fmt.Println(memory.RenderStats(stats))

    store.Flush()
}
```

## Siguientes pasos

- [API.md](API.md) — Referencia completa de la API
- [ADVANCED.md](ADVANCED.md) — Sessions, ledger, graph, tools
