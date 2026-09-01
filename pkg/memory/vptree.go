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
	vector []float32
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
	dists := make([]distIdx, end-start-1)
	pivotVec := t.capsules[pivotIdx].vector
	for i := start + 1; i < end; i++ {
		dists[i-start-1] = distIdx{
			dist: euclideanDist(pivotVec, t.capsules[indices[i]].vector),
			idx:  indices[i],
		}
	}

	// Encontrar la mediana in-place usando quickselect (O(N)).
	median := len(dists) / 2
	quickselect(dists, median)

	// El umbral es la mediana de las distancias.
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
func (t *VPTree) Search(query []float32, k int, maxDist float64) []SearchResult {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if len(t.nodes) == 0 || k <= 0 {
		return nil
	}
	if maxDist <= 0 {
		maxDist = math.MaxFloat64
	}

	results := make([]SearchResult, 0, k)
	// Pasamos maxDist como puntero para que la poda sea efectiva en la recursión
	t.searchRecursive(query, 0, &maxDist, k, &results)

	// Ordenar por distancia (menor primero).
	sort.Slice(results, func(i, j int) bool {
		return results[i].Distance < results[j].Distance
	})

	// Convertir distancias a scores [0,1] (similitud coseno aproximada).
	for i := range results {
		results[i].Score = distToScore(results[i].Distance)
	}
	return results
}

// searchRecursive busca recursivamente en el árbol.
func (t *VPTree) searchRecursive(query []float32, nodeIdx int, maxDist *float64, k int, results *[]SearchResult) {
	if nodeIdx < 0 || nodeIdx >= len(t.nodes) {
		return
	}

	node := t.nodes[nodeIdx]
	pivotVec := t.capsules[node.pivotIdx].vector
	dist := euclideanDist(query, pivotVec)

	// Agregar pivot si está dentro del radio actual.
	if dist <= *maxDist {
		*results = append(*results, SearchResult{
			Key:      t.capsules[node.pivotIdx].key,
			Distance: dist,
		})

		// Si tenemos más de k resultados, eliminamos el más lejano.
		if len(*results) > k {
			maxIdx := 0
			for i, r := range *results {
				if r.Distance > (*results)[maxIdx].Distance {
					maxIdx = i
				}
			}
			*results = append((*results)[:maxIdx], (*results)[maxIdx+1:]...)
		}

		// Si el top-K está lleno, encogemos el radio máximo al peor elemento.
		// Esto es crucial para el rendimiento O(log N) del VP-Tree.
		if len(*results) == k {
			worstDist := -1.0
			for _, r := range *results {
				if r.Distance > worstDist {
					worstDist = r.Distance
				}
			}
			*maxDist = worstDist
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

	// Visitar segundo subárbol solo si el radio actual intersecta el umbral.
	if math.Abs(dist-node.threshold) <= *maxDist {
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
// Utiliza loop unrolling para permitir vectorización SIMD por el compilador.
func euclideanDist(a, b []float32) float64 {
	n := len(a)
	if n != len(b) || n == 0 {
		return math.MaxFloat64
	}

	var sum float32
	var i int
	// Procesar de a 4 elementos
	for i = 0; i <= n-4; i += 4 {
		d0 := a[i] - b[i]
		d1 := a[i+1] - b[i+1]
		d2 := a[i+2] - b[i+2]
		d3 := a[i+3] - b[i+3]
		sum += d0*d0 + d1*d1 + d2*d2 + d3*d3
	}
	// Elementos restantes
	for ; i < n; i++ {
		d := a[i] - b[i]
		sum += d * d
	}
	return math.Sqrt(float64(sum))
}

type distIdx struct {
	dist float64
	idx  int
}

// quickselect encuentra el k-ésimo elemento más pequeño en el arreglo dists,
// reordenando los elementos in-place de forma parcial (O(N)).
func quickselect(dists []distIdx, k int) {
	left, right := 0, len(dists)-1
	for left < right {
		pivotIdx := left + rand.Intn(right-left+1)
		pivotVal := dists[pivotIdx].dist
		dists[pivotIdx], dists[right] = dists[right], dists[pivotIdx]
		storeIdx := left
		for i := left; i < right; i++ {
			if dists[i].dist < pivotVal {
				dists[i], dists[storeIdx] = dists[storeIdx], dists[i]
				storeIdx++
			}
		}
		dists[storeIdx], dists[right] = dists[right], dists[storeIdx]

		if storeIdx == k {
			return
		} else if storeIdx < k {
			left = storeIdx + 1
		} else {
			right = storeIdx - 1
		}
	}
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
