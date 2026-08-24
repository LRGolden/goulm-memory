package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
)

// archiveDays: cápsulas con más de 30 días se archivan.
const archiveDays = 30

// RememberOptions son las opciones de guardado.
type RememberOptions struct {
	Category  Category
	Key       string
	Content   string
	File      string
	Tags      []string
	TTL       string
	Links     []string
	Priority  int
	PathScope string
	Origin    Origin
	Verbatim  bool      // true: sin inferir tags ni recalcular calidad
	Embedding []float64 // embedding pre-calculado (opcional)
}

// RememberResult describe el resultado de un guardado.
type RememberResult struct {
	Capsule  *Capsule
	Created  bool
	Merged   bool
	Inferred []string
}

// Remember guarda una cápsula: valida, infiere tags si faltan, calcula
// calidad/confianza, mergea si la clave ya existe y persiste.
func (s *MemoryStore) Remember(o RememberOptions) (RememberResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	caps, err := NewCapsule(o.Category, o.Key, o.Content)
	if err != nil {
		return RememberResult{}, err
	}
	caps.File = o.File
	if len(o.Tags) > 0 {
		caps.Tags = dedupeStrings(o.Tags)
	}
	if o.Priority > 0 {
		if o.Priority > 5 {
			o.Priority = 5
		}
		caps.Priority = o.Priority
	}
	caps.PathScope = o.PathScope
	caps.ApplyTTL(o.TTL, now)
	if len(o.Links) > 0 {
		caps.Links = dedupeStrings(o.Links)
	}
	if o.Origin != "" {
		caps.Origin = o.Origin
		caps.Confidence = ConfidenceFor(o.Origin)
	}
	if len(o.Embedding) > 0 {
		caps.Embedding = append([]float64(nil), o.Embedding...)
		if s.embedder != nil {
			caps.EmbeddingDim = s.embedder.Dimension()
		}
	}

	res := RememberResult{}
	if !o.Verbatim {
		if len(caps.Tags) == 0 {
			caps.Tags = InferTags(caps.Content, caps.Key, s.vocab)
			res.Inferred = caps.Tags
		}
		caps.Quality = QualityScore(caps, now)
	}

	// Pre-computar tokens para BM25.
	caps.Tokens = computeTokens(caps)

	// Merge-dedup por clave.
	if existing, ok := s.byKey(caps.Key); ok {
		merged := MergeCapsules(existing, caps)
		if !o.Verbatim {
			merged.Quality = QualityScore(merged, now)
		}
		s.entries[merged.ID] = merged
		s.byKeyIdx[merged.Key] = merged.ID
		res.Capsule = merged
		res.Merged = true
	} else {
		s.entries[caps.ID] = caps
		s.byKeyIdx[caps.Key] = caps.ID
		res.Capsule = caps
		res.Created = true
	}

	// Auto-archive si se supera el límite.
	if len(s.entries) > s.cfg.MaxEntries {
		s.trimToMaxLocked(now)
	}

	s.bumpGraph()
	if err := s.persistLocked(); err != nil {
		return res, err
	}
	return res, nil
}

// byKey busca una cápsula por clave usando el índice key→ID con fallback
// lineal por compatibilidad.
func (s *MemoryStore) byKey(key string) (*Capsule, bool) {
	if id, ok := s.byKeyIdx[key]; ok {
		if c, ok := s.entries[id]; ok {
			return c, true
		}
	}
	for _, c := range s.entries {
		if c.Key == key {
			return c, true
		}
	}
	return nil, false
}

func (s *MemoryStore) rebuildByKeyLocked() {
	s.byKeyIdx = make(map[string]string, len(s.entries))
	for id, c := range s.entries {
		s.byKeyIdx[c.Key] = id
	}
}

// trimToMaxLocked archiva cápsulas viejas/expiradas hasta caber en el límite.
func (s *MemoryStore) trimToMaxLocked(now time.Time) {
	// 1. Expiradas y viejas (>30 días).
	for id, c := range s.entries {
		if len(s.entries) <= s.cfg.MaxEntries {
			break
		}
		if c.IsExpired(now) || createdDaysAgo(c, now) > archiveDays {
			s.archive[id] = c
			delete(s.entries, id)
			delete(s.byKeyIdx, c.Key)
		}
	}
	// 2. Si sigue sobrando, archivar las de menor importancia. Primera
	// pasada sin fijadas; si todo está fijado, archivar igualmente la menos
	// importante (el límite es duro).
	for len(s.entries) > s.cfg.MaxEntries {
		var worst *Capsule
		worstScore := 1e9
		for _, c := range s.entries {
			if c.Priority > 0 {
				continue
			}
			if sc := Importance(c, now); sc < worstScore {
				worstScore = sc
				worst = c
			}
		}
		if worst == nil {
			worstScore = 1e9
			for _, c := range s.entries {
				if sc := Importance(c, now); sc < worstScore {
					worstScore = sc
					worst = c
				}
			}
		}
		if worst == nil {
			break
		}
		s.archive[worst.ID] = worst
		delete(s.entries, worst.ID)
		delete(s.byKeyIdx, worst.Key)
	}
}

