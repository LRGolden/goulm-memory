package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Primer genera el mapa de conocimiento del proyecto: top cápsulas, conteos
// por categoría y patrones destacados. Es el contexto auto-inyectado al
// inicio de sesión. Determinista y sin LLM.
func (s *MemoryStore) Primer(limit int) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if limit <= 0 {
		limit = 5
	}
	now := time.Now()

	active := make([]*Capsule, 0, len(s.entries))
	byCat := map[Category]int{CategoryDecision: 0, CategoryPattern: 0, CategoryBug: 0, CategoryKnowledge: 0}
	for _, c := range s.entries {
		if !c.IsVisible(now, "") {
			continue
		}
		active = append(active, c)
		byCat[c.Category]++
	}
	if len(active) == 0 {
		return "🧠 Memoria: vacía — usa memory_remember tras la primera decisión.", nil
	}

	// Top por prioridad, luego importancia.
	sort.SliceStable(active, func(i, j int) bool {
		if active[i].Priority != active[j].Priority {
			return active[i].Priority > active[j].Priority
		}
		return Importance(active[i], now) > Importance(active[j], now)
	})

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🧠 Memoria del proyecto (%d cápsulas: %d decisiones, %d patrones, %d bugs, %d conocimiento)\n",
		len(active), byCat[CategoryDecision], byCat[CategoryPattern], byCat[CategoryBug], byCat[CategoryKnowledge]))
	sb.WriteString("Top:\n")
	for i, c := range active {
		if i >= limit {
			break
		}
		label := ""
		switch {
		case c.Quality >= 0.5:
			label = "alta"
		case c.Quality >= 0.3:
			label = "media"
		default:
			label = "baja"
		}
		pin := ""
		if c.Priority > 0 {
			pin = fmt.Sprintf(" 📌%d", c.Priority)
		}
		sb.WriteString(fmt.Sprintf("  [%s] %s — %s (q:%s)%s\n",
			c.Category, c.Key, firstLine(c.Content), label, pin))
	}
	// Patrones destacados.
	var patterns []*Capsule
	for _, c := range active {
		if c.Category == CategoryPattern {
			patterns = append(patterns, c)
		}
	}
	if len(patterns) > 0 {
		sort.SliceStable(patterns, func(i, j int) bool {
			return Importance(patterns[i], now) > Importance(patterns[j], now)
		})
		sb.WriteString("Patrones:\n")
		n := 3
		if len(patterns) < n {
			n = len(patterns)
		}
		for _, p := range patterns[:n] {
			sb.WriteString(fmt.Sprintf("  • %s: %s\n", p.Key, firstLine(p.Content)))
		}
	}
	return strings.TrimRight(sb.String(), "\n"), nil
}

// StatsView es el estado resumido del almacén.
type StatsView struct {
	Total       int            `json:"total"`
	ByCategory  map[string]int `json:"by_category"`
	ByOrigin    map[string]int `json:"by_origin"`
	ByStatus    map[string]int `json:"by_status"`
	Archived    int            `json:"archived"`
	Expired     int            `json:"expired"`
	Pinned      int            `json:"pinned"`
	AvgQuality  float64        `json:"avg_quality"`
	FileSizeKB  float64        `json:"file_size_kb"`
	LastUpdated string         `json:"last_updated"`
}

// Stats calcula el estado del almacén.
func (s *MemoryStore) Stats() (StatsView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	st := StatsView{
		ByCategory: make(map[string]int),
		ByOrigin:   make(map[string]int),
		ByStatus:   make(map[string]int),
	}
	var qSum float64
	for _, c := range s.entries {
		st.Total++
		st.ByCategory[string(c.Category)]++
		st.ByOrigin[string(c.Origin)]++
		st.ByStatus[string(c.Status)]++
		qSum += c.Quality
		if c.Priority > 0 {
			st.Pinned++
		}
		if c.IsExpired(now) {
			st.Expired++
		}
	}
	if st.Total > 0 {
		st.AvgQuality = qSum / float64(st.Total)
	}
	st.Archived = len(s.archive)
	if fi, err := os.Stat(s.files.Memory); err == nil {
		st.FileSizeKB = float64(fi.Size()) / 1024
	}
	if fi, err := os.Stat(s.files.Config); err == nil {
		st.LastUpdated = fi.ModTime().UTC().Format(time.RFC3339)
	}
	return st, nil
}

