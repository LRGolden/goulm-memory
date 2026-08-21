package memory

import (
	"strings"
	"testing"
)

func TestAmbarRoundtrip(t *testing.T) {
	now := dateOf("2026-07-31")
	c1, _ := NewCapsule(CategoryDecision, "use-zod", "Usar Zod para validación — más simple que Joi.")
	c1.File = "src/types.ts"
	c1.Tags = []string{"types", "validation"}
	c1.Date = "2026-07-10"
	c1.ApplyTTL("", now)
	c1.Accessed = 3
	c1.Quality = 0.72
	c1.Confidence = 1.0
	c1.Origin = OriginHuman
	c1.Links = []string{"zod-schemas"}

	c2, _ := NewCapsule(CategoryBug, "redis-pool-fix", "Pool agotado — fix max_connections=20 (ver [[use-zod]]).\nSegunda línea con |pipe| y \\backslash\\.")
	c2.Tags = []string{"redis", "fix"}
	c2.ApplyTTL("7d", now)
	c2.Date = "2026-07-18"
	c2.Priority = 2
	c2.PathScope = "pkg/**/*.go"

	data := MarshalAmbar("goulm-cli-test", []*Capsule{c1, c2})

	project, got, err := UnmarshalAmbar(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if project != "goulm-cli-test" {
		t.Errorf("proyecto = %q", project)
	}
	if len(got) != 2 {
		t.Fatalf("cápsulas = %d, esperaba 2", len(got))
	}
	g1 := got[0]
	if g1.Key != "use-zod" || g1.Category != CategoryDecision {
		t.Errorf("g1 key/cat = %q/%q", g1.Key, g1.Category)
	}
	if g1.Quality != 0.72 || g1.Confidence != 1.0 {
		t.Errorf("g1 q/c = %.2f/%.2f", g1.Quality, g1.Confidence)
	}
	if g1.Origin != OriginHuman {
		t.Errorf("g1 origin = %q", g1.Origin)
	}
	if len(g1.Links) != 1 || g1.Links[0] != "zod-schemas" {
		t.Errorf("g1 links = %v", g1.Links)
	}
	g2 := got[1]
	if g2.TTL != "2026-08-07" {
		t.Errorf("g2 ttl = %q, esperaba 2026-08-07", g2.TTL)
	}
	if !strings.Contains(g2.Content, "|pipe|") || !strings.Contains(g2.Content, "\\backslash\\") {
		t.Errorf("g2 content con escapes perdidos: %q", g2.Content)
	}
	if !strings.Contains(g2.Content, "\n") {
		t.Errorf("g2 saltos de línea perdidos: %q", g2.Content)
	}
	if g2.Priority != 2 || g2.PathScope != "pkg/**/*.go" {
		t.Errorf("g2 pri/scope = %d/%q", g2.Priority, g2.PathScope)
	}
}

func TestAmbarTolerance(t *testing.T) {
	// Archivo con campos faltantes y líneas desconocidas.
	data := `v:1|project:test|updated:2026-07-31T00:00:00Z|count:9
~
id:abc123|key:solo-esencial|cat:decision
content>Solo lo mínimo.
campo-desconocido>se ignora
~
id:def456|key:segunda
~
`
	_, caps, err := UnmarshalAmbar(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(caps) != 2 {
		t.Fatalf("cápsulas = %d, esperaba 2", len(caps))
	}
	if caps[0].Category != CategoryDecision || caps[0].Status != StatusActive {
		t.Errorf("defaults de cat/status: %q/%q", caps[0].Category, caps[0].Status)
	}
	if caps[1].Category != CategoryKnowledge {
		t.Errorf("categoría default = %q, esperaba knowledge", caps[1].Category)
	}
}

func TestAmbarEmpty(t *testing.T) {
	_, caps, err := UnmarshalAmbar("")
	if err != nil {
		t.Fatalf("parse vacío: %v", err)
	}
	if len(caps) != 0 {
		t.Fatalf("cápsulas = %d, esperaba 0", len(caps))
	}
}

func TestAmbarInvalidHeader(t *testing.T) {
	if _, _, err := UnmarshalAmbar("no es ambar"); err == nil {
		t.Fatal("cabecera inválida debería fallar")
	}
}
