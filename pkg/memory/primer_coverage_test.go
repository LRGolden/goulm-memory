package memory

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestPrimerDefaultLimitAndQuality(t *testing.T) {
	s := newTestStore(t, FormatJSON)
	for i := 0; i < 8; i++ {
		s.Remember(RememberOptions{Category: CategoryKnowledge, Key: "k" + itoa(i), Content: "contenido numero" + itoa(i)})
	}
	// limit <= 0 → default 5.
	primer, err := s.Primer(0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(primer, "(q:") {
		t.Errorf("primer debería incluir calidad: %q", primer)
	}
	// Solo 5 tops (más la línea de cabecera).
	lines := 0
	for _, l := range strings.Split(primer, "\n") {
		if strings.HasPrefix(l, "  [") {
			lines++
		}
	}
	if lines != 5 {
		t.Errorf("primer con limit=0 debería mostrar 5 tops, got %d", lines)
	}
}

func TestPrimerPriorityPin(t *testing.T) {
	s := newTestStore(t, FormatJSON)
	s.Remember(RememberOptions{Category: CategoryDecision, Key: "pin-a", Content: "Normal"})
	s.Remember(RememberOptions{Category: CategoryDecision, Key: "pin-b", Content: "Fijada", Priority: 3})
	primer, err := s.Primer(5)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(primer, "📌3") {
		t.Errorf("debería mostrar el pin, got: %q", primer)
	}
	// La fijada va primero (prioridad >).
	idxA := strings.Index(primer, "pin-a")
	idxB := strings.Index(primer, "pin-b")
	if idxB > idxA {
		t.Errorf("la cápsula con prioridad debería aparecer antes, primer=%q", primer)
	}
}

func TestPrimerNoPatternsSection(t *testing.T) {
	s := newTestStore(t, FormatJSON)
	s.Remember(RememberOptions{Category: CategoryDecision, Key: "solo-decision", Content: "X"})
	primer, err := s.Primer(5)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(primer, "Patrones:") {
		t.Errorf("no debería haber sección de patrones, got: %q", primer)
	}
}

func TestPrimerQualityLabels(t *testing.T) {
	s := newTestStore(t, FormatJSON)
	s.Remember(RememberOptions{Category: CategoryKnowledge, Key: "lab1", Content: "baja calidad"})
	s.Remember(RememberOptions{Category: CategoryKnowledge, Key: "lab2", Content: "alta calidad"})
	s.mu.Lock()
	for _, c := range s.entries {
		if c.Key == "lab1" {
			c.Quality = 0.2
		}
		if c.Key == "lab2" {
			c.Quality = 0.8
		}
	}
	s.mu.Unlock()
	primer, err := s.Primer(5)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(primer, "(q:baja)") || !strings.Contains(primer, "(q:alta)") {
		t.Errorf("primer debería incluir etiquetas de calidad, got: %q", primer)
	}
}

func TestDiff7dAndDate(t *testing.T) {
	s := newTestStore(t, FormatJSON)
	s.Remember(RememberOptions{Category: CategoryKnowledge, Key: "nueva", Content: "C"})

	rep7, err := s.Diff("7d")
	if err != nil {
		t.Fatal(err)
	}
	if len(rep7.New) != 1 {
		t.Errorf("diff 7d = %d, want 1", len(rep7.New))
	}

	// since inválido → default 7d (no error).
	repBad, err := s.Diff("no-es-fecha")
	if err != nil {
		t.Fatal(err)
	}
	if len(repBad.New) != 1 {
		t.Errorf("diff inválida = %d, want 1", len(repBad.New))
	}
}

func TestCoverage_DiffUpdated(t *testing.T) {
	s := newTestStore(t, FormatJSON)
	// Cápsula con Date antigua creada directamente en entries.
	old, _ := NewCapsule(CategoryBug, "old-date", "contenido")
	old.Date = "2020-01-05"
	old.Origin = OriginAgent
	s.mu.Lock()
	s.entries[old.ID] = old
	s.mu.Unlock()
	// Acceso reciente → updated, no new.
	old.BumpAccess(time.Now().Add(time.Hour))

	rep, err := s.Diff("2025-12-31")
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Updated) == 0 {
		t.Fatalf("debería haber cápsulas actualizadas, rep=%+v", rep)
	}
	out := RenderDiff(rep)
	if !strings.Contains(out, "actualizadas") {
		t.Errorf("RenderDiff = %q", out)
	}
}

func TestRenderDiffEmpty(t *testing.T) {
	out := RenderDiff(DiffReport{Since: "2020-01-01"})
	if !strings.Contains(out, "Sin cápsulas") {
		t.Errorf("RenderDiff vacío = %q", out)
	}
}

func TestRenderStatsFull(t *testing.T) {
	st := StatsView{
		Total:      2,
		ByCategory: map[string]int{"decision": 1, "pattern": 1},
		Archived:   1,
		Pinned:     2,
		Expired:    0,
		AvgQuality: 0.75,
		FileSizeKB: 3.5,
	}
	out := RenderStats(st)
	if !strings.Contains(out, "Total: 2") || !strings.Contains(out, "Calidad media: 0.75") {
		t.Errorf("RenderStats = %q", out)
	}
}

func TestBackupMissingFile(t *testing.T) {
	s := newTestStore(t, FormatJSON)
	// Borrar el archivo de memoria para cubrir la rama IsNotExist.
	os.Remove(s.files.Memory)
	path, err := s.Backup()
	if err != nil {
		t.Fatalf("Backup con memoria ausente no debería fallar: %v", err)
	}
	if path == "" {
		t.Fatal("Backup debería devolver ruta")
	}
}

func TestStatsFileSizeAndUpdated(t *testing.T) {
	s := newTestStore(t, FormatJSON)
	s.Remember(RememberOptions{Category: CategoryDecision, Key: "st", Content: "x"})
	st, err := s.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if st.Total != 1 {
		t.Errorf("Total = %d, want 1", st.Total)
	}
	if st.FileSizeKB <= 0 {
		t.Errorf("FileSizeKB debería ser > 0")
	}
	if st.LastUpdated == "" {
		t.Error("LastUpdated no debería estar vacío")
	}
	if st.ByOrigin["agent"] != 1 || st.ByStatus["active"] != 1 {
		t.Errorf("stats origin/status = %+v", st)
	}
}

func itoa(n int) string {
	return strconv.Itoa(n)
}

var _ = filepath.Join
