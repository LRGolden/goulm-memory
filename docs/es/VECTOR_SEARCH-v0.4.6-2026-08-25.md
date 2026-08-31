# Vector Search

goulm-memory usa embeddings para busqueda semantica. Este documento
describe los metodos de busqueda vectorial implementados y por que.

## Metodos evaluados

| Metodo | Precision | Memoria extra | Build | Query | Go complexity |
|---|---|---|---|---|---|
| Brute-force | 100% | 0 | 0 | O(N×D) | Ninguna |
| **VP-Tree** | ~95% | O(N) | O(N×log N) | O(log N×D) | Media |
| KD-Tree | ~90% | O(N) | O(N×log N) | O(log N×D) | Media |
| HNSW | ~98% | O(N×M) | O(N×log N) | O(log N×D) | Alta |
| IVF-PQ | ~85% | O(N/k) | O(N×k) | O(k×D/q) | Alta |

- **D** = dimensionalidad (ej: 1536 para OpenAI ada-002)
- **N** = numero de capsules
- **M** = conexiones por nodo en HNSW (tipicamente 16)
- **k** = numero de clusters en IVF
- **q** = buckets por query en IVF

## Metodo implementado: VP-Tree

### Que es

Un arbol VP (Vantage Point) particiona el espacio vectorial usando
puntos de referencia (pivots). Cada nodo divide el espacio en dos
mitades: puntos mas cercanos al pivot (izquierda) y mas lejanos
(derecha).

### Por que VP-Tree

1. **Zero dependencias** — implementacion pura en Go, sin CGo
2. **Precision aceptable** — ~95% para recall@10
3. **Memoria** — O(N) adicional (arreglo de structs, no maps)
4. **Build** — Estricto O(N×log N) usando Quickselect in-place para la mediana
5. **Query** — Poda verdadera O(log N) con loop unrolling (SIMD)
6. **Simple** — ~250 lineas de codigo

### Por que NO los otros

- **Brute-force**: Funciona para N<5000, pero O(N×D) es lento para N>10K
- **KD-Tree**: Degradacion en dimensiones altas (>20 dim), mismo costo que VP-Tree
- **HNSW**: Mejor precision (~98%), pero ~500 lineas con skip lists. Overkill para N<50K
- **IVF-PQ**: Compresion cuantizada, perdida de precision. Solo para N>100K

### Integracion

El VP-Tree se construye automaticamente cuando:
- Hay un `EmbeddingProvider` configurado
- Hay capsules con embeddings pre-calculados
- El primer Recall se ejecuta

El tree se cachea y reconstruye cuando el store muta (Remember, Forget, etc).

### Uso externo

```go
import "github.com/LRGolden/goulm-memory/pkg/memory"

// Construir tree desde capsules
caps := store.All() // o filtrar por visibilidad
tree := memory.BuildVPTree(caps)

// Buscar 5 nearest neighbors
results := tree.Search(queryVector, 5, 0)

for _, r := range results {
    fmt.Printf("%s: score=%.3f, dist=%.3f\n", r.Key, r.Score, r.Distance)
}
```

### Limitaciones

- **Dimensiones altas** (>1024): Precision baja a ~80-85%
- **Distribucion uniforme**: Peor rendimiento que clustering
- **Updates**: Rebuild completo en cada mutacion (amortizado por cache)

### Benchmarks

```bash
# Ejecutar benchmarks de vector search
go test ./pkg/memory/ -run "^$" -bench "BenchmarkVectorScores" -benchmem

# Ejecutar benchmark de Recall completo
go test ./pkg/memory/ -run "^$" -bench "BenchmarkRecall" -benchmem
```

### Analisis de memoria

Para diagnostico de allocs en Recall:

```bash
# Ejecutar benchmark con memprofile
go test ./pkg/memory/ -run "^$" -bench "BenchmarkRecallProfile" -benchmem

# Analizar profile
go tool pprof -alloc_objects -inuse_space memprofile.out
```