// RenderStats formatea StatsView para el agente.
func RenderStats(st StatsView) string {
	if st.Total == 0 {
		return "🧠 Memoria vacía. Guarda tu primera cápsula con memory_remember."
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🧠 Memoria del proyecto\n"))
	sb.WriteString(fmt.Sprintf("Total: %d cápsulas\n", st.Total))
	sb.WriteString(fmt.Sprintf("├── decision: %d\n", st.ByCategory["decision"]))
	sb.WriteString(fmt.Sprintf("├── pattern: %d\n", st.ByCategory["pattern"]))
	sb.WriteString(fmt.Sprintf("├── bug: %d\n", st.ByCategory["bug"]))
	sb.WriteString(fmt.Sprintf("└── knowledge: %d\n", st.ByCategory["knowledge"]))
	sb.WriteString(fmt.Sprintf("Archivadas: %d · Fijadas: %d · Expiradas: %d\n", st.Archived, st.Pinned, st.Expired))
	sb.WriteString(fmt.Sprintf("Calidad media: %.2f · Tamaño: %.1f KB\n", st.AvgQuality, st.FileSizeKB))
	if st.LastUpdated != "" {
		sb.WriteString("Última actualización: " + st.LastUpdated + "\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

// DiffReport resume los cambios desde una fecha.
type DiffReport struct {
	Since   string     `json:"since"`
	New     []*Capsule `json:"new"`
	Updated []*Capsule `json:"updated"`
}

// Diff devuelve cápsulas nuevas (fecha = hoy) y actualizadas (último acceso
// reciente, sin ser nuevas) desde since ("24h", "7d" o YYYY-MM-DD).
func (s *MemoryStore) Diff(since string) (DiffReport, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().AddDate(0, 0, -7)
	switch since {
	case "24h":
		cutoff = time.Now().Add(-24 * time.Hour)
	case "7d":
		cutoff = time.Now().AddDate(0, 0, -7)
	default:
		if t, err := time.ParseInLocation("2006-01-02", since, time.Local); err == nil {
			cutoff = t
		}
	}
	rep := DiffReport{Since: cutoff.Format("2006-01-02")}
	for _, c := range s.entries {
		date, dateOK := time.ParseInLocation("2006-01-02", c.Date, time.Local)
		if dateOK == nil && date.After(cutoff) {
			rep.New = append(rep.New, c)
			continue
		}
		// Actualizadas: accedidas tras el corte, no creadas en el período.
		if c.LastAccessed != "" {
			if la, err := time.Parse(time.RFC3339, c.LastAccessed); err == nil && la.After(cutoff) {
				rep.Updated = append(rep.Updated, c)
			}
		}
	}
	sort.SliceStable(rep.New, func(i, j int) bool { return rep.New[i].Date > rep.New[j].Date })
	sort.SliceStable(rep.Updated, func(i, j int) bool { return rep.Updated[i].LastAccessed > rep.Updated[j].LastAccessed })
	return rep, nil
}

// RenderDiff formatea el diff para el agente.
func RenderDiff(rep DiffReport) string {
	if len(rep.New) == 0 && len(rep.Updated) == 0 {
		return fmt.Sprintf("📋 Sin cápsulas nuevas desde %s.", rep.Since)
	}
	var sb strings.Builder
	if len(rep.New) > 0 {
		sb.WriteString(fmt.Sprintf("📋 Cápsulas nuevas desde %s (%d):\n", rep.Since, len(rep.New)))
		for _, c := range rep.New {
			sb.WriteString(fmt.Sprintf("  [%s] %s\n    %s\n", c.Category, c.Key, firstLine(c.Content)))
		}
	}
	if len(rep.Updated) > 0 {
		sb.WriteString(fmt.Sprintf("🔄 Cápsulas actualizadas desde %s (%d):\n", rep.Since, len(rep.Updated)))
		for _, c := range rep.Updated {
			sb.WriteString(fmt.Sprintf("  [%s] %s\n    %s\n", c.Category, c.Key, firstLine(c.Content)))
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}

// Backup copia memory (+ archive) a backups/ con timestamp y poda a MaxBackups.
// Devuelve la ruta del backup creado.
func (s *MemoryStore) Backup() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(s.files.Backups, 0700); err != nil {
		return "", err
	}
	ext := filepath.Ext(s.files.Memory)
	stamp := time.Now().UTC().Format("2006-01-02T15-04-05")
	name := "memory-" + stamp + ext
	dest := filepath.Join(s.files.Backups, name)
	data, err := os.ReadFile(s.files.Memory)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if err := os.WriteFile(dest, data, 0600); err != nil {
		return "", err
	}
	// Podar backups viejos.
	entries, err := os.ReadDir(s.files.Backups)
	if err != nil {
		return dest, nil
	}
	var names []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "memory-") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for len(names) > s.cfg.MaxBackups {
		old := names[0]
		names = names[1:]
		os.Remove(filepath.Join(s.files.Backups, old))
	}
	return dest, nil
}
