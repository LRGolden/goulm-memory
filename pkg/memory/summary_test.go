package memory

import (
	"strings"
	"testing"
	"time"
)

func TestSummarySections(t *testing.T) {
	l, _ := newTestLedger(t, true)
	now := time.Now()

	l.Append(LedgerEvent{TS: now.Add(-2 * time.Hour).UTC().Format(time.RFC3339), Type: EventEdit, Action: "edit_file", Path: "src/auth.go"})
	l.Append(LedgerEvent{TS: now.Add(-1 * time.Hour).UTC().Format(time.RFC3339), Type: EventCommit, Action: "commit", Hash: "abcd1234", Detail: "fix: pool"})
	l.Append(LedgerEvent{TS: now.UTC().Format(time.RFC3339), Type: EventMemory, Action: "remember", Path: "use-zod", Detail: "decision"})
	l.Append(LedgerEvent{TS: now.AddDate(0, 0, -1).UTC().Format(time.RFC3339), Type: EventMemory, Action: "remember", Path: "redis-pool-fix", Detail: "bug"})
	l.Append(LedgerEvent{TS: now.AddDate(0, -2, 0).UTC().Format(time.RFC3339), Type: EventMilestone, Action: "mark", Detail: "release v0.2"})

	s := l.Summary()
	if s == "" {
		t.Fatal("summary vacío")
	}
	for _, want := range []string{"# Sucesos", "Últimos 7 días", "90 días", "Histórico", "Total", "src/auth.go", "abcd1234", "1 decision", "1 bug", "1 hitos"} {
		if !strings.Contains(s, want) {
			t.Fatalf("summary debe contener %q\n%s", want, s)
		}
	}
	if strings.Contains(s, "use-zod") {
		t.Fatalf("el digest no debe incluir keys de memoria crudas:\n%s", s)
	}
}

func TestSummaryExcludesTestEdits(t *testing.T) {
	l, _ := newTestLedger(t, true)
	now := time.Now()
	l.Append(LedgerEvent{TS: now.UTC().Format(time.RFC3339), Type: EventEdit, Action: "edit_file", Path: "test-helper.go", Test: true})
	l.Append(LedgerEvent{TS: now.UTC().Format(time.RFC3339), Type: EventEdit, Action: "edit_file", Path: "real.go"})
	s := l.Summary()
	if strings.Contains(s, "test-helper.go") {
		t.Fatalf("los edits de test no deben entrar al summary:\n%s", s)
	}
	if !strings.Contains(s, "real.go") {
		t.Fatalf("el edit real debe estar en el summary:\n%s", s)
	}
}

func TestSummaryBudget(t *testing.T) {
	l, _ := newTestLedger(t, true)
	now := time.Now()
	for i := 0; i < 300; i++ {
		ts := now.Add(-time.Duration(i) * time.Hour).UTC().Format(time.RFC3339)
		l.Append(LedgerEvent{TS: ts, Type: EventEdit, Action: "edit_file", Path: "src/file-" + strings.Repeat("x", 40) + ".go"})
	}
	s := l.Summary()
	if len([]rune(s)) > SummaryBudget {
		t.Fatalf("summary excede el presupuesto: %d > %d", len([]rune(s)), SummaryBudget)
	}
}

func TestSummaryCompactConsistent(t *testing.T) {
	l, _ := newTestLedger(t, true)
	now := time.Now()
	for i := 0; i < 30; i++ {
		ts := now.Add(-time.Duration(30-i) * time.Hour).UTC().Format(time.RFC3339)
		l.Append(LedgerEvent{TS: ts, Type: EventEdit, Action: "edit_file", Path: "f.go"})
	}
	before := l.Summary()
	l.CompactNow()
	after := l.Summary()
	if strings.Count(before, "30 eventos") != strings.Count(after, "30 eventos") {
		t.Fatalf("el resumen no es estable tras compactar:\nANTES:\n%s\nDESPUÉS:\n%s", before, after)
	}
}

func TestSummaryEmptyLedger(t *testing.T) {
	l, _ := newTestLedger(t, true)
	s := l.Summary()
	if !strings.Contains(s, "# Sucesos") {
		t.Fatalf("summary vacío debe tener cabecera:\n%s", s)
	}
}
