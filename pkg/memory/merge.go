package memory

import (
	"sort"
	"time"
)

// nearDupThreshold es el umbral de similitud Jaccard para near-duplicates.
const nearDupThreshold = 0.7

// MergeCapsules fusiona dos cápsulas de la misma clave. `existing` conserva
// su identidad; `incoming` aporta contenido y atributos nuevos. El contenido
// que prevalece es el más detallado (mayor longitud en runes): un re-guardado
// con contenido más corto no pierde el detalle anterior.
func MergeCapsules(existing, incoming *Capsule) *Capsule {
	out := existing.Clone()
	if len([]rune(incoming.Content)) > len([]rune(out.Content)) {
		out.Content = incoming.Content
	}
	out.Tags = unionStrings(existing.Tags, incoming.Tags)
	out.Links = unionStrings(existing.Links, incoming.Links)
	if incoming.Date > out.Date {
		out.Date = incoming.Date
	}
	if incoming.TTL != "" {
		out.TTL = incoming.TTL
	}
	out.Accessed += incoming.Accessed
	if incoming.Confidence > out.Confidence {
		out.Confidence = incoming.Confidence
	}
	if incoming.LastAccessed > out.LastAccessed {
		out.LastAccessed = incoming.LastAccessed
	}
	if incoming.Priority > out.Priority {
		out.Priority = incoming.Priority
	}
	if incoming.PathScope != "" {
		out.PathScope = incoming.PathScope
	}
	if originRank(incoming.Origin) > originRank(out.Origin) {
		out.Origin = incoming.Origin
	}
	if out.Status != StatusActive && incoming.Status == StatusActive {
		out.Status = StatusActive
	}
	if incoming.File != "" {
		out.File = incoming.File
	}
	if len(incoming.Embedding) > 0 {
		out.Embedding = incoming.Embedding
	}
	return out
}

func originRank(o Origin) int {
	switch o {
	case OriginHuman:
		return 3
	case OriginAgent:
		return 2
	default:
		return 1
	}
}

func unionStrings(a, b []string) []string {
	set := make(map[string]bool, len(a)+len(b))
	for _, s := range a {
		set[s] = true
	}
	for _, s := range b {
		set[s] = true
	}
	out := make([]string, 0, len(set))
	for s := range set {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// Jaccard calcula la similitud de conjuntos de palabras entre dos textos.
func Jaccard(a, b string) float64 {
	wa := tokenize(a)
	wb := tokenize(b)
	if len(wa) == 0 && len(wb) == 0 {
		return 1
	}
	sa := make(map[string]bool, len(wa))
	for _, w := range wa {
		sa[w] = true
	}
	intersection := 0
	uniqueB := make(map[string]bool, len(wb))
	for _, w := range wb {
		uniqueB[w] = true
		if sa[w] {
			intersection++
		}
	}
	union := len(sa) + len(uniqueB) - intersection
	if union == 0 {
		return 1
	}
	return float64(intersection) / float64(union)
}

// ConsolidateReport resume el resultado de una consolidación.
type ConsolidateReport struct {
	Merged         int `json:"merged"`
	NearDuplicates int `json:"near_duplicates"`
	Removed        int `json:"removed"`
	Before         int `json:"before"`
	After          int `json:"after"`
}

// Consolidate fusiona cápsulas duplicadas:
//  1. Misma clave → merge.
//  2. Contenido idéntico (normalizado) → conservar la primera.
//  3. Near-duplicates (Jaccard ≥ 0.7, misma categoría) → merge en la de mejor
//     calidad. Determinista y sin LLM.
func (s *MemoryStore) Consolidate() (ConsolidateReport, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rep := ConsolidateReport{Before: len(s.entries)}
	now := time.Now()

	// Fase 1: merge por clave.
	byKey := make(map[string]*Capsule)
	for _, c := range s.entries {
		if c == nil || c.Key == "" {
			continue
		}
		if existing, ok := byKey[c.Key]; ok {
			merged := MergeCapsules(existing, c)
			merged.Quality = QualityScore(merged, now)
			byKey[c.Key] = merged
			rep.Merged++
		} else {
			byKey[c.Key] = c
		}
	}

	// Fase 2: duplicados exactos por contenido normalizado (misma categoría).
	seenContent := make(map[string]*Capsule)
	unique := make([]*Capsule, 0, len(byKey))
	for _, c := range byKey {
		norm := c.Normalized()
		if norm == "" {
			unique = append(unique, c)
			continue
		}
		// La clave incluye la categoría: contenido idéntico con distinta
		// categoría es conocimiento distinto y se conserva.
		key := norm + "\x00" + string(c.Category)
		if existing, ok := seenContent[key]; ok {
			merged := MergeCapsules(existing, c)
			merged.Quality = QualityScore(merged, now)
			seenContent[key] = merged
			rep.Removed++
		} else {
			seenContent[key] = c
			unique = append(unique, c)
		}
	}

	// Fase 3: near-duplicates (Jaccard ≥ 0.7, misma categoría).
	// Limitado a maxNearDupPairs comparaciones para evitar O(N²) con
	// colecciones grandes (>500 cápsulas).
	pairs := 0
	for i := 0; i < len(unique) && pairs < maxNearDupPairs; i++ {
		for j := i + 1; j < len(unique) && pairs < maxNearDupPairs; j++ {
			a, b := unique[i], unique[j]
			if a == nil || b == nil || a.Key == b.Key {
				continue
			}
			if a.Category != b.Category {
				continue
			}
			pairs++
			if Jaccard(a.Content, b.Content) < nearDupThreshold {
				continue
			}
			// Fusionar el peor en el mejor (mejor calidad).
			var keeper, other *Capsule
			if a.Quality >= b.Quality {
				keeper, other = a, b
			} else {
				keeper, other = b, a
			}
			merged := MergeCapsules(keeper, other)
			merged.Quality = QualityScore(merged, now)
			*keeper = *merged
			unique = append(unique[:j], unique[j+1:]...)
			rep.NearDuplicates++
			j--
		}
	}

	s.entries = make(map[string]*Capsule, len(unique))
	for _, c := range unique {
		s.entries[c.ID] = c
	}
	s.rebuildByKeyLocked()
	s.bumpGraph()
	// Prune archive si excede MaxArchive.
	if len(s.archive) > s.cfg.MaxArchive {
		archived := make([]*Capsule, 0, len(s.archive))
		for _, c := range s.archive {
			archived = append(archived, c)
		}
		sort.SliceStable(archived, func(i, j int) bool {
			return archived[i].Quality < archived[j].Quality
		})
		pruned := archived[len(archived)-s.cfg.MaxArchive:]
		s.archive = make(map[string]*Capsule, len(pruned))
		for _, c := range pruned {
			s.archive[c.ID] = c
		}
	}
	rep.After = len(s.entries)
	if err := s.persistLocked(); err != nil {
		return rep, err
	}
	return rep, nil
}
