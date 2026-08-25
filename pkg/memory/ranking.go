package memory

import (
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
)

// Budget controla la verbosidad de la salida de recall.
type Budget string

const (
	BudgetTiny   Budget = "tiny"
	BudgetNormal Budget = "normal"
	BudgetDeep   Budget = "deep"
)

// Query es el conjunto de parámetros de una búsqueda.
type Query struct {
	Text      string   // texto o intento a buscar
	Category  Category // filtro opcional
	Tags      []string // filtro AND: todos deben estar presentes
	FromDate  string   // YYYY-MM-DD
	ToDate    string   // YYYY-MM-DD
	PathScope string   // glob
	AsOf      string   // vista temporal YYYY-MM-DD
	Limit     int      // default 6
	Graph     bool     // expandir ego-subgraph
	Hops      int      // 1 o 2 (default 1)
	RRF       bool     // usar fusión de rangos en vez de score lineal
	// SessionFiles son los archivos tocados por la sesión actual: las
	// cápsulas que los referencian reciben +15% de score (sesgo de sesión).
	SessionFiles map[string]bool
}

// RankOptions son las opciones internas de ejecución de una búsqueda.
type RankOptions struct {
	Query
	Now time.Time
}

// Ranked es el resultado de un recall con su puntuación.
type Ranked struct {
	Capsule *Capsule
	Score   float64
	IsSeed  bool
	Dist    int // 0 = seed; 1, 2 = vecino a N saltos
}

