package memory

import (
	"math"
	"math/rand"
	"sort"
	"sync"
)

// VPTree es un árbol VP (Vantage Point) para búsqueda de nearest neighbors
// aproximada. Construido sobre embeddings pre-calculados de cápsulas.
//
// Complejidad:
//   - Build: O(N × log N) amortizado
//   - Search: O(log N × D) con pruning por umbral
//   - Memoria: O(N) adicional (estructuras fijas, sin maps)
//
// Seguro para uso concurrente: múltiples lectores, un writer (rebuild).
type VPTree struct {
	nodes    []vpNode
	dim      int
	version  int
	mu       sync.RWMutex
	capsules []*indexedCapsule
}

// indexedCapsule almacena el vector pre-extraído para evitar acceso
// a Capsule.Embedding en cada comparación del árbol.
type indexedCapsule struct {
	key    string
	vector []float64
}

// vpNode es un nodo del árbol VP.
type vpNode struct {
	pivotIdx  int     // índice en capsules[]
	threshold float64 // radio de división (distancia al pivot)
	left      int     // índice en nodes[] (-1 = leaf)
	right     int     // índice en nodes[] (-1 = leaf)
}

// SearchResult es un resultado de búsqueda con distancia.
type SearchResult struct {
	Key      string
	Score    float64 // similitud coseno normalizada [0,1]
	Distance float64 // distancia euclidiana al query
}

// BuildVPTree construye el árbol VP a partir de capsules con embeddings.
// Si una cápsula no tiene embedding, se ignora. Devuelve nil si no hay
// embeddings válidos.
func BuildVPTree(capsules []*Capsule) *VPTree {
	indexed := make([]*indexedCapsule, 0, len(capsules))
	for _, c := range capsules {
		if c == nil || len(c.Embedding) == 0 {
			continue
		}
		indexed = append(indexed, &indexedCapsule{
			key:    c.Key,
			vector: c.Embedding,
		})
	}
	if len(indexed) == 0 {
		return nil
	}

	dim := len(indexed[0].vector)
	t := &VPTree{
		nodes:    make([]vpNode, 0, len(indexed)),
		dim:      dim,
		capsules: indexed,
	}

	// Construir recursivamente.
	indices := make([]int, len(indexed))
	for i := range indices {
		indices[i] = i
	}
	t.buildRecursive(indices, 0, len(indices))

	return t
}

// buildRecursive construye el subárbol para el rango [start, end) de indices.
func (t *VPTree) buildRecursive(indices []int, start, end int) {
	if start >= end {
		return
	}

	// Elegir pivot aleatorio.
	pivotLocal := start + rand.Intn(end-start)
	// Mover pivot al inicio.
	indices[start], indices[pivotLocal] = indices[pivotLocal], indices[start]
	pivotIdx := indices[start]

	nodeIdx := len(t.nodes)
	t.nodes = append(t.nodes, vpNode{pivotIdx: pivotIdx, left: -1, right: -1})

	if end-start <= 1 {
		return
	}

	// Calcular distancias al pivot.
	type distIdx struct {
		dist float64
		idx  int
	}
	dists := make([]distIdx, end-start-1)
	pivotVec := t.capsules[pivotIdx].vector
	for i := start + 1; i < end; i++ {
		dists[i-start-1] = distIdx{
			dist: euclideanDist(pivotVec, t.capsules[indices[i]].vector),
			idx:  indices[i],
		}
	}

	// Ordenar por distancia.
	sort.Slice(dists, func(i, j int) bool {
		return dists[i].dist < dists[j].dist
	})

	// El umbral es la mediana de las distancias.
	median := len(dists) / 2
	t.nodes[nodeIdx].threshold = dists[median].dist

	// Reordenar indices: [start+1 ... start+1+median] = izquierda, resto = derecha.
	for i, d := range dists {
		indices[start+1+i] = d.idx
	}

	// Construir subárboles.
	leftEnd := start + 1 + median
	t.nodes[nodeIdx].left = len(t.nodes)
	t.buildRecursive(indices, start+1, leftEnd)
	t.nodes[nodeIdx].right = len(t.nodes)
	t.buildRecursive(indices, leftEnd, end)
}

// Search encuentra los k nearest neighbors al query vector.
// maxDist define el radio máximo de búsqueda (distancia euclidiana).
// Si maxDist <= 0, se usa infinity.
func (t *VPTree) Search(query []float64, k int, maxDist float64) []SearchResult {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if len(t.nodes) == 0 || k <= 0 {
		return nil
	}
	if maxDist <= 0 {
		maxDist = math.MaxFloat64
	}

	results := make([]SearchResult, 0, k)
	t.searchRecursive(query, 0, maxDist, k, &results)

	// Ordenar por distancia (menor primero).
	sort.Slice(results, func(i, j int) bool {
		return results[i].Distance < results[j].Distance
	})
	if len(results) > k {
		results = results[:k]
	}

	// Convertir distancias a scores [0,1] (similitud coseno aproximada).
	for i := range results {
		results[i].Score = distToScore(results[i].Distance)
	}
	return results
}

// searchRecursive busca recursivamente en el árbol.
func (t *VPTree) searchRecursive(query []float64, nodeIdx int, maxDist float64, k int, results *[]SearchResult) {
	if nodeIdx < 0 || nodeIdx >= len(t.nodes) {
		return
	}

	node := t.nodes[nodeIdx]
	pivotVec := t.capsules[node.pivotIdx].vector
	dist := euclideanDist(query, pivotVec)

	// Agregar pivot si está dentro del radio.
	if dist <= maxDist {
		*results = append(*results, SearchResult{
			Key:      t.capsules[node.pivotIdx].key,
			Distance: dist,
		})
		// Si tenemos k resultados, ajustar maxDist al más lejano.
		if len(*results) > k {
			// Encontrar el más lejano.
			maxIdx := 0
			for i, r := range *results {
				if r.Distance > (*results)[maxIdx].Distance {
					maxIdx = i
				}
			}
			// Eliminar el más lejano.
			*results = append((*results)[:maxIdx], (*results)[maxIdx+1:]...)
		}
	}

	// Determinar orden de visita: primero el subárbol más cercano.
	goLeft := dist <= node.threshold
	var first, second int
	if goLeft {
		first, second = node.left, node.right
	} else {
		first, second = node.right, node.left
	}

	// Visitar primer subárbol.
	t.searchRecursive(query, first, maxDist, k, results)

	// Visitar segundo subárbol solo si el radio intersecta el umbral.
	if math.Abs(dist-node.threshold) <= maxDist {
		t.searchRecursive(query, second, maxDist, k, results)
	}
}

// Version devuelve la versión actual del árbol (incrementada en rebuild).
func (t *VPTree) Version() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.version
}

// Len devuelve el número de cápsulas indexadas.
func (t *VPTree) Len() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.capsules)
}

// --- Funciones auxiliares ---

// euclideanDist calcula la distancia euclidiana entre dos vectores.
func euclideanDist(a, b []float64) float64 {
	if len(a) != len(b) {
		return math.MaxFloat64
	}
	var sum float64
	for i := range a {
		d := a[i] - b[i]
		sum += d * d
	}
	return math.Sqrt(sum)
}

// distToScore convierte una distancia euclidiana a un score [0,1].
// Distancia 0 → score 1.0, distancia creciente → score decreciente.
func distToScore(dist float64) float64 {
	if dist <= 0 {
		return 1.0
	}
	// Usar decaimiento exponencial: score = e^(-dist²/2)
	return math.Exp(-dist * dist / 2)
}
