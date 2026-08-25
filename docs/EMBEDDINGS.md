# Embeddings Integration

goulm-memory supports semantic search via embeddings. The library does not
import any provider -- the user brings their own via the
`EmbeddingProvider` interface.

## What is an EmbeddingProvider

A Go interface with two methods:

```go
type EmbeddingProvider interface {
    Embed(ctx context.Context, text string) ([]float64, error)
    Dimension() int
}
```

Implementations must be safe for concurrent use by multiple
goroutines (the store may call Embed from multiple simultaneous Recalls).

The context allows cancelling the operation if the provider takes too long
(5s timeout by default in VectorScores).

The user implements it. The library uses it for:
1. Searching by cosine similarity in `Rank` (automatic if the provider is configured)
2. Validating stored embedding dimensions vs the current provider

## Example: OpenAI

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
        return nil, fmt.Errorf("no embeddings")
    }
    return result.Data[0].Embedding, nil
}

func (e *OpenAIEmbedder) Dimension() int { return 1536 }
```

## Example: local model (ollama)

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
        return nil, fmt.Errorf("no embeddings")
    }
    return result.Embeddings[0], nil
}

func (e *OllamaEmbedder) Dimension() int { return 768 }
```

## Configuring the store

```go
store, _ := memory.NewStore(memory.Config{
    Dir:     dir,
    Project: "my-app",
})

// Set the provider
store.SetEmbedder(&OpenAIEmbedder{APIKey: "sk-..."})
```

## Pre-calculating embeddings (recommended)

To avoid HTTP calls inside the `Remember` lock, pre-calculate the
embedding and pass it in the options:

```go
ctx := context.Background()
emb, _ := embedder.Embed(ctx, "Use JWT for auth")

res, _ := store.Remember(memory.RememberOptions{
    Key:       "auth-jwt",
    Category:  memory.CategoryDecision,
    Content:   "Use JWT for auth",
    Embedding: emb, // pre-calculated, outside the lock
})
```

## Semantic search

Once the provider is configured, vector search is automatic:

```go
// BM25 + vector similarity (weight 0.3)
ranked, _ := store.Recall("authentication", &memory.Query{Limit: 5})

// With RRF (rank fusion): BM25 + vector as separate rankers
ranked, _ = store.Recall("authentication", &memory.Query{
    Limit: 5,
    RRF:   true,
})
```

Without a configured provider, the pipeline is identical to v0.3.x.

## Dimension validation

The store automatically validates that stored embeddings match
the dimension of the current provider. If a capsule has an embedding
of a different dimension, it is skipped during vector search (graceful
degradation, no error).

The `EmbeddingDim` field in the capsule records the provider's dimension
at the time of storage.

## Amber Format

Embeddings are persisted as `embedding>0.12,0.34,...` in Amber format.
Old files without embeddings are read correctly (the field remains nil).

## Storage cost

Each 1536-dimensional embedding (OpenAI text-embedding-3-small) takes up:

| Format | Per capsule | 100 capsules | 1000 capsules |
|---------|-------------|--------------|---------------|
| Memory | ~12 KB | ~1.2 MB | ~12 MB |
| JSON | ~27 KB | ~2.7 MB | ~27 MB |
| Amber | ~18 KB | ~1.8 MB | ~18 MB |

To reduce cost:
- Use lower-dimensional models (384 instead of 1536)
- The default `MaxEntries: 100` limits growth

## Further reading

- [API.md](API.md) — `EmbeddingProvider` reference
- [ADVANCED.md](ADVANCED.md) — Advanced integration
