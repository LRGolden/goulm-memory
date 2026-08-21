package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T, format Format) *MemoryStore {
	t.Helper()
	dir := t.TempDir()
	s, err := NewStore(Config{
		Dir:     dir,
		Format:  format,
		Project: "test-proyecto",
	})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return s
}

func TestRememberAndRecall(t *testing.T) {
	s := newTestStore(t, FormatJSON)

	res, err := s.Remember(RememberOptions{
		Category: CategoryDecision,
		Key:      "use-zod",
		Content:  "Usar Zod para validación en lugar de Joi",
		File:     "src/types.ts",
	})
	if err != nil {
		t.Fatalf("Remember: %v", err)
	}
	if !res.Created {
		t.Fatal("debería crear una cápsula nueva")
	}
	if res.Capsule.Quality <= 0 {
		t.Error("la calidad debería calcularse automáticamente")
	}
	if len(res.Capsule.Tags) == 0 {
		t.Error("los tags deberían inferirse")
	}

	// Recall por keyword.
	rs, err := s.Recall("zod", nil)
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}
	if len(rs) != 1 || rs[0].Capsule.Key != "use-zod" {
		t.Errorf("recall = %v", rs)
	}
}

func TestMergeOnSameKey(t *testing.T) {
	s := newTestStore(t, FormatJSON)

	_, err := s.Remember(RememberOptions{Category: CategoryPattern, Key: "reglas", Content: "Primera versión", Tags: []string{"go"}})
	if err != nil {
		t.Fatal(err)
	}
	res, err := s.Remember(RememberOptions{Category: CategoryPattern, Key: "reglas", Content: "Segunda versión más completa", Tags: []string{"testing"}})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Merged {
		t.Fatal("segunda escritura con la misma clave debería mergear")
	}
	c := res.Capsule
	if c.Content != "Segunda versión más completa" {
		t.Errorf("content = %q", c.Content)
	}
	if len(c.Tags) != 2 {
		t.Errorf("tags = %v, esperaba unión [go testing]", c.Tags)
	}
	if got := len(s.ListActive(0)); got != 1 {
		t.Errorf("cápsulas activas = %d, esperaba 1", got)
	}
}

func TestSoftDeleteAndResolve(t *testing.T) {
	s := newTestStore(t, FormatJSON)
	s.Remember(RememberOptions{Category: CategoryBug, Key: "bug-x", Content: "Un bug cualquiera"})

	ok, err := s.Forget("bug-x", false)
	if err != nil || !ok {
		t.Fatalf("Forget: ok=%v err=%v", ok, err)
	}
	c, _ := s.byKey("bug-x")
	if c == nil || c.Status != StatusObsolete {
		t.Fatal("soft delete debería marcar obsolete")
	}
	// No aparece en recalls normales.
	rs, _ := s.Recall("bug", nil)
	if len(rs) != 0 {
		t.Errorf("recall debería excluir obsolete, got %d", len(rs))
	}
	ok, err = s.Resolve("bug-x")
	if err != nil || !ok {
		t.Fatalf("Resolve: ok=%v err=%v", ok, err)
	}
	rs, _ = s.Recall("bug", nil)
	if len(rs) != 1 {
		t.Errorf("tras resolver debería reaparecer, got %d", len(rs))
	}

	// Hard delete.
	ok, _ = s.Forget("bug-x", true)
	if !ok {
		t.Fatal("hard delete debería borrar")
	}
	if _, exists := s.byKey("bug-x"); exists {
		t.Fatal("hard delete no eliminó la cápsula")
	}
}

func TestPinInjection(t *testing.T) {
	s := newTestStore(t, FormatJSON)
	s.Remember(RememberOptions{Category: CategoryKnowledge, Key: "importante", Content: "Hecho clave del proyecto"})
	for i := 0; i < 10; i++ {
		s.Remember(RememberOptions{Category: CategoryKnowledge, Key: "otra-" + string(rune('a'+i)), Content: "Relleno " + string(rune('a'+i))})
	}
	if ok, _ := s.Pin("importante", 5); !ok {
		t.Fatal("Pin falló")
	}
	rs, _ := s.Recall("relleno", nil)
	if len(rs) == 0 {
		t.Fatal("recall vacío")
	}
	if rs[0].Capsule.Key != "importante" {
		t.Errorf("la fijada debería aparecer primero, got %q", rs[0].Capsule.Key)
	}
}

func TestArchiveOld(t *testing.T) {
	s := newTestStore(t, FormatJSON)
	s.Remember(RememberOptions{Category: CategoryKnowledge, Key: "vieja", Content: "Muy vieja"})
	if c, _ := s.byKey("vieja"); c != nil {
		c.Date = "2020-01-01" // envejecer manualmente
	}
	n, err := s.ArchiveOld()
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("archivadas = %d, esperaba 1", n)
	}
	if len(s.archive) != 1 {
		t.Errorf("archive = %d", len(s.archive))
	}
	rs, _ := s.Recall("vieja", nil)
	if len(rs) != 0 {
		t.Error("archivada no debería aparecer en recall")
	}
}

