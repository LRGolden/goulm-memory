package memory

import (
	"testing"
	"time"
)

func TestTokenize(t *testing.T) {
	cases := map[string][]string{
		"redisPoolFix":     {"redis", "pool", "fix"},
		"Redis Pool Fix":   {"redis", "pool", "fix"},
		"kebab-case stuff": {"kebab", "case", "stuff"},
		"snake_case_name":  {"snake", "case", "name"},
		"a b c":            nil, // tokens de 1 char se descartan
	}
	for in, want := range cases {
		got := tokenize(in)
		if len(got) != len(want) {
			t.Errorf("tokenize(%q) = %v, esperaba %v", in, got, want)
			continue
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("tokenize(%q)[%d] = %q, esperaba %q", in, i, got[i], want[i])
			}
		}
	}
}

func TestBM25RanksRelevantFirst(t *testing.T) {
	docs := []*Capsule{
		{Key: "redis-cache", Content: "Capa de caché Redis para sesiones"},
		{Key: "auth-jwt", Content: "Autenticación con JWT"},
		{Key: "postgres-db", Content: "Postgres para almacenamiento"},
	}
	bm := BM25Scores("redis caché", docs)
	if bm["redis-cache"] <= 0 {
		t.Fatal("el documento relevante debería puntuar > 0")
	}
	if bm["redis-cache"] <= bm["auth-jwt"] {
		t.Error("BM25 debería dar más peso al documento con los términos")
	}
	if bm["postgres-db"] != 0 {
		t.Error("sin términos compartidos, el score debería ser 0")
	}
}

func TestRRF(t *testing.T) {
	// Dos rankers con órdenes parcialmente distintos.
	r1 := map[string]float64{"a": 3, "b": 2, "c": 1}
	r2 := map[string]float64{"b": 3, "c": 2, "d": 1}
	sa := rrfScore([]map[string]float64{r1, r2}, "a", 4)
	sb := rrfScore([]map[string]float64{r1, r2}, "b", 4)
	sc := rrfScore([]map[string]float64{r1, r2}, "c", 4)
	sd := rrfScore([]map[string]float64{r1, r2}, "d", 4)
	if !(sb > sa && sa > sc && sc > sd) {
		t.Errorf("orden RRF incorrecto: a=%.4f b=%.4f c=%.4f d=%.4f", sa, sb, sc, sd)
	}
}

func TestLevenshtein(t *testing.T) {
	if levenshtein("redis", "redis") != 0 {
		t.Fatal("idénticas → 0")
	}
	if levenshtein("redis", "redix") != 1 {
		t.Fatal("1 sustitución → 1")
	}
	if levenshtein("cat", "cats") != 1 {
		t.Fatal("1 inserción → 1")
	}
	if levenshtein("", "abc") != 3 {
		t.Fatal("vacía → len")
	}
}

func TestImportance(t *testing.T) {
	now := dateOf("2026-07-31")
	recent := &Capsule{Date: "2026-07-30", Accessed: 5}
	old := &Capsule{Date: "2025-01-01", Accessed: 0}
	if Importance(recent, now) <= Importance(old, now) {
		t.Error("cápsula reciente y accedida debería puntuar más que una vieja olvidada")
	}
	if got := Importance(old, now); got != 0 {
		t.Errorf("cápsula vieja sin accesos → 0, got %.2f", got)
	}
}

func TestQualityBounds(t *testing.T) {
	now := dateOf("2026-07-31")
	rich, _ := NewCapsule(CategoryDecision, "rich-key", "Contenido detallado con varias palabras únicas específicas del dominio.")
	rich.Tags = []string{"a", "b", "c"}
	rich.Links = []string{"x", "y"}
	rich.Origin = OriginHuman
	rich.Accessed = 5
	rich.LastAccessed = "2026-07-31T10:00:00Z"
	q := QualityScore(rich, now)
	if q > 1 || q < 0.5 {
		t.Errorf("cápsula rica debería puntuar alto (0.5-1), got %.2f", q)
	}
	poor, _ := NewCapsule(CategoryKnowledge, "poor-key", "x")
	poor.Origin = OriginInferred
	poor.LastAccessed = "2025-01-01T00:00:00Z"
	if qp := QualityScore(poor, now); qp >= q {
		t.Errorf("cápsula pobre no debería superar a la rica (%.2f >= %.2f)", qp, q)
	}
}

// TestSessionBias verifica el sesgo de sesión del ranking: una cápsula que
// referencia un archivo tocado por la sesión actual puntúa +15% (×1.15) frente
// a una idéntica que referencia otro archivo.
func TestSessionBias(t *testing.T) {
	s := newTestStore(t, FormatJSON)
	mk := func(key, file string) {
		if _, err := s.Remember(RememberOptions{Category: CategoryKnowledge, Key: key, Content: "documentación del módulo de auth con detalle", File: file}); err != nil {
			t.Fatal(err)
		}
	}
	mk("auth-actual", "src/auth.ts")
	mk("auth-otro", "src/legacy.ts")

	query := "documentación del módulo de auth"
	now := time.Now()

	base, err := s.Rank(RankOptions{Query: Query{Text: query, Limit: 6}, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if len(base) != 2 {
		t.Fatalf("esperaba 2 resultados base, got %d", len(base))
	}
	// Sin sesgo, ambas deberían puntuar igual (contenido idéntico).
	if base[0].Score != base[1].Score {
		t.Fatalf("sin sesgo los scores deberían ser iguales: %.4f vs %.4f", base[0].Score, base[1].Score)
	}

	biased, err := s.Rank(RankOptions{
		Query: Query{Text: query, Limit: 6, SessionFiles: map[string]bool{"src/auth.ts": true}},
		Now:   now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(biased) != 2 {
		t.Fatalf("esperaba 2 resultados con sesgo, got %d", len(biased))
	}
	if biased[0].Capsule.Key != "auth-actual" {
		t.Errorf("con sesgo debería ganar auth-actual, got %s", biased[0].Capsule.Key)
	}
	// El sesgo es multiplicativo ×1.15 sobre el score de la cápsula tocada.
	// Ambas recibieron el mismo bump de accesos entre las dos llamadas, así que
	// la razón auth-actual / auth-otro debe ser ~1.15.
	if ratio := biased[0].Score / biased[1].Score; ratio < 1.149 || ratio > 1.151 {
		t.Errorf("ratio del sesgo = %.4f, esperaba ~1.15", ratio)
	}
}