func createdDaysAgo(c *Capsule, now time.Time) int {
	t, err := time.ParseInLocation("2006-01-02", c.Date, time.Local)
	if err != nil {
		return 0
	}
	return int(now.Sub(t).Hours() / 24)
}

func dedupeStrings(in []string) []string {
	set := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v != "" && !set[v] {
			set[v] = true
			out = append(out, v)
		}
	}
	return out
}

// SmartRecall es el recall unificado para inicio de tarea: BM25 + grafo +
// calidad + decay en una llamada, con salida compacta. sessionFiles (opcional)
// activa el sesgo de sesión del ranking.
func (s *MemoryStore) SmartRecall(intent string, limit int, sessionFiles ...map[string]bool) ([]Ranked, error) {
	q := Query{Text: intent, Limit: limit, Graph: true, Hops: 1}
	if len(sessionFiles) > 0 {
		q.SessionFiles = sessionFiles[0]
	}
	return s.Rank(RankOptions{
		Query: q,
		Now:   time.Now(),
	})
}

// Recall busca cápsulas por texto con filtros opcionales.
// opts puede ser nil (usa defaults: límite 6, modo plano).
func (s *MemoryStore) Recall(query string, opts *Query) ([]Ranked, error) {
	q := Query{Text: query, Limit: 6}
	if opts != nil {
		q = *opts
		q.Text = query
	}
	return s.Rank(RankOptions{Query: q, Now: time.Now()})
}

// Suggest encuentra cápsulas relacionadas a un contexto (sin ser estrictas).
// sessionFiles (opcional) activa el sesgo de sesión del ranking.
func (s *MemoryStore) Suggest(context string, limit int, sessionFiles ...map[string]bool) ([]Ranked, error) {
	if limit <= 0 {
		limit = 5
	}
	q := Query{Text: context, Limit: limit}
	if len(sessionFiles) > 0 {
		q.SessionFiles = sessionFiles[0]
	}
	return s.Rank(RankOptions{
		Query: q,
		Now:   time.Now(),
	})
}

// Forget elimina o soft-deletea una cápsula por clave.
// En soft-delete, marca la cápsula como obsolete y registra la fecha de
// supersedencia (SupersededOn) para habilitar la vista temporal.
func (s *MemoryStore) Forget(key string, hard bool) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.byKey(key)
	if !ok {
		return false, nil
	}
	if hard {
		delete(s.entries, c.ID)
		delete(s.byKeyIdx, c.Key)
	} else {
		c.Status = StatusObsolete
		c.SupersededOn = time.Now().Format("2006-01-02")
	}
	s.bumpGraph()
	if err := s.persistLocked(); err != nil {
		return false, err
	}
	return true, nil
}

// Resolve restaura una cápsula soft-deleted a active y limpia SupersededOn.
func (s *MemoryStore) Resolve(key string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, c := range s.entries {
		if c.Key == key {
			c.Status = StatusActive
			c.SupersededOn = ""
			s.bumpGraph()
			if err := s.persistLocked(); err != nil {
				return false, err
			}
			return true, nil
		}
	}
	return false, nil
}

// Pin fija (o desfija con priority=0) la prioridad de una cápsula.
func (s *MemoryStore) Pin(key string, priority int) (bool, error) {
	if priority < 0 || priority > 5 {
		return false, fmt.Errorf("prioridad inválida: %d (0-5)", priority)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.byKey(key)
	if !ok {
		return false, nil
	}
	c.Priority = priority
	s.bumpGraph()
	if err := s.persistLocked(); err != nil {
		return false, err
	}
	return true, nil
}

// ArchiveOld mueve al archive las cápsulas con >30 días o TTL expirado.
// Devuelve cuántas se archivaron.
func (s *MemoryStore) ArchiveOld() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	moved := 0
	for id, c := range s.entries {
		if c.IsExpired(now) || createdDaysAgo(c, now) > archiveDays {
			s.archive[id] = c
			delete(s.entries, id)
			delete(s.byKeyIdx, c.Key)
			moved++
		}
	}
	if moved == 0 {
		return 0, nil
	}
	s.bumpGraph()
	if err := s.persistLocked(); err != nil {
		return moved, err
	}
	return moved, nil
}

// ExportJSON devuelve todas las cápsulas (activas + archivadas) como JSON.
func (s *MemoryStore) ExportJSON() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	all := make([]*Capsule, 0, len(s.entries)+len(s.archive))
	for _, c := range s.entries {
		all = append(all, c)
	}
	for _, c := range s.archive {
		all = append(all, c)
	}
	slices.SortStableFunc(all, func(a, b *Capsule) int {
		return strings.Compare(a.Key, b.Key)
	})
	return jsonMarshalIndent(all)
}

