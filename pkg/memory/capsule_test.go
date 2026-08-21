package memory

import (
	"testing"
	"time"
)

func dateOf(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return t
}

func TestNewCapsuleValidation(t *testing.T) {
	if _, err := NewCapsule(CategoryDecision, "use-zod", "Usar Zod"); err != nil {
		t.Fatalf("cápsula válida rechazada: %v", err)
	}
	if _, err := NewCapsule(CategoryDecision, "", "contenido"); err == nil {
		t.Fatal("clave vacía debería fallar")
	}
	if _, err := NewCapsule(CategoryDecision, "con:colon", "contenido"); err == nil {
		t.Fatal("clave con colon debería fallar (reservado para typed links)")
	}
	if _, err := NewCapsule("invalida", "key", "contenido"); err == nil {
		t.Fatal("categoría inválida debería fallar")
	}
	if _, err := NewCapsule(CategoryDecision, "key", "   "); err == nil {
		t.Fatal("contenido vacío debería fallar")
	}
}

func TestResolveTTL(t *testing.T) {
	now := dateOf("2026-07-31")
	if got := ResolveTTL("7d", now); got != "2026-08-07" {
		t.Errorf("7d → %q, esperaba 2026-08-07", got)
	}
	if got := ResolveTTL("30d", now); got != "2026-08-30" {
		t.Errorf("30d → %q, esperaba 2026-08-30", got)
	}
	if got := ResolveTTL("2026-09-01", now); got != "2026-09-01" {
		t.Errorf("absoluta → %q, esperaba 2026-09-01", got)
	}
	if got := ResolveTTL("invalido", now); got != "" {
		t.Errorf("inválido → %q, esperaba vacío", got)
	}
	if got := ResolveTTL("0d", now); got != "" {
		t.Errorf("0d → %q, esperaba vacío", got)
	}
}

func TestIsExpired(t *testing.T) {
	now := dateOf("2026-07-31")
	c := &Capsule{TTL: "2026-07-30"}
	if !c.IsExpired(now) {
		t.Fatal("TTL anterior debería estar expirado")
	}
	c.TTL = "2026-07-31"
	if c.IsExpired(now) {
		t.Fatal("TTL del mismo día no está expirado")
	}
	c.TTL = ""
	if c.IsExpired(now) {
		t.Fatal("sin TTL nunca expira")
	}
}

func TestIsVisibleAsOf(t *testing.T) {
	now := dateOf("2026-07-31")
	c := &Capsule{
		Status:       StatusObsolete,
		SupersededOn: "2026-07-30",
		Date:         "2026-07-01",
	}
	if c.IsVisible(now, "") {
		t.Fatal("obsolete no debería ser visible sin as_of")
	}
	if !c.IsVisible(now, "2026-07-29") {
		t.Fatal("con as_of anterior a la supersesión debería ser visible")
	}
	if c.IsVisible(now, "2026-07-31") {
		t.Fatal("con as_of posterior a la supersesión no debería ser visible")
	}
}

func TestBumpAccess(t *testing.T) {
	c := &Capsule{}
	now := dateOf("2026-07-31")
	c.BumpAccess(now)
	if c.Accessed != 1 {
		t.Fatalf("accessed = %d, esperaba 1", c.Accessed)
	}
	if c.LastAccessed == "" {
		t.Fatal("last_accessed debería fijarse")
	}
}

func TestNormalized(t *testing.T) {
	c := &Capsule{Content: "  Redis   pool  fix\n"}
	if got := c.Normalized(); got != "redis pool fix" {
		t.Errorf("normalizado = %q", got)
	}
}
