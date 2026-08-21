package memory

import (
	"os"
	"strings"
	"testing"
)

func TestHealthCleanStore(t *testing.T) {
	s := newTestStore(t, FormatJSON)
	s.Remember(RememberOptions{Category: CategoryDecision, Key: "k", Content: "Contenido saludable"})
	rep, err := s.Health(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if rep.Score != 100 {
		t.Errorf("score = %d, esperaba 100 (warnings: %+v)", rep.Score, rep)
	}
}

func TestHealthIssues(t *testing.T) {
	s := newTestStore(t, FormatJSON)

	// Link huérfano.
	s.Remember(RememberOptions{Category: CategoryDecision, Key: "con-link", Content: "Apunta a algo", Links: []string{"no-existe"}})
	// Duplicado exacto.
	s.Remember(RememberOptions{Category: CategoryBug, Key: "bug-1", Content: "El pool de redis se agota"})
	s.Remember(RememberOptions{Category: CategoryBug, Key: "bug-2", Content: "El pool de redis se agota"})
	// TTL expirado.
	s.Remember(RememberOptions{Category: CategoryKnowledge, Key: "expirada", Content: "Deadline viejo"})
	if c, ok := s.byKey("expirada"); ok {
		c.TTL = "2020-01-01"
	}
	// Archivo inexistente.
	s.Remember(RememberOptions{Category: CategoryKnowledge, Key: "file-roto", Content: "Referencia un archivo", File: "no-existe-12345.go"})
	// Path scope sin archivo.
	s.Remember(RememberOptions{Category: CategoryPattern, Key: "scope-solo", Content: "Scope sin archivo", PathScope: "pkg/**/*.go"})

	rep, err := s.Health(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if rep.Score >= 100 {
		t.Error("con problemas, el score debería bajar de 100")
	}
	if len(rep.OrphanLinks) != 1 {
		t.Errorf("orphan links = %v", rep.OrphanLinks)
	}
	if rep.ExactDuplicates != 1 {
		t.Errorf("exact duplicates = %d", rep.ExactDuplicates)
	}
	if len(rep.ExpiredTTL) != 1 {
		t.Errorf("expired ttl = %v", rep.ExpiredTTL)
	}
	if len(rep.BrokenFiles) != 1 {
		t.Errorf("broken files = %v", rep.BrokenFiles)
	}
	if len(rep.MissingEvidence) != 1 {
		t.Errorf("missing evidence = %v", rep.MissingEvidence)
	}
}

func TestHealthDetectsSecrets(t *testing.T) {
	s := newTestStore(t, FormatJSON)
	s.Remember(RememberOptions{Category: CategoryKnowledge, Key: "secreto", Content: "la api key es sk-ant-abcdefghijklmnopqrstuvwxyz123456"})
	rep, err := s.Health(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Secrets) != 1 {
		t.Errorf("secrets = %v", rep.Secrets)
	}
}

func TestRenderHealth(t *testing.T) {
	rep := HealthReport{Score: 87, Entries: 10, AvgQuality: 0.65, OrphanLinks: []string{"a → b"}}
	out := RenderHealth(rep)
	if !strings.Contains(out, "87") || !strings.Contains(out, "a → b") {
		t.Errorf("render = %q", out)
	}
}

func TestPrimerEmptyAndFilled(t *testing.T) {
	s := newTestStore(t, FormatJSON)
	primer, err := s.Primer(5)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(primer, "vacía") {
		t.Errorf("primer vacío = %q", primer)
	}

	s.Remember(RememberOptions{Category: CategoryDecision, Key: "k1", Content: "Decisión importante"})
	s.Remember(RememberOptions{Category: CategoryPattern, Key: "p1", Content: "Convención del repo"})
	primer, _ = s.Primer(5)
	if !strings.Contains(primer, "k1") || !strings.Contains(primer, "Patrones") {
		t.Errorf("primer = %q", primer)
	}
}

func TestStatsAndRender(t *testing.T) {
	s := newTestStore(t, FormatJSON)
	st, err := s.Stats()
	if err != nil {
		t.Fatal(err)
	}
	out := RenderStats(st)
	if !strings.Contains(out, "vacía") {
		t.Errorf("stats vacías = %q", out)
	}
	s.Remember(RememberOptions{Category: CategoryDecision, Key: "d", Content: "Decisión"})
	st, _ = s.Stats()
	if st.Total != 1 || st.ByCategory["decision"] != 1 {
		t.Errorf("stats = %+v", st)
	}
}

func TestDiff(t *testing.T) {
	s := newTestStore(t, FormatJSON)
	s.Remember(RememberOptions{Category: CategoryKnowledge, Key: "reciente", Content: "Nueva hoy"})
	rep, err := s.Diff("24h")
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.New) != 1 || rep.New[0].Key != "reciente" {
		t.Errorf("diff = %+v", rep.New)
	}
	repOld, _ := s.Diff("2020-01-01")
	if len(repOld.New) != 1 {
		t.Errorf("diff con fecha antigua = %d", len(repOld.New))
	}
}

func TestBackupPrune(t *testing.T) {
	s := newTestStore(t, FormatJSON)
	s.Remember(RememberOptions{Category: CategoryKnowledge, Key: "k", Content: "C"})
	s.cfg.MaxBackups = 3
	for i := 0; i < 5; i++ {
		if _, err := s.Backup(); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(s.files.Backups)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) > 3 {
		t.Errorf("backups = %d, esperaba ≤ 3", len(entries))
	}
}
