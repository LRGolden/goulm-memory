package memory

import (
	"testing"
)

func TestConsolidateSameKey(t *testing.T) {
	s := newTestStore(t, FormatJSON)
	s.Remember(RememberOptions{Category: CategoryPattern, Key: "dup", Content: "Contenido A", Tags: []string{"x"}})
	s.Remember(RememberOptions{Category: CategoryPattern, Key: "dup", Content: "Contenido B", Tags: []string{"y"}})
	// La segunda escritura ya mergea por clave en Remember; fuerza un duplicado
	// directo en el mapa para probar la fase 1 del consolidator.
	c := s.entries[firstKey(s)]
	c2 := c.Clone()
	c2.ID = NewID()
	s.entries[c2.ID] = c2

	rep, err := s.Consolidate()
	if err != nil {
		t.Fatal(err)
	}
	if rep.Merged == 0 {
		t.Error("debería haber merges por clave")
	}
	if rep.After >= rep.Before {
		t.Errorf("tras consolidar debería reducir: before=%d after=%d", rep.Before, rep.After)
	}
}

func TestConsolidateNearDuplicates(t *testing.T) {
	s := newTestStore(t, FormatJSON)
	s.Remember(RememberOptions{Category: CategoryBug, Key: "bug-a", Content: "El pool de redis se agota con muchas conexiones abiertas a la vez"})
	s.Remember(RememberOptions{Category: CategoryBug, Key: "bug-b", Content: "El pool de redis se agota con muchas conexiones abiertas simultáneamente"})

	if len(s.entries) != 2 {
		t.Fatalf("setup: %d entradas", len(s.entries))
	}
	rep, err := s.Consolidate()
	if err != nil {
		t.Fatal(err)
	}
	if rep.NearDuplicates == 0 {
		t.Fatal("near-duplicates no detectados")
	}
	if len(s.entries) != 1 {
		t.Errorf("entradas = %d, esperaba 1", len(s.entries))
	}
}

func TestConsolidateDifferentCategoriesKeep(t *testing.T) {
	s := newTestStore(t, FormatJSON)
	s.Remember(RememberOptions{Category: CategoryBug, Key: "bug", Content: "El pool de redis se agota con muchas conexiones abiertas a la vez"})
	s.Remember(RememberOptions{Category: CategoryKnowledge, Key: "info", Content: "El pool de redis se agota con muchas conexiones abiertas a la vez"})
	rep, err := s.Consolidate()
	if err != nil {
		t.Fatal(err)
	}
	if rep.NearDuplicates != 0 {
		t.Error("categorías distintas no deberían fusionarse")
	}
	if len(s.entries) != 2 {
		t.Errorf("entradas = %d, esperaba 2", len(s.entries))
	}
}

func TestJaccard(t *testing.T) {
	a := "El pool de redis se agota con muchas conexiones abiertas"
	b := "El pool de redis se agota con muchas conexiones abiertas"
	if got := Jaccard(a, b); got != 1.0 {
		t.Errorf("idénticos → %.2f, esperaba 1", got)
	}
	if got := Jaccard("hola mundo", "adios"); got != 0 {
		t.Errorf("disjuntos → %.2f, esperaba 0", got)
	}
	if got := Jaccard(a, a+" adicionales palabras nuevas"); got < 0.5 {
		t.Errorf("parciales → %.2f, esperaba > 0.5", got)
	}
}

func TestMergeCapsules(t *testing.T) {
	a := &Capsule{ID: "a", Key: "k", Category: CategoryDecision, Content: "viejo", Tags: []string{"x"}, Confidence: 0.6, Origin: OriginAgent, Priority: 1, Status: StatusObsolete, Date: "2026-01-01", Accessed: 1}
	b := &Capsule{ID: "b", Key: "k", Category: CategoryDecision, Content: "nuevo", Tags: []string{"y"}, Confidence: 1.0, Origin: OriginHuman, Priority: 2, Status: StatusActive, Date: "2026-02-01", Accessed: 3}
	m := MergeCapsules(a, b)
	if m.ID != "a" {
		t.Error("el merge debería conservar el ID del existente")
	}
	if m.Content != "viejo" {
		t.Errorf("content = %q: longitud igual → conserva el existente", m.Content)
	}
	if len(m.Tags) != 2 {
		t.Errorf("tags = %v", m.Tags)
	}
	if m.Confidence != 1.0 || m.Priority != 2 || m.Origin != OriginHuman {
		t.Error("max confidence/priority y origin ranking fallaron")
	}
	if m.Status != StatusActive {
		t.Error("status activo debería ganar")
	}
	if m.Date != "2026-02-01" || m.Accessed != 4 {
		t.Error("fecha más reciente y suma de accesos fallaron")
	}
}

func TestMergeCapsulesContentDetail(t *testing.T) {
	a := &Capsule{ID: "a", Key: "k", Category: CategoryDecision, Content: "resumen corto", Status: StatusActive}
	b := &Capsule{ID: "b", Key: "k", Category: CategoryDecision, Content: "detalle extenso con mucha más información útil para el futuro", Status: StatusActive}
	m := MergeCapsules(a, b)
	if m.Content != b.Content {
		t.Errorf("el entrante más detallado debería ganar, content = %q", m.Content)
	}
	m2 := MergeCapsules(b, a)
	if m2.Content != b.Content {
		t.Errorf("un re-guardado corto no debe perder el detalle previo, content = %q", m2.Content)
	}
}

func firstKey(s *MemoryStore) string {
	for k := range s.entries {
		return k
	}
	return ""
}