// Rank ejecuta el pipeline completo: filtrado → match → (grafo) → puntuación
// → orden → límite. Además incrementa los contadores de acceso de los
// devueltos (bump).
func (s *MemoryStore) Rank(opts RankOptions) ([]Ranked, error) {
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	limit := opts.Limit
	if limit <= 0 {
		limit = 6
	}
	hops := opts.Hops
	if hops < 1 {
		hops = 1
	}
	if hops > 2 {
		hops = 2
	}

	// 1. Filtrado de visibilidad + filtros explícitos.
	visible := make(map[string]*Capsule)
	for _, c := range s.entries {
		if !c.IsVisible(now, opts.AsOf) {
			continue
		}
		if opts.Category != "" && c.Category != opts.Category {
			continue
		}
		if !matchTags(c.Tags, opts.Tags) {
			continue
		}
		if opts.FromDate != "" && c.Date < opts.FromDate {
			continue
		}
		if opts.ToDate != "" && c.Date > opts.ToDate {
			continue
		}
		if opts.PathScope != "" && !pathMatch(c.PathScope, opts.PathScope) {
			continue
		}
		visible[c.Key] = c
	}

	allVisible := make([]*Capsule, 0, len(visible))
	for _, c := range visible {
		allVisible = append(allVisible, c)
	}

	// 2. Match textual → seeds.
	query := strings.ToLower(opts.Text)
	seeds := make(map[string]bool)
	if query != "" {
		qTokens := tokenize(query)
		for _, c := range allVisible {
			if matchQuery(c, qTokens) {
				seeds[c.Key] = true
			}
		}
	}

	// 3. Expansión del grafo (opcional). El grafo y la centralidad usan la
	// cache del almacén (invalidada por mutación): el grafo se construye
	// sobre la visibilidad completa, no sobre el subconjunto filtrado.
	graph := s.graphFor(now, opts.AsOf)
	centrality := s.centralityFor(now, opts.AsOf)
	dist := make(map[string]int)
	if opts.Graph && len(seeds) > 0 {
		dist = graph.EgoExpand(keysOf(seeds), hops, nil)
	} else {
		for k := range seeds {
			dist[k] = 0
		}
	}

	// 4. Puntuación.
	bm25 := BM25Scores(query, allVisible)
	var vecScores map[string]float64
	if s.embedder != nil {
		vecScores = VectorScores(s.embedder, query, allVisible)
	}

	type scored struct {
		c    *Capsule
		sc   float64
		seed bool
		d    int
	}
	// Pre-construir rankers para RRF (reusar en el loop).
	var rankers []map[string]float64
	if opts.RRF {
		rankers = make([]map[string]float64, 0, 5)
		rankers = append(rankers, bm25, bm25, bm25, centrality)
		if vecScores != nil {
			rankers = append(rankers, vecScores)
		}
	}
	candidates := make([]scored, 0, len(visible))
	for _, c := range allVisible {
		d, inDist := dist[c.Key]
		if !inDist && !seeds[c.Key] {
			continue
		}
		var sc float64
		if opts.RRF {
			sc = rrfScore(rankers, c.Key, len(allVisible))
		} else {
			sc = bm25[c.Key] +
				0.4*centrality[c.Key] +
				0.25*Importance(c, now)
			if vecScores != nil {
				sc += 0.3 * vecScores[c.Key]
			}
			if seeds[c.Key] {
				sc += 1.0 // seed bonus
			}
		}
		// Decay por distancia al seed (0.5^d).
		if d > 0 {
			sc *= math.Pow(0.5, float64(d))
		}
		// Sesgo de sesión: ×1.15 si referencia un archivo tocado.
		if len(opts.SessionFiles) > 0 && c.File != "" {
			f := c.File
			if i := strings.IndexByte(f, ':'); i >= 0 {
				f = f[:i]
			}
			if opts.SessionFiles[filepath.ToSlash(f)] {
				sc *= 1.15
			}
		}
		candidates = append(candidates, scored{c: c, sc: sc, seed: seeds[c.Key], d: d})
	}

	// 5. Separar fijadas (prioridad > 0) del resto: las fijadas siempre
	// entran al top (score 999), en orden de prioridad, aunque no matcheen
	// la query (respetan los filtros de visibilidad/categoría/fechas).
	var pinned, regular []scored
	for _, c := range allVisible {
		if c.Priority > 0 {
			pinned = append(pinned, scored{c: c, sc: 999, seed: seeds[c.Key], d: dist[c.Key]})
		}
	}
	for _, cd := range candidates {
		if cd.c.Priority == 0 {
			regular = append(regular, cd)
		}
	}
	sort.SliceStable(pinned, func(i, j int) bool {
		if pinned[i].c.Priority != pinned[j].c.Priority {
			return pinned[i].c.Priority > pinned[j].c.Priority
		}
		return pinned[i].sc > pinned[j].sc
	})
	sort.SliceStable(regular, func(i, j int) bool { return regular[i].sc > regular[j].sc })

	selected := make([]scored, 0, limit)
	for _, cd := range pinned {
		if len(selected) >= limit {
			break
		}
		selected = append(selected, cd)
	}
	for _, cd := range regular {
		if len(selected) >= limit {
			break
		}
		if cd.sc <= 0 && !cd.seed {
			continue
		}
		selected = append(selected, cd)
	}

	// 7. Fallback sin seeds: top por importancia.
	if len(selected) == 0 {
		imp := make([]scored, 0, len(allVisible))
		for _, c := range allVisible {
			imp = append(imp, scored{c: c, sc: Importance(c, now), d: -1})
		}
		sort.SliceStable(imp, func(i, j int) bool { return imp[i].sc > imp[j].sc })
		for _, cd := range imp {
			if len(selected) >= limit {
				break
			}
			selected = append(selected, cd)
		}
	}

	// 8. Bump de accesos de los devueltos: solo se marca dirty; la
	// persistencia se difiere a Flush() (una escritura por turno, no por
	// recall). El recall nunca falla por errores de disco.
	for _, cd := range selected {
		if !cd.c.IsVisible(now, opts.AsOf) {
			continue
		}
		cd.c.BumpAccess(now)
		s.dirty = true
	}

	out := make([]Ranked, 0, len(selected))
	for _, cd := range selected {
		out = append(out, Ranked{Capsule: cd.c, Score: cd.sc, IsSeed: cd.seed, Dist: cd.d})
	}
	return out, nil
}

func keysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// matchTags indica si la cápsula tiene todos los tags del filtro.
// Usa sync.Pool para reusar el map entre llamadas.
var tagPool = sync.Pool{
	New: func() interface{} {
		m := make(map[string]bool, 16)
		return &m
	},
}

