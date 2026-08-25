# Integracion de Embeddings

goulm-memory soporta busqueda semantica via embeddings. La libreria no
importa ningun proveedor -- el usuario trae el suyo via la interfaz
`EmbeddingProvider`.

## Que es un EmbeddingProvider

Una interfaz Go con dos metodos:

```go
type EmbeddingProvider interface {
    Embed(ctx context.Context, text string) ([]float64, error)
    Dimension() int
}
```

Las implementaciones deben ser seguras para uso concurrente por multiples
goroutines (el store puede llamar Embed desde multiples Recall simultaneos).

El contexto permite cancelar la operacion si el proveedor tarda demasiado
(timeout de 5s por defecto en VectorScores).

El usuario la implementa. La libreria la usa para:
1. Buscar por similitud coseno en `Rank` (automatico si el provider esta configurado)
2. Validar dimension de embeddings almacenados vs provider actual

## Ejemplo: OpenAI

```go
package main

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
)

type OpenAIEmbedder struct {
    APIKey string
}

func (e *OpenAIEmbedder) Embed(ctx context.Context, text string) ([]float64, error) {
    body, _ := json.Marshal(map[string]interface{}{
        "model": "text-embedding-3-small",
        "input": text,
    })

    req, _ := http.NewRequestWithContext(ctx, "POST",
        "https://api.openai.com/v1/embeddings",
        bytes.NewBuffer(body))
    req.Header.Set("Authorization", "Bearer "+e.APIKey)
    req.Header.Set("Content-Type", "application/json")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var result struct {
        Data []struct {
            Embedding []float64 `json:"embedding"`
        } `json:"data"`
    }
    json.NewDecoder(resp.Body).Decode(&result)

    if len(result.Data) == 0 {
        return nil, fmt.Errorf("sin embeddings")
    }
    return result.Data[0].Embedding, nil
}

func (e *OpenAIEmbedder) Dimension() int { return 1536 }
```

## Ejemplo: modelo local (ollama)

```go
type OllamaEmbedder struct {
    Model string // "nomic-embed-text"
}

func (e *OllamaEmbedder) Embed(ctx context.Context, text string) ([]float64, error) {
    body, _ := json.Marshal(map[string]interface{}{
        "model": e.Model,
        "input": text,
    })

    req, _ := http.NewRequestWithContext(ctx, "POST",
        "http://localhost:11434/api/embed",
        bytes.NewBuffer(body))
    req.Header.Set("Content-Type", "application/json")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var result struct {
        Embeddings [][]float64 `json:"embeddings"`
    }
    json.NewDecoder(resp.Body).Decode(&result)

    if len(result.Embeddings) == 0 {
        return nil, fmt.Errorf("sin embeddings")
    }
    return result.Embeddings[0], nil
}

func (e *OllamaEmbedder) Dimension() int { return 768 }
```

## Configurar el store

```go
store, _ := memory.NewStore(memory.Config{
    Dir:     dir,
    Project: "my-app",
})

// Configurar el provider
store.SetEmbedder(&OpenAIEmbedder{APIKey: "sk-..."})
```

## Pre-calcul embeddings (recomendado)

Para evitar llamadas HTTP dentro del lock de `Remember`, pre-calcula el
embedding y pasalo en las opciones:

```go
ctx := context.Background()
emb, _ := embedder.Embed(ctx, "Usar JWT para auth")

res, _ := store.Remember(memory.RememberOptions{
    Key:       "auth-jwt",
    Category:  memory.CategoryDecision,
    Content:   "Usar JWT para auth",
    Embedding: emb, // pre-calculado, fuera del lock
})
```

## Busqueda semantica

Una vez configurado el provider, la busqueda vectorial es automatica:

```go
// BM25 + vector similarity (peso 0.3)
ranked, _ := store.Recall("autenticacion", &memory.Query{Limit: 5})

// Con RRF (fusion de rangos): BM25 + vector como rankers separados
ranked, _ = store.Recall("autenticacion", &memory.Query{
    Limit: 5,
    RRF:   true,
})
```

Sin provider configurado, el pipeline es identico al de v0.3.x.

## Validacion de dimension

El store valida automaticamente que los embeddings almacenados coincidan
con la dimension del provider actual. Si una capsula tiene un embedding
de dimension diferente, se salta en la busqueda vectorial (degradation
graceful, no error).

El campo `EmbeddingDim` en la capsula registra la dimension del provider
al momento de almacenar.

## Formato Ambar

Los embeddings se persisten como `embedding>0.12,0.34,...` en formato Ambar.
Archivos viejos sin embedding se leen correctamente (el campo queda nil).

## Costo en almacenamiento

Cada embedding de 1536 dimensiones (OpenAI text-embedding-3-small) ocupa:

| Formato | Por capsula | 100 capsulas | 1000 capsulas |
|---------|-------------|--------------|---------------|
| Memoria | ~12 KB | ~1.2 MB | ~12 MB |
| JSON | ~27 KB | ~2.7 MB | ~27 MB |
| Ambar | ~18 KB | ~1.8 MB | ~18 MB |

Para reducir el costo:
- Usar modelos de menor dimension (384 en vez de 1536)
- El default `MaxEntries: 100` limita el crecimiento

## Mas informacion

- [API.md](API.md) — Referencia de `EmbeddingProvider`
- [ADVANCED.md](ADVANCED.md) — Integracion avanzada
