package memory

import (
	"regexp"
	"strings"
)

// linkRefRE detecta referencias implícitas [[clave]] dentro del contenido.
var linkRefRE = regexp.MustCompile(`\[\[([a-z0-9][a-z0-9-]*)\]\]`)

// Graph es el grafo de conocimiento: nodos = cápsulas (por key),
// edges no dirigidos entre claves existentes.
type Graph struct {
	byKey map[string]*Capsule
	adj   map[string]map[string]bool
}

// BuildGraph construye el grafo a partir de cápsulas activas.
// Fuentes de edges: links explícitos, refs [[clave]] implícitas y
// tags compartidas (≥2 en común).
func BuildGraph(capsules []*Capsule) *Graph {
	g := &Graph{
		byKey: make(map[string]*Capsule),
		adj:   make(map[string]map[string]bool),
	}
	for _, c := range capsules {
		if c == nil || c.Key == "" {
			continue
		}
		if _, exists := g.byKey[c.Key]; !exists {
			g.byKey[c.Key] = c
		}
	}
	for key := range g.byKey {
		g.adj[key] = make(map[string]bool)
	}

	// Links explícitos (se ignoran los que apuntan a claves inexistentes).
	for key, c := range g.byKey {
		for _, link := range c.Links {
			if target := LinkKey(link); target != "" && target != key {
				if _, ok := g.byKey[target]; ok {
					g.addEdge(key, target)
				}
			}
		}
	}

	// Refs implícitas [[clave]] en el contenido.
	for key, c := range g.byKey {
		for _, m := range linkRefRE.FindAllStringSubmatch(c.Content, -1) {
			target := m[1]
			if target != key {
				if _, ok := g.byKey[target]; ok {
					g.addEdge(key, target)
				}
			}
		}
	}

	// Tags compartidas: si dos cápsulas comparten ≥2 tags → edge sintético.
	// Implementación O(N*T) con índice invertido tag→keys en vez de O(N²).
	// Umbral: tags que aparecen en >maxTagCapsules cápsulas se ignoran
	// porque son genéricos ("important", "todo") y no crean edges útiles.
	const maxTagCapsules = 50
	tagIndex := make(map[string][]string)
	for _, c := range capsules {
		if c == nil || c.Key == "" {
			continue
		}
		for _, tag := range c.Tags {
			tagIndex[tag] = append(tagIndex[tag], c.Key)
		}
	}
	sharedCount := make(map[[2]string]int)
	for _, keys := range tagIndex {
		if len(keys) < 2 || len(keys) > maxTagCapsules {
			continue
		}
		for i := 0; i < len(keys); i++ {
			for j := i + 1; j < len(keys); j++ {
				a, b := keys[i], keys[j]
				if a > b {
					a, b = b, a
				}
				pair := [2]string{a, b}
				sharedCount[pair]++
				if sharedCount[pair] == 2 {
					g.addEdge(a, b)
				}
			}
		}
	}
	return g
}

func (g *Graph) addEdge(a, b string) {
	g.adj[a][b] = true
	g.adj[b][a] = true
}

func sharedTagCount(a, b []string) int {
	set := make(map[string]bool, len(a))
	for _, t := range a {
		set[t] = true
	}
	n := 0
	for _, t := range b {
		if set[t] {
			n++
		}
	}
	return n
}

// LinkKey extrae la clave destino de un token de link, descartando el tipo
// ("supersedes:engine-arch" → "engine-arch"). Usa el último ':' como separador
// porque las claves kebab-case no contienen colones.
func LinkKey(token string) string {
	idx := strings.LastIndex(token, ":")
	if idx < 0 {
		return token
	}
	return token[idx+1:]
}

// Neighbors devuelve las claves adyacentes a la clave dada.
func (g *Graph) Neighbors(key string) []string {
	set, ok := g.adj[key]
	if !ok {
		return nil
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	return out
}

// HasKey indica si la clave existe en el grafo.
func (g *Graph) HasKey(key string) bool {
	_, ok := g.byKey[key]
	return ok
}

// Node devuelve la cápsula del nodo (nil si no existe).
func (g *Graph) Node(key string) *Capsule {
	return g.byKey[key]
}

// Degree devuelve el grado (número de vecinos) de una clave.
func (g *Graph) Degree(key string) int {
	return len(g.adj[key])
}

// Centrality devuelve el grado normalizado (0–1) por clave.
func (g *Graph) Centrality() map[string]float64 {
	out := make(map[string]float64, len(g.adj))
	maxDeg := 0
	for key := range g.adj {
		if d := len(g.adj[key]); d > maxDeg {
			maxDeg = d
		}
	}
	if maxDeg == 0 {
		return out
	}
	for key := range g.adj {
		out[key] = float64(len(g.adj[key])) / float64(maxDeg)
	}
	return out
}

// EgoExpand expande desde los seeds hasta `hops` niveles (BFS) y devuelve
// la distancia mínima por clave. Solo visita nodos visibles.
func (g *Graph) EgoExpand(seeds []string, hops int, visible func(*Capsule) bool) map[string]int {
	if hops < 1 {
		hops = 1
	}
	if hops > 2 {
		hops = 2
	}
	dist := make(map[string]int)
	queue := make([]string, 0, len(seeds))
	for _, s := range seeds {
		if !g.HasKey(s) {
			continue
		}
		if visible != nil && !visible(g.byKey[s]) {
			continue
		}
		if _, seen := dist[s]; !seen {
			dist[s] = 0
			queue = append(queue, s)
		}
	}
	for len(queue) > 0 && hops > 0 {
		next := make([]string, 0)
		for _, cur := range queue {
			for _, nb := range g.Neighbors(cur) {
				if _, seen := dist[nb]; seen {
					continue
				}
				if visible != nil && !visible(g.byKey[nb]) {
					continue
				}
				dist[nb] = dist[cur] + 1
				if dist[nb] < hops {
					next = append(next, nb)
				}
			}
		}
		queue = next
	}
	return dist
}

// ShortestPath encuentra el camino más corto (BFS) entre dos claves.
// Devuelve nil si no hay camino.
func (g *Graph) ShortestPath(a, b string) []string {
	if a == b {
		if g.HasKey(a) {
			return []string{a}
		}
		return nil
	}
	prev := make(map[string]string)
	queue := []string{a}
	visited := map[string]bool{a: true}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, nb := range g.Neighbors(cur) {
			if visited[nb] {
				continue
			}
			visited[nb] = true
			prev[nb] = cur
			if nb == b {
				return reconstructPath(prev, a, b)
			}
			queue = append(queue, nb)
		}
	}
	return nil
}

func reconstructPath(prev map[string]string, a, b string) []string {
	path := []string{b}
	cur := b
	for cur != a {
		p, ok := prev[cur]
		if !ok {
			return nil
		}
		path = append(path, p)
		cur = p
	}
	// Invertir: el último elemento es a.
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path
}
