package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestConcurrentStores simula dos procesos sobre el mismo directorio: con el
// lockfile + adopción, ninguna cápsula del otro proceso se pierde.
func TestConcurrentStores(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{Dir: dir, Format: FormatJSON, Project: "concurrente"}
	a, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("store A: %v", err)
	}
	b, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("store B: %v", err)
	}

	keys := []string{"k-a1", "k-b1", "k-a2", "k-b2", "k-a3", "k-b3", "k-a4", "k-b4"}
	for _, k := range keys {
		var s *MemoryStore
		if strings.HasPrefix(k, "k-a") {
			s = a
		} else {
			s = b
		}
		if _, err := s.Remember(RememberOptions{Category: CategoryKnowledge, Key: k, Content: "contenido de " + k}); err != nil {
			t.Fatalf("remember %s: %v", k, err)
		}
	}

	c, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("store C: %v", err)
	}
	if got := len(c.ListActive(0)); got != 8 {
		t.Errorf("tras la concurrencia hay %d cápsulas, esperaba 8", got)
	}
	for _, k := range keys {
		if _, ok := c.byKey(k); !ok {
			t.Errorf("cápsula %s perdida en la concurrencia", k)
		}
	}
}

// TestAdoptForeignDedupByKey verifica que adoptForeignLocked no duplique una
// cápsula que un proceso ajeno escribió con la MISMA clave pero DISTINTO ID:
// la identidad lógica es la clave, así que la ajena se descarta (last-writer
// gana para el contenido local ya presente) y los mapas indexados por ID se
// mantienen consistentes.
func TestAdoptForeignDedupByKey(t *testing.T) {
	s := newTestStore(t, FormatJSON)
	if _, err := s.Remember(RememberOptions{Category: CategoryKnowledge, Key: "k1", Content: "contenido local"}); err != nil {
		t.Fatal(err)
	}
	localID := s.byKeyIdx["k1"]
	if localID == "" {
		t.Fatal("índice key→ID vacío tras remember")
	}

	foreign := []*Capsule{{ID: "foreign-id-diferente", Key: "k1", Category: CategoryKnowledge, Content: "contenido ajeno"}}
	if err := os.WriteFile(s.files.Memory, s.encode(foreign), 0600); err != nil {
		t.Fatal(err)
	}
	s.memStamp = fileStamp{} // forzar adopción

	s.adoptForeignLocked()

	if len(s.entries) != 1 {
		t.Fatalf("entries = %d, esperaba 1 (duplicado por clave)", len(s.entries))
	}
	if _, ok := s.entries[localID]; !ok {
		t.Error("la cápsula local original se perdió del mapa por ID")
	}
	if _, ok := s.byKey("k1"); !ok {
		t.Error("byKey(k1) falló tras la adopción")
	} else if s.byKeyIdx["k1"] != localID {
		t.Error("el índice key→ID fue sobreescrito por la cápsula ajena")
	}
}

// TestRankDoesNotPersist verifica que un recall no escriba a disco; solo Flush.
func TestRankDoesNotPersist(t *testing.T) {
	s := newTestStore(t, FormatJSON)
	if _, err := s.Remember(RememberOptions{Category: CategoryKnowledge, Key: "k1", Content: "algo importante de memoria"}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(s.files.Memory)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Rank(RankOptions{Query: Query{Text: "memoria", Limit: 3}, Now: time.Now()}); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(s.files.Memory)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Error("Rank escribió a disco: los bumps deben diferirse a Flush")
	}
	if !s.dirty {
		t.Error("Rank debería marcar dirty")
	}
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	if s.dirty {
		t.Error("Flush debería limpiar dirty")
	}
	flushed, err := os.ReadFile(s.files.Memory)
	if err != nil {
		t.Fatal(err)
	}
	if string(flushed) == string(after) {
		t.Error("Flush no persistió los bumps")
	}
}