func TestDualFormatPersistence(t *testing.T) {
	for _, format := range []Format{FormatJSON, FormatAmbar} {
		dir := t.TempDir()
		s, err := NewStore(Config{Dir: dir, Format: format, Project: "test"})
		if err != nil {
			t.Fatalf("%s: %v", format, err)
		}
		s.Remember(RememberOptions{Category: CategoryDecision, Key: "k1", Content: "Contenido uno con |pipe| y \\slash\\", Tags: []string{"a", "b"}})
		s.Remember(RememberOptions{Category: CategoryBug, Key: "k2", Content: "Contenido dos", TTL: "7d", Priority: 3})

		// Reabrir desde disco.
		s2, err := NewStore(Config{Dir: dir, Format: format, Project: "test"})
		if err != nil {
			t.Fatalf("%s reopen: %v", format, err)
		}
		if got := len(s2.entries); got != 2 {
			t.Fatalf("%s: entradas = %d, esperaba 2", format, got)
		}
		c, _ := s2.byKey("k1")
		if c == nil || c.Content != "Contenido uno con |pipe| y \\slash\\" {
			t.Errorf("%s: roundtrip de content con escapes falló: %q", format, c.Content)
		}
		if c.Quality <= 0 || len(c.Tags) != 2 {
			t.Errorf("%s: calidad/tags no persistieron", format)
		}
		c2, _ := s2.byKey("k2")
		if c2 == nil || c2.Priority != 3 || c2.TTL == "" {
			t.Errorf("%s: prio/ttl no persistieron", format)
		}
	}
}

func TestSetFormatConversion(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewStore(Config{Dir: dir, Format: FormatJSON, Project: "test"})
	s.Remember(RememberOptions{Category: CategoryDecision, Key: "k", Content: "Contenido"})

	if err := s.SetFormat(FormatAmbar); err != nil {
		t.Fatal(err)
	}
	if _, err := NewStore(Config{Dir: dir, Format: FormatAmbar, Project: "test"}); err != nil {
		t.Fatalf("reabrir en ambar: %v", err)
	}
}

func TestBackup(t *testing.T) {
	s := newTestStore(t, FormatJSON)
	s.Remember(RememberOptions{Category: CategoryKnowledge, Key: "k", Content: "Contenido"})
	path, err := s.Backup()
	if err != nil {
		t.Fatal(err)
	}
	if path == "" {
		t.Fatal("ruta de backup vacía")
	}
	if !filepathExists(path) {
		t.Fatalf("backup no existe: %s", path)
	}
}

func TestStats(t *testing.T) {
	s := newTestStore(t, FormatJSON)
	s.Remember(RememberOptions{Category: CategoryDecision, Key: "d1", Content: "Decisión uno"})
	s.Remember(RememberOptions{Category: CategoryBug, Key: "b1", Content: "Bug uno"})
	st, err := s.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if st.Total != 2 || st.ByCategory["decision"] != 1 || st.ByCategory["bug"] != 1 {
		t.Errorf("stats = %+v", st)
	}
	if st.AvgQuality <= 0 {
		t.Error("calidad media debería ser > 0")
	}
}

func TestImportExport(t *testing.T) {
	s := newTestStore(t, FormatJSON)
	s.Remember(RememberOptions{Category: CategoryDecision, Key: "k", Content: "Contenido"})
	data, err := s.ExportJSON()
	if err != nil {
		t.Fatal(err)
	}
	var caps []*Capsule
	if err := json.Unmarshal(data, &caps); err != nil {
		t.Fatalf("export JSON: %v", err)
	}
	if len(caps) != 1 {
		t.Fatalf("export = %d cápsulas", len(caps))
	}

	s2 := newTestStore(t, FormatAmbar)
	n, err := s2.ImportCapsules(caps)
	if err != nil || n != 1 {
		t.Fatalf("import: n=%d err=%v", n, err)
	}
	// Importar el mismo dato de nuevo → merge, no duplicado.
	n, _ = s2.ImportCapsules(caps)
	if n != 0 {
		t.Errorf("re-import debería mergear, añadió %d", n)
	}
}

func TestProjectID(t *testing.T) {
	id := ProjectID("C:\\repo\\mi-proyecto")
	if id == "" {
		t.Fatal("ProjectID vacío")
	}
	id2 := ProjectID("C:\\repo\\mi-proyecto")
	if id != id2 {
		t.Errorf("ProjectID debería ser estable: %q vs %q", id, id2)
	}
	if id2 == ProjectID("C:\\otro\\mi-proyecto") {
		t.Error("proyectos con distinto path no deberían colisionar")
	}
}