func matchTags(capsuleTags, filter []string) bool {
	if len(filter) == 0 {
		return true
	}
	setp := tagPool.Get().(*map[string]bool)
	set := *setp
	// Limpiar map reusado.
	for k := range set {
		delete(set, k)
	}
	defer tagPool.Put(setp)

	for _, t := range capsuleTags {
		set[strings.ToLower(t)] = true
	}
	for _, f := range filter {
		if !set[strings.ToLower(f)] {
			return false
		}
	}
	return true
}

func pathMatch(scopeGlob, filterGlob string) bool {
	ok, err := filepath.Match(filterGlob, scopeGlob)
	return err == nil && ok
}

// tokenize normaliza un texto: divide camelCase, reemplaza -_ por espacio,
// lowercase y divide por whitespace.
func tokenize(text string) []string {
	text = splitCamel(text)
	text = strings.ReplaceAll(text, "-", " ")
	text = strings.ReplaceAll(text, "_", " ")
	fields := strings.Fields(strings.ToLower(text))
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if len(f) > 1 {
			out = append(out, f)
		}
	}
	return out
}

// splitCamel separa transiciones minúscula→mayúscula (redisPoolFix → redis Pool Fix).
func splitCamel(s string) string {
	if len(s) < 2 {
		return s
	}
	var sb strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		if i > 0 && isUpper(r) && !isUpper(runes[i-1]) {
			sb.WriteByte(' ')
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

func isUpper(r rune) bool { return r >= 'A' && r <= 'Z' }

// isWordRune indica si un rune forma parte de una palabra (alfanuméricos y
// acentos incluidos; no los separadores -_ espacio).
func isWordRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') || r == '_' ||
		unicode.IsLetter(r) || unicode.IsDigit(r)
}

// containsWord busca `word` en `text` con límite de palabra (no matchea
// substrings internos: "api" no está en "rapid"). Si relaxAfter, no se exige
// límite tras la palabra (para frases: "pull request" en "pull requests").
func containsWord(text, word string, relaxAfter bool) bool {
	if word == "" {
		return false
	}
	idx := 0
	for {
		i := strings.Index(text[idx:], word)
		if i < 0 {
			return false
		}
		i += idx
		beforeOK := i == 0 || !isWordRune(rune(text[i-1]))
		after := i + len(word)
		afterOK := relaxAfter || after >= len(text) || !isWordRune(rune(text[after]))
		if beforeOK && afterOK {
			return true
		}
		idx = i + len(word)
	}
}

// accentReplacer normaliza diacríticos para que "caché" matchee "cache".
var accentReplacer = strings.NewReplacer(
	"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n",
	"à", "a", "è", "e", "ì", "i", "ò", "o", "ù", "u", "â", "a", "ê", "e", "î", "i", "ô", "o", "û", "u",
	"Á", "A", "É", "E", "Í", "I", "Ó", "O", "Ú", "U", "Ü", "U", "Ñ", "N",
)

func normalizeAccents(s string) string { return accentReplacer.Replace(s) }

// matchKeyword indica si un keyword coincide con un texto: token exacto,
// prefijo de token sin vocal tras la raíz ("log" ↔ "logging", "api" ↔
// "apikey", pero no "logic"/"apiary"), o substring con límite de palabra
// (soporta frases y guiones: "use-zod", "pull request"). Sin falsos
// positivos de substring interno ("api" no está en "rapid").
func matchKeyword(text, kw string) bool {
	kw = normalizeAccents(strings.ToLower(strings.TrimSpace(kw)))
	if kw == "" {
		return false
	}
	text = normalizeAccents(strings.ToLower(text))
	for _, tok := range tokenize(text) {
		if tok == kw {
			return true
		}
		if len(tok) > len(kw) && strings.HasPrefix(tok, kw) && !vowelStart(tok[len(kw):]) {
			return true
		}
		if len(kw) > len(tok) && strings.HasPrefix(kw, tok) && !vowelStart(kw[len(tok):]) {
			return true
		}
	}
	relaxAfter := strings.ContainsAny(kw, " -_")
	return containsWord(text, kw, relaxAfter)
}

// vowelStart indica si un resto de token empieza por vocal (bloquea falsos
// positivos: "logic", "apiary", "logical").
func vowelStart(s string) bool {
	if s == "" {
		return false
	}
	switch s[0] {
	case 'a', 'e', 'i', 'o', 'u':
		return true
	}
	return false
}

// matchQuery indica si la cápsula matchea el query: match exacto de tokens
// o fuzzy (Levenshtein ≤ 2) sobre tokens del documento.
func matchQuery(c *Capsule, qTokens []string) bool {
	text := c.FullText()
	lower := strings.ToLower(text)
	// Usar tokens pre-computed si existen, fallback a tokenizar.
	docTokens := c.Tokens
	if len(docTokens) == 0 {
		docTokens = tokenize(lower)
		if len(docTokens) > maxBM25Tokens {
			docTokens = docTokens[:maxBM25Tokens]
		}
	}
	for _, qt := range qTokens {
		if matchKeyword(lower, qt) {
			return true
		}
	}
	for _, qt := range qTokens {
		if len(qt) <= 2 {
			continue
		}
		for _, dt := range docTokens {
			if levenshtein(qt, dt) <= 2 {
				return true
			}
		}
	}
	return false
}

// levenshtein distancia de edición (implementación simple con dos filas).
// Usa sync.Pool para reusar los buffers prev/cur entre llamadas.
var (
	levPrevPool = sync.Pool{
		New: func() interface{} {
			s := make([]int, 0, 64)
			return &s
		},
	}
	levCurPool = sync.Pool{
		New: func() interface{} {
			s := make([]int, 0, 64)
			return &s
		},
	}
)

func levenshtein(a, b string) int {
	ar, br := []rune(a), []rune(b)
	if len(ar) == 0 {
		return len(br)
	}
	if len(br) == 0 {
		return len(ar)
	}

	prevp := levPrevPool.Get().(*[]int)
	curp := levCurPool.Get().(*[]int)
	prev := (*prevp)[:0]
	cur := (*curp)[:0]

	// Expandir buffers si es necesario.
	if cap(prev) < len(br)+1 {
		prev = make([]int, len(br)+1)
		cur = make([]int, len(br)+1)
	} else {
		prev = prev[:len(br)+1]
		cur = cur[:len(br)+1]
	}

	for j := 0; j <= len(br); j++ {
		prev[j] = j
	}
	for i := 1; i <= len(ar); i++ {
		cur[0] = i
		for j := 1; j <= len(br); j++ {
			cost := 1
			if ar[i-1] == br[j-1] {
				cost = 0
			}
			cur[j] = min3(cur[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	result := prev[len(br)]

	// Devolver buffers al pool.
	*prevp = prev
	*curp = cur
	levPrevPool.Put(prevp)
	levCurPool.Put(curp)

	return result
}

func min3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

// BM25Scores calcula BM25 (k1=1.5, b=0.75) para cada cápsula contra el query.
func BM25Scores(query string, docs []*Capsule) map[string]float64 {
	out := make(map[string]float64)
	qTokens := tokenize(query)
	if len(qTokens) == 0 || len(docs) == 0 {
		return out
	}

	const k1 = 1.5
	const b = 0.75

	type doc struct {
		key   string
		text  string
		tf    map[string]int
		total int
	}
	parsed := make([]doc, 0, len(docs))
	var avgdl float64
	for _, c := range docs {
		text := strings.ToLower(c.FullText())
		// Usar tokens pre-computed si existen, fallback a tokenizar.
		tokens := c.Tokens
		if len(tokens) == 0 {
			tokens = tokenize(text)
			if len(tokens) > maxBM25Tokens {
				tokens = tokens[:maxBM25Tokens]
			}
		}
		tf := make(map[string]int)
		for _, t := range tokens {
			tf[t]++
		}
		d := doc{key: c.Key, text: text, tf: tf, total: len(tokens)}
		parsed = append(parsed, d)
		avgdl += float64(len(tokens))
	}
	if len(parsed) == 0 {
		return out
	}
	avgdl /= float64(len(parsed))
	if avgdl < 1 {
		avgdl = 1
	}

	n := float64(len(parsed))
	for _, qt := range qTokens {
		df := 0
		for _, d := range parsed {
			if d.tf[qt] > 0 {
				df++
			}
		}
		if df == 0 {
			continue
		}
		idf := math.Log((n-float64(df)+0.5)/(float64(df)+0.5) + 1)
		for _, d := range parsed {
			tf := float64(d.tf[qt])
			if tf == 0 {
				continue
			}
			denom := tf + k1*(1-b+b*float64(d.total)/avgdl)
			out[d.key] += idf * tf * (k1 + 1) / denom
		}
	}
	return out
}

// rrfScore fusiona rangos con Reciprocal Rank Fusion: Σ 1/(k+rank).
// rankers: map[key]score (el orden por score define el rango).
func rrfScore(rankers []map[string]float64, key string, n int) float64 {
	k := int(math.Round(math.Sqrt(float64(n))))
	if k < 3 {
		k = 3
	}
	if k > 60 {
		k = 60
	}
	var total float64
	for _, r := range rankers {
		rank := rankOf(r, key)
		total += 1.0 / (float64(k) + float64(rank))
	}
	return total
}

func rankOf(scores map[string]float64, key string) int {
	rank := 0
	v := scores[key]
	for _, other := range scores {
		if other > v {
			rank++
		}
	}
	return rank
}

// Render formatea los resultados según el presupuesto. Nunca muta el almacén.
func Render(rs []Ranked, budget Budget) string {
	if len(rs) == 0 {
		return "(sin resultados)"
	}
	index := make(map[string]int, len(rs))
	for i, r := range rs {
		index[r.Capsule.Key] = i + 1
	}
	var sb strings.Builder
	for i, r := range rs {
		c := r.Capsule
		switch budget {
		case BudgetTiny:
			sb.WriteString(fmt.Sprintf("[%s] %s — %s\n", c.Category, c.Key, firstLine(c.Content)))
		case BudgetDeep:
			sb.WriteString(fmt.Sprintf("[%d] %s/%s\n", i+1, c.Category, c.Key))
			sb.WriteString(fmt.Sprintf("  %s\n", c.Content))
			meta := []string{}
			if c.File != "" {
				meta = append(meta, "file: "+c.File)
			}
			if len(c.Tags) > 0 {
				meta = append(meta, "tags: "+strings.Join(c.Tags, ";"))
			}
			meta = append(meta, "date: "+c.Date)
			if c.TTL != "" {
				meta = append(meta, "ttl: "+c.TTL)
			}
			meta = append(meta, fmt.Sprintf("q: %.2f · c: %.2f", c.Quality, c.Confidence))
			meta = append(meta, "origin: "+string(c.Origin))
			if c.Status != StatusActive {
				meta = append(meta, "status: "+string(c.Status))
			}
			if c.Priority > 0 {
				meta = append(meta, fmt.Sprintf("pri: %d", c.Priority))
			}
			if c.LastAccessed != "" {
				meta = append(meta, "last: "+c.LastAccessed)
			}
			sb.WriteString("  " + strings.Join(meta, " | ") + "\n")
		default: // normal
			sb.WriteString(fmt.Sprintf("[%d] %s/%s\n", i+1, c.Category, c.Key))
			text := c.Content
			if !r.IsSeed && len([]rune(text)) > 90 {
				text = truncateRunes(text, 87) + "..."
			}
			sb.WriteString("  " + text + "\n")
			parts := []string{}
			if len(c.Tags) > 0 {
				parts = append(parts, "tags: "+strings.Join(c.Tags, ";"))
			}
			if len(c.Links) > 0 {
				edges := make([]string, 0, len(c.Links))
				for _, l := range c.Links {
					if n, ok := index[LinkKey(l)]; ok {
						edges = append(edges, fmt.Sprintf("->%d", n))
					}
				}
				if len(edges) > 0 {
					parts = append(parts, "edges: "+strings.Join(edges, ", "))
				}
			}
			if c.Priority > 0 {
				parts = append(parts, fmt.Sprintf("📌 pri:%d", c.Priority))
			}
			if len(parts) > 0 {
				sb.WriteString("  " + strings.Join(parts, " · ") + "\n")
			}
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