// TestGraphCache verifica que el grafo/centralidad se reutilicen entre recalls
// sin mutaciones y se reconstruyan tras una escritura.
func TestGraphCache(t *testing.T) {
	s := newTestStore(t, FormatJSON)
	for _, k := range []string{"a", "b"} {
		if _, err := s.Remember(RememberOptions{Category: CategoryKnowledge, Key: k, Content: "contenido " + k, Tags: []string{"x"}}); err != nil {
			t.Fatal(err)
		}
	}
	opts := RankOptions{Query: Query{Text: "contenido", Limit: 2, Graph: true}, Now: time.Now()}
	if _, err := s.Rank(opts); err != nil {
		t.Fatal(err)
	}
	g1 := s.cachedGraph
	if g1 == nil {
		t.Fatal("el primer Rank debería poblar la cache")
	}
	if _, err := s.Rank(opts); err != nil {
		t.Fatal(err)
	}
	if s.cachedGraph != g1 {
		t.Error("Rank sin mutaciones no debería reconstruir el grafo")
	}
	if _, err := s.Remember(RememberOptions{Category: CategoryKnowledge, Key: "c", Content: "tercera cápsula", Tags: []string{"x"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Rank(opts); err != nil {
		t.Fatal(err)
	}
	if s.cachedGraph == g1 {
		t.Error("tras una mutación el grafo debería reconstruirse")
	}
}

func TestMatchKeyword(t *testing.T) {
	cases := []struct {
		text string
		kw   string
		want bool
	}{
		{"el servidor rapidamente se agota", "api", false},
		{"el catálogo de productos", "log", false},
		{"use-zod para validación", "zod", true},
		{"sistema de logging centralizado", "log", true},
		{"abre un pull request ya", "pull request", true},
		{"abre pull requests ya", "pull request", true},
		{"validación de apikey", "api", true},
		{"pool de redis agotado", "redis", true},
		{"módulo de caché en memoria", "cache", true},
	}
	for _, c := range cases {
		if got := matchKeyword(c.text, c.kw); got != c.want {
			t.Errorf("matchKeyword(%q, %q) = %v, esperaba %v", c.text, c.kw, got, c.want)
		}
	}
}

func TestInferTagsWordBoundaries(t *testing.T) {
	got := InferTags("el catálogo quedó rapidamente lento tras el deploy", "perf-catalogo", nil)
	for _, tag := range got {
		if tag == "api" || tag == "logging" {
			t.Errorf("falso positivo: tag %q en %v", tag, got)
		}
	}
	auth := InferTags("el login usa token jwt", "auth-login", nil)
	if !contains(auth, "auth") {
		t.Errorf("debería inferir auth, got %v", auth)
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// TestDiffUpdated verifica que las cápsulas accedidas recientemente salgan en
// Updated (y no en New).
func TestDiffUpdated(t *testing.T) {
	s := newTestStore(t, FormatJSON)
	res, err := s.Remember(RememberOptions{Category: CategoryKnowledge, Key: "vieja", Content: "conocimiento antiguo"})
	if err != nil {
		t.Fatal(err)
	}
	res.Capsule.Date = time.Now().AddDate(0, 0, -10).Format("2006-01-02")
	res.Capsule.LastAccessed = time.Now().UTC().Format(time.RFC3339)

	if _, err := s.Remember(RememberOptions{Category: CategoryKnowledge, Key: "nueva", Content: "creada hoy"}); err != nil {
		t.Fatal(err)
	}

	rep, err := s.Diff("24h")
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.New) != 1 || rep.New[0].Key != "nueva" {
		t.Errorf("new = %v", keysOfCaps(rep.New))
	}
	if len(rep.Updated) != 1 || rep.Updated[0].Key != "vieja" {
		t.Errorf("updated = %v", keysOfCaps(rep.Updated))
	}
}

func keysOfCaps(caps []*Capsule) []string {
	out := make([]string, 0, len(caps))
	for _, c := range caps {
		out = append(out, c.Key)
	}
	return out
}

func TestRenderTruncatesRunes(t *testing.T) {
	content := strings.Repeat("a", 86) + "ñ" + strings.Repeat("b", 20)
	c := &Capsule{Category: CategoryKnowledge, Key: "k", Content: content, Tags: []string{"x"}}
	rs := []Ranked{{Capsule: c, Score: 1}}
	out := Render(rs, BudgetNormal)
	if strings.Contains(out, "\uFFFD") {
		t.Error("el truncado cortó un rune UTF-8 a la mitad")
	}
	if !strings.Contains(out, "ñ") {
		t.Error("el rune ñ debería conservarse entero")
	}
}

// TestTrimToMaxAllPinned verifica que el límite sea duro incluso si todas las
// cápsulas tienen prioridad.
func TestTrimToMaxAllPinned(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(Config{Dir: dir, Format: FormatJSON, Project: "pins", MaxEntries: 5})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 12; i++ {
		if _, err := s.Remember(RememberOptions{
			Category: CategoryKnowledge,
			Key:      fmt.Sprintf("pin-%d", i),
			Content:  fmt.Sprintf("cápsula fijada número %d", i),
			Priority: 1,
		}); err != nil {
			t.Fatal(err)
		}
	}
	if got := len(s.entries); got > 5 {
		t.Errorf("entries = %d, el límite duro de 5 no se cumplió", got)
	}
	if got := len(s.archive); got != 7 {
		t.Errorf("archive = %d, esperaba 7", got)
	}
}

// TestImportInvalidStatus verifica que Status/Origin inválidos se corrijan.
func TestImportInvalidStatus(t *testing.T) {
	s := newTestStore(t, FormatJSON)
	bad := &Capsule{Key: "malo", Category: CategoryKnowledge, Content: "import inválido", Status: "inventado", Origin: "extraterrestre"}
	added, err := s.ImportCapsules([]*Capsule{bad})
	if err != nil {
		t.Fatal(err)
	}
	if added != 1 {
		t.Fatalf("added = %d", added)
	}
	c, ok := s.byKey("malo")
	if !ok {
		t.Fatal("la cápsula no se importó")
	}
	if c.Status != StatusActive {
		t.Errorf("status = %q, esperaba active", c.Status)
	}
	if c.Origin != OriginAgent {
		t.Errorf("origin = %q, esperaba agent", c.Origin)
	}
	if bad.Status != "inventado" {
		t.Error("no se debe mutar el slice del llamador")
	}
}

// TestHealthColonPath verifica que "archivo:línea" no rompa archivos reales
// con dos puntos en el nombre.
func TestHealthColonPath(t *testing.T) {
	s := newTestStore(t, FormatJSON)
	dir := s.cfg.Dir
	realFile := filepath.Join(dir, "a:b.txt")
	if err := os.WriteFile(realFile, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	rel := "a:b.txt"
	if _, err := s.Remember(RememberOptions{Category: CategoryKnowledge, Key: "con-colon", Content: "archivo con colon", File: rel}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Remember(RememberOptions{Category: CategoryKnowledge, Key: "roto", Content: "archivo roto", File: "no-existe.go:42"}); err != nil {
		t.Fatal(err)
	}
	rep, err := s.Health(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, bf := range rep.BrokenFiles {
		if strings.Contains(bf, "con-colon") {
			t.Errorf("falso broken: %s", bf)
		}
	}
	found := false
	for _, bf := range rep.BrokenFiles {
		if strings.Contains(bf, "roto") {
			found = true
		}
	}
	if !found {
		t.Error("el archivo roto debería aparecer en BrokenFiles")
	}
}

// TestByKeyIndex verifica el índice key→ID tras persistencia y recarga.
func TestByKeyIndex(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(Config{Dir: dir, Format: FormatJSON, Project: "idx"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Remember(RememberOptions{Category: CategoryKnowledge, Key: "uno", Content: "primera"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Remember(RememberOptions{Category: CategoryKnowledge, Key: "dos", Content: "segunda"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.byKey("uno"); !ok {
		t.Fatal("byKey(uno) falló tras remember")
	}
	if _, err := s.Forget("dos", true); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.byKey("dos"); ok {
		t.Error("byKey(dos) debería fallar tras forget hard")
	}
	reloaded, err := NewStore(Config{Dir: dir, Format: FormatJSON, Project: "idx"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := reloaded.byKey("uno"); !ok {
		t.Error("byKey(uno) falló tras recargar desde disco")
	}
	if _, ok := reloaded.byKey("dos"); ok {
		t.Error("byKey(dos) encontró una cápsula borrada tras recargar")
	}
}

// TestHeartbeatFileLimit verifica que Files se pode a maxHeartbeatFiles.
func TestHeartbeatFileLimit(t *testing.T) {
	tr, err := NewSessionTracker(filepath.Join(t.TempDir(), "sessions"), "test")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < maxHeartbeatFiles+50; i++ {
		if err := tr.Touch(fmt.Sprintf("src/file-%d.go", i)); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(tr.path(tr.selfID))
	if err != nil {
		t.Fatal(err)
	}
	var hb Heartbeat
	if err := json.Unmarshal(data, &hb); err != nil {
		t.Fatal(err)
	}
	if len(hb.Files) != maxHeartbeatFiles {
		t.Errorf("files = %d, esperaba %d", len(hb.Files), maxHeartbeatFiles)
	}
}

// TestHeartbeatBranchRoot verifica que la rama se resuelva desde la raíz
// fijada con SetRoot.
func TestHeartbeatBranchRoot(t *testing.T) {
	root := t.TempDir()
	gitDir := filepath.Join(root, ".git")
	if err := os.MkdirAll(gitDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/feature-x\n"), 0600); err != nil {
		t.Fatal(err)
	}
	tr, err := NewSessionTracker(filepath.Join(t.TempDir(), "sessions"), "test")
	if err != nil {
		t.Fatal(err)
	}
	tr.SetRoot(root)
	if err := tr.Heartbeat("main.go", false); err != nil {
		t.Fatal(err)
	}
	if tr.branch != "feature-x" {
		t.Errorf("branch = %q, esperaba feature-x", tr.branch)
	}
}

// TestTrimToMaxUnpinnedFirst verifica que las fijadas se conservan cuando hay
// alternativas sin prioridad.
func TestTrimToMaxUnpinnedFirst(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(Config{Dir: dir, Format: FormatJSON, Project: "mix", MaxEntries: 4})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Remember(RememberOptions{Category: CategoryKnowledge, Key: "fijada", Content: "no se debe archivar", Priority: 3}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		if _, err := s.Remember(RememberOptions{Category: CategoryKnowledge, Key: fmt.Sprintf("libre-%d", i), Content: fmt.Sprintf("libre número %d", i)}); err != nil {
			t.Fatal(err)
		}
	}
	if _, ok := s.byKey("fijada"); !ok {
		t.Error("la cápsula fijada no debería archivarse si hay alternativas")
	}
	if got := len(s.entries); got > 4 {
		t.Errorf("entries = %d, esperaba ≤ 4", got)
	}
}
