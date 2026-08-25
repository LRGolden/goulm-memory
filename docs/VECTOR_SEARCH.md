# Vector Search

goulm-memory uses embeddings for semantic search. This document
describes the vector search methods implemented and why.

## Evaluated methods

| Method | Accuracy | Extra memory | Build | Query | Go complexity |
|---|---|---|---|---|---|
| Brute-force | 100% | 0 | 0 | O(N×D) | None |
| **VP-Tree** | ~95% | O(N) | O(N×log N) | O(log N×D) | Medium |
| KD-Tree | ~90% | O(N) | O(N×log N) | O(log N×D) | Medium |
| HNSW | ~98% | O(N×M) | O(N×log N) | O(log N×D) | High |
| IVF-PQ | ~85% | O(N/k) | O(N×k) | O(k×D/q) | High |

- **D** = dimensionality (e.g., 1536 for OpenAI ada-002)
- **N** = number of capsules
- **M** = connections per node in HNSW (typically 16)
- **k** = number of clusters in IVF
- **q** = buckets per query in IVF

## Implemented method: VP-Tree

### What is it

A VP (Vantage Point) tree partitions the vector space using
reference points (pivots). Each node divides the space into two
halves: points closer to the pivot (left) and points farther away
(right).

### Why VP-Tree

1. **Zero dependencies** — pure Go implementation, no CGo
2. **Acceptable accuracy** — ~95% for recall@10
3. **Memory** — O(N) additional (array of structs, not maps)
4. **Build** — O(N×log N) amortized, lazy rebuild
5. **Query** — O(log N×D) with threshold pruning
6. **Simple** — ~200 lines of code

### Why NOT the others

- **Brute-force**: Works for N<5000, but O(N×D) is slow for N>10K
- **KD-Tree**: Degradation in high dimensions (>20 dim), same cost as VP-Tree
- **HNSW**: Better accuracy (~98%), but ~500 lines with skip lists. Overkill for N<50K
- **IVF-PQ**: Quantized compression, precision loss. Only for N>100K

### Integration

The VP-Tree is built automatically when:
- There is a configured `EmbeddingProvider`
- There are capsules with pre-computed embeddings
- The first Recall is executed

The tree is cached and reconstructed when the store mutates (Remember, Forget, etc.).

### External usage

```go
import "github.com/LRGolden/goulm-memory/pkg/memory"

// Build tree from capsules
caps := store.All() // or filter by visibility
tree := memory.BuildVPTree(caps)

// Search for 5 nearest neighbors
results := tree.Search(queryVector, 5, 0)

for _, r := range results {
    fmt.Printf("%s: score=%.3f, dist=%.3f\n", r.Key, r.Score, r.Distance)
}
```

### Limitations

- **High dimensions** (>1024): Accuracy drops to ~80-85%
- **Uniform distribution**: Worse performance than clustering
- **Updates**: Full rebuild on each mutation (amortized by cache)

### Benchmarks

```bash
# Run vector search benchmarks
go test ./pkg/memory/ -run "^$" -bench "BenchmarkVectorScores" -benchmem

# Run full Recall benchmark
go test ./pkg/memory/ -run "^$" -bench "BenchmarkRecall" -benchmem
```

### Memory analysis

For alloc diagnostics in Recall:

```bash
# Run benchmark with memprofile
go test ./pkg/memory/ -run "^$" -bench "BenchmarkRecallProfile" -benchmem

# Analyze profile
go tool pprof -alloc_objects -inuse_space memprofile.out
```