// ImportCapsules inserta cápsulas externas con merge por clave (unión de
// tags, máxima confianza). Devuelve cuántas se añadieron.
func (s *MemoryStore) ImportCapsules(capsules []*Capsule) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	added := 0
	changed := false
	for _, raw := range capsules {
		if raw == nil || raw.Key == "" || !ValidCategory(raw.Category) {
			continue
		}
		if strings.TrimSpace(raw.Content) == "" {
			continue
		}
		in := raw.Clone()
		// Pre-computar tokens para BM25 si no existen.
		if len(in.Tokens) == 0 {
			in.Tokens = computeTokens(in)
		}
		// Validar/corregir estados y orígenes inválidos del import.
		if !ValidStatus(in.Status) {
			in.Status = ""
		}
		if !ValidOrigin(in.Origin) {
			in.Origin = ""
		}
		// Dedupe por ID: si ya existe una capsula con el mismo ID, merge.
		if existing, ok := s.entries[in.ID]; ok {
			merged := MergeCapsules(existing, in)
			merged.Quality = QualityScore(merged, now)
			s.entries[merged.ID] = merged
			s.byKeyIdx[merged.Key] = merged.ID
			changed = true
			continue
		}
		if existing, ok := s.byKey(in.Key); ok {
			merged := MergeCapsules(existing, in)
			merged.Quality = QualityScore(merged, now)
			s.entries[merged.ID] = merged
			s.byKeyIdx[merged.Key] = merged.ID
			changed = true
		} else {
			c := in
			if c.ID == "" {
				c.ID = NewID()
			}
			if c.Date == "" {
				c.Date = now.Format("2006-01-02")
			}
			if c.Origin == "" {
				c.Origin = OriginAgent
			}
			if c.Status == "" {
				c.Status = StatusActive
			}
			s.entries[c.ID] = c
			s.byKeyIdx[c.Key] = c.ID
			added++
			changed = true
		}
	}
	if !changed {
		return 0, nil
	}
	s.bumpGraph()
	if err := s.persistLocked(); err != nil {
		return added, err
	}
	return added, nil
}

// SetVocab reemplaza el vocabulario del proyecto (deps). Se mergea con el
// vocabulario built-in en InferTags.
func (s *MemoryStore) SetVocab(v map[string][]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if v == nil {
		s.vocab = make(map[string][]string)
	} else {
		s.vocab = v
	}
	return s.writeMetaLocked()
}

// Vocab devuelve el vocabulario del proyecto (copia).
func (s *MemoryStore) Vocab() map[string][]string {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make(map[string][]string, len(s.vocab))
	for k, v := range s.vocab {
		out[k] = append([]string(nil), v...)
	}
	return out
}

// SetEmbedder configura el proveedor de embeddings. Si es nil, la busqueda
// vectorial se desactiva (comportamiento identico a v0.3.x).
func (s *MemoryStore) SetEmbedder(p EmbeddingProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.embedder = p
}

// Embedder devuelve el proveedor de embeddings configurado (puede ser nil).
func (s *MemoryStore) Embedder() EmbeddingProvider {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.embedder
}

// ListActive devuelve las cápsulas activas visibles ordenadas por importancia.
func (s *MemoryStore) ListActive(limit int) []*Capsule {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	out := make([]*Capsule, 0, len(s.entries))
	for _, c := range s.entries {
		if c.IsVisible(now, "") {
			out = append(out, c)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return Importance(out[i], now) > Importance(out[j], now)
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// Clear vacía el almacén (tras backup automático) y devuelve cuántas había.
func (s *MemoryStore) Clear() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	n := len(s.entries)
	if n == 0 {
		return 0, nil
	}
	// Backup de seguridad antes de vaciar.
	if err := os.MkdirAll(s.files.Backups, 0700); err == nil {
		ext := filepath.Ext(s.files.Memory)
		data, _ := os.ReadFile(s.files.Memory)
		os.WriteFile(filepath.Join(s.files.Backups, "memory-pre-clear-"+time.Now().UTC().Format("20060102T150405")+ext), data, 0600)
	}
	s.entries = make(map[string]*Capsule)
	s.rebuildByKeyLocked()
	s.bumpGraph()
	if err := s.persistLocked(); err != nil {
		return n, err
	}
	return n, nil
}

// Sessions abre el tracker de sesiones del almacén.
func (s *MemoryStore) Sessions(agent string) (*SessionTracker, error) {
	return NewSessionTracker(s.files.Sessions, agent)
}

// Format devuelve el formato actual de almacenamiento.
func (s *MemoryStore) Format() Format {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg.Format
}

// Dir devuelve el directorio del almacén.
func (s *MemoryStore) Dir() string { return s.cfg.Dir }

// Project devuelve el nombre del proyecto declarado.
func (s *MemoryStore) Project() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg.Project
}
