package memory

import (
	"strings"
	"testing"
	"time"
)

func TestSummary_ErrorsAndTestsFlags(t *testing.T) {
	l, _ := newTestLedger(t, true)
	now := time.Now()
	l.Append(LedgerEvent{TS: now.UTC().Format(time.RFC3339), Type: EventError, Action: "run", Detail: "boom"})
	l.Append(LedgerEvent{TS: now.UTC().Format(time.RFC3339), Type: EventTest, Action: "test", Detail: "ok"})
	s := l.Summary()
	if !strings.Contains(s, "1 errores") {
		t.Errorf("summary debería incluir errores:\n%s", s)
	}
	if !strings.Contains(s, "1 tests") {
		t.Errorf("summary debería incluir tests:\n%s", s)
	}
}

func TestSummary_MilestoneTruncation(t *testing.T) {
	l, _ := newTestLedger(t, true)
	long := strings.Repeat("hito-muy-largo", 20)
	l.Append(LedgerEvent{TS: time.Now().UTC().Format(time.RFC3339), Type: EventMilestone, Detail: long})
	s := l.Summary()
	if !strings.Contains(s, "…") {
		t.Errorf("el hito largo debería truncarse:\n%s", s)
	}
}

func TestSummary_SingleEvents(t *testing.T) {
	l, _ := newTestLedger(t, true)
	now := time.Now()
	l.Append(LedgerEvent{TS: now.UTC().Format(time.RFC3339), Type: EventEdit, Action: "edit_file", Path: "a.go"})
	s := l.Summary()
	if !strings.Contains(s, "1 archivos") || !strings.Contains(s, "1 eventos") {
		t.Errorf("summary con un edit:\n%s", s)
	}
}

func TestSummary_ParseTSFallbacks(t *testing.T) {
	now := time.Now()
	// Formato sin zona horaria.
	t1 := parseTS(now.Format("2006-01-02T15:04:05"), now)
	if t1.IsZero() {
		t.Error("parseTS formato corto debería funcionar")
	}
	// Formato inválido → fallback.
	t2 := parseTS("no-es-fecha", now)
	if !t2.Equal(now) {
		t.Error("parseTS inválido debería devolver fallback")
	}
}

func TestParseWKeyBad(t *testing.T) {
	if got := parseWKey("no-valid"); !got.IsZero() {
		t.Errorf("parseWKey inválido = %v, want zero", got)
	}
}

func TestTruncateRunes(t *testing.T) {
	if got := truncateRunes("hola", 10); got != "hola" {
		t.Errorf("truncate corto = %q", got)
	}
	if got := truncateRunes("hola mundo", 4); got != "hola…" {
		t.Errorf("truncate largo = %q", got)
	}
}

func TestLastN(t *testing.T) {
	in := []string{"1", "2", "3", "4", "5"}
	if got := lastN(in, 2); len(got) != 2 || got[0] != "4" {
		t.Errorf("lastN(5,2) = %v", got)
	}
	if got := lastN(in, 20); len(got) != 5 {
		t.Errorf("lastN(5,20) = %v", got)
	}
}

func TestAggMergeAndLines(t *testing.T) {
	a := newAgg()
	b := newAgg()
	a.add(LedgerEvent{Type: EventEdit, Path: "x.go"})
	b.add(LedgerEvent{Type: EventEdit, Path: "y.go"})
	b.add(LedgerEvent{Type: EventCommit, Hash: "deadbeef"})
	b.add(LedgerEvent{Type: EventError})
	b.add(LedgerEvent{Type: EventTest})
	a.merge(b)
	if a.total != 5 {
		t.Errorf("total tras merge = %d, want 5", a.total)
	}
	if len(a.files()) != 2 {
		t.Errorf("files tras merge = %v", a.files())
	}
	if a.errors != 1 || a.tests != 1 {
		t.Errorf("errores/tests tras merge = %d/%d", a.errors, a.tests)
	}
	if len(a.commits) != 1 || a.commits[0] != "deadbeef" {
		t.Errorf("commits tras merge = %v", a.commits)
	}

	// memoryLine con categoría conocida.
	am := newAgg()
	am.add(LedgerEvent{Type: EventMemory, Detail: "decision"})
	am.add(LedgerEvent{Type: EventMemory, Detail: "bug"})
	if got := am.memoryLine(); got != "1 decision · 1 bug" {
		t.Errorf("memoryLine = %q", got)
	}
	// memoryLine con detalle no canónico.
	am2 := newAgg()
	am2.add(LedgerEvent{Type: EventMemory, Detail: "otra-cosa"})
	if got := am2.memoryLine(); got != "" {
		t.Errorf("memoryLine no canónico = %q, want vacío", got)
	}
	// memoryLine vacío.
	if got := newAgg().memoryLine(); got != "" {
		t.Errorf("memoryLine vacío = %q", got)
	}

	// milestoneLine.
	am3 := newAgg()
	am3.add(LedgerEvent{Type: EventMilestone, Detail: "hito"})
	if got := am3.milestoneLine(); got != "hito" {
		t.Errorf("milestoneLine = %q", got)
	}
	if got := newAgg().milestoneLine(); got != "" {
		t.Errorf("milestoneLine vacío = %q", got)
	}
}

func TestCompactLine(t *testing.T) {
	a := newAgg()
	a.add(LedgerEvent{Type: EventEdit, Path: "f.go"})
	a.add(LedgerEvent{Type: EventCommit, Hash: "abcd"})
	line := compactLine(a)
	if !strings.Contains(line, "1 archivos") || !strings.Contains(line, "1 commits") {
		t.Errorf("compactLine = %q", line)
	}
}

func TestTrimToBudgetNoNewline(t *testing.T) {
	// Cadena larga sin nuevas líneas → sufijo "truncado".
	s := strings.Repeat("a", SummaryBudget+100)
	out := trimToBudget(s)
	if !strings.Contains(out, "truncado") {
		t.Errorf("trimToBudget sin newline debería añadir sufijo, len=%d", len(out))
	}
}

func TestRenderDayErrorsFlags(t *testing.T) {
	var sb strings.Builder
	a := newAgg()
	a.add(LedgerEvent{Type: EventEdit, Path: "a.go"})
	a.add(LedgerEvent{Type: EventError})
	a.add(LedgerEvent{Type: EventTest})
	renderDay(&sb, "2026-08-09", a)
	out := sb.String()
	if !strings.Contains(out, "1 errores") || !strings.Contains(out, "1 tests") {
		t.Errorf("renderDay = %q", out)
	}
	// nil agg → no op.
	var sb2 strings.Builder
	renderDay(&sb2, "2026-08-09", nil)
	if sb2.String() != "" {
		t.Errorf("renderDay nil debería no-op")
	}
}

func TestSummary_DisabledLedger(t *testing.T) {
	t.Setenv("GOULM_LEDGER", "off")
	l, err := NewLedger(t.TempDir())
	if err != nil {
		t.Fatalf("NewLedger: %v", err)
	}
	if l.Enabled {
		t.Fatal("ledger debería estar deshabilitado con GOULM_LEDGER=off")
	}
	if s := l.Summary(); s != "" {
		t.Errorf("summary con ledger deshabilitado = %q, want vacío", s)
	}
}