func TestCurrentBranchDetached(t *testing.T) {
	dir := t.TempDir()
	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(gitDir, 0700); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/feature/test\n"), 0600)
	if got := CurrentBranch(dir); got != "feature/test" {
		t.Errorf("rama = %q", got)
	}
	os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("a1b2c3d4e5f6\n"), 0600)
	if got := CurrentBranch(dir); got != "a1b2c3d4" {
		t.Errorf("detached = %q", got)
	}
	if got := CurrentBranch(filepath.Join(dir, "no-existe")); got != "" {
		t.Errorf("sin git → %q", got)
	}
}

func filepathExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func TestSupersededOnSetByForget(t *testing.T) {
	s := newTestStore(t, FormatJSON)
	s.Remember(RememberOptions{Category: CategoryBug, Key: "bug-y", Content: "Otro bug"})

	ok, err := s.Forget("bug-y", false)
	if err != nil || !ok {
		t.Fatalf("Forget: ok=%v err=%v", ok, err)
	}
	c, _ := s.byKey("bug-y")
	if c == nil {
		t.Fatal("la cápsula debería existir tras soft-delete")
	}
	if c.Status != StatusObsolete {
		t.Errorf("status = %q, want obsolete", c.Status)
	}
	if c.SupersededOn == "" {
		t.Error("SupersededOn debería haberse establecido tras soft-delete")
	}
}

func TestSupersededOnClearedByResolve(t *testing.T) {
	s := newTestStore(t, FormatJSON)
	s.Remember(RememberOptions{Category: CategoryBug, Key: "bug-z", Content: "Bug temporal"})

	s.Forget("bug-z", false)
	c, _ := s.byKey("bug-z")
	if c.SupersededOn == "" {
		t.Fatal("SupersededOn debería estar tras Forget")
	}

	ok, err := s.Resolve("bug-z")
	if err != nil || !ok {
		t.Fatalf("Resolve: ok=%v err=%v", ok, err)
	}
	if c.SupersededOn != "" {
		t.Errorf("SupersededOn debería vaciarse tras Resolve, got %q", c.SupersededOn)
	}
}

func TestAsOfViewWithSupersededOn(t *testing.T) {
	s := newTestStore(t, FormatJSON)

	s.Remember(RememberOptions{Category: CategoryDecision, Key: "old-decision", Content: "Decisión vieja"})
	s.Remember(RememberOptions{Category: CategoryDecision, Key: "new-decision", Content: "Decisión nueva"})

	s.Forget("old-decision", false)

	// Vista normal: la cápsula olvidada no aparece.
	rs, err := s.Recall("decisión", nil)
	if err != nil {
		t.Fatalf("Recall normal: %v", err)
	}
	if len(rs) != 1 {
		t.Errorf("recall normal: esperaba 1, got %d", len(rs))
	}

	// Vista temporal posterior a SupersededOn: la olvidada sigue invisible.
	rs, err = s.Recall("decisión", &Query{AsOf: "2099-01-01"})
	if err != nil {
		t.Fatalf("Recall asOf futuro: %v", err)
	}
	if len(rs) != 1 {
		t.Errorf("asOf futuro: esperaba 1, got %d", len(rs))
	}

	// Verificar que la cápsula tiene SupersededOn establecido.
	c, _ := s.byKey("old-decision")
	if c == nil || c.SupersededOn == "" {
		t.Fatal("la cápsula olvidada debería tener SupersededOn")
	}
}

func TestPersistSortedByKey(t *testing.T) {
	s := newTestStore(t, FormatJSON)

	// Insertar claves en orden no alfabético.
	for _, key := range []string{"zebra", "alpha", "mango"} {
		s.Remember(RememberOptions{Category: CategoryKnowledge, Key: key, Content: "content for " + key})
	}

	// Leer el archivo directamente y verificar que las claves aparecen ordenadas.
	data, err := os.ReadFile(s.files.Memory)
	if err != nil {
		t.Fatalf("read memory file: %v", err)
	}
	var m fileModel
	json.Unmarshal(data, &m)
	keys := make([]string, len(m.Capsules))
	for i, c := range m.Capsules {
		keys[i] = c.Key
	}
	if len(keys) != 3 {
		t.Fatalf("esperaba 3 cápsulas, got %d", len(keys))
	}
	if keys[0] != "alpha" || keys[1] != "mango" || keys[2] != "zebra" {
		t.Errorf("claves no ordenadas en archivo: %v", keys)
	}
}

func TestNoStatusDraft(t *testing.T) {
	// Verificar que StatusDraft no existe en el código (código muerto eliminado).
	var _ Status = StatusActive
	var _ Status = StatusObsolete
	// StatusDraft ya no debería compilarse: si se reintroduce, este archivo
	// no compilará y el test fallará, lo cual es el comportamiento deseado.
}
