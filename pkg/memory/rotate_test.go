package memory

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCompactToMonthlyArchives(t *testing.T) {
	l, _ := newTestLedger(t, true)
	now := time.Now()
	base := now.Add(-40 * 24 * time.Hour)
	for i := 0; i < 50; i++ {
		ts := base.Add(time.Duration(i) * time.Hour).UTC().Format(time.RFC3339)
		l.Append(LedgerEvent{TS: ts, Type: EventEdit, Action: "edit_file", Path: "a.go", Detail: "x"})
	}
	if err := l.CompactNow(); err != nil {
		t.Fatalf("CompactNow: %v", err)
	}
	active := l.Tail(1000, "", false)
	if len(active) != l.Window {
		t.Fatalf("el activo debe tener %d eventos, tiene %d", l.Window, len(active))
	}
	archives := l.archivePaths()
	if len(archives) == 0 {
		t.Fatal("debe haber archivos mensuales tras compactar")
	}
	var archivedTotal int
	for _, p := range archives {
		archivedTotal += len(readEvents(p))
	}
	if archivedTotal != 50-l.Window {
		t.Fatalf("esperados %d eventos archivados, hay %d", 50-l.Window, archivedTotal)
	}
	all := l.allEvents()
	if len(all) != 50 {
		t.Fatalf("ningún evento debe perderse: esperados 50, hay %d", len(all))
	}
}

func TestCompactKeepsNewest(t *testing.T) {
	l, _ := newTestLedger(t, true)
	now := time.Now()
	for i := 0; i < 60; i++ {
		ts := now.Add(-time.Duration(60-i) * time.Minute).UTC().Format(time.RFC3339)
		l.Append(LedgerEvent{TS: ts, Type: EventEdit, Action: "edit_file", Path: "f.go", Detail: "x"})
	}
	l.CompactNow()
	active := l.Tail(1000, "", false)
	newestTS := now.Add(-time.Minute).UTC().Format(time.RFC3339)
	if active[0].TS < newestTS {
		t.Fatalf("el evento más reciente no está en el activo: %s", active[0].TS)
	}
}

func TestCompactRaceAppend(t *testing.T) {
	l, _ := newTestLedger(t, true)
	now := time.Now()
	for i := 0; i < 25; i++ {
		ts := now.Add(-time.Duration(25-i) * time.Minute).UTC().Format(time.RFC3339)
		l.Append(LedgerEvent{TS: ts, Type: EventEdit, Action: "edit_file", Path: "f.go", Detail: "x"})
	}
	l.Append(LedgerEvent{TS: now.UTC().Format(time.RFC3339), Type: EventMilestone, Action: "mark", Detail: "durante compactación"})
	for i := 0; i < 5; i++ {
		l.AppendEdit("edit_file", "g.go", "y", "s", false)
	}
	if err := l.CompactNow(); err != nil {
		t.Fatalf("CompactNow: %v", err)
	}
	all := l.allEvents()
	if len(all) != 31 {
		t.Fatalf("todos los eventos deben conservarse, hay %d", len(all))
	}
	active := l.Tail(1000, "", false)
	if len(active) > l.Window {
		t.Fatalf("el activo excede la ventana: %d", len(active))
	}
}

func TestCompactEmptyNoop(t *testing.T) {
	l, _ := newTestLedger(t, true)
	if err := l.CompactNow(); err != nil {
		t.Fatalf("compactar vacío: %v", err)
	}
	if _, err := os.Stat(l.Active); err == nil {
		t.Fatal("el activo no debe crearse si no hay eventos")
	}
}

func TestCompactDisabled(t *testing.T) {
	t.Setenv("GOULM_LEDGER", "off")
	l, _ := NewLedger(t.TempDir(), WithHome(t.TempDir()))
	if err := l.CompactNow(); err != ErrLedgerDisabled {
		t.Fatalf("esperado ErrLedgerDisabled, obtenido %v", err)
	}
}

func TestMonthlyArchiveName(t *testing.T) {
	l, _ := newTestLedger(t, true)
	ev := LedgerEvent{TS: "2026-03-15T10:00:00Z", Type: EventEdit, Action: "edit_file", Path: "a.go"}
	l.Append(ev)
	l.Append(LedgerEvent{TS: "2026-03-20T10:00:00Z", Type: EventEdit, Action: "edit_file", Path: "b.go"})
	l.Append(LedgerEvent{TS: "2026-04-01T10:00:00Z", Type: EventEdit, Action: "edit_file", Path: "c.go"})
	for i := 0; i < 40; i++ {
		l.AppendEdit("edit_file", "d.go", "x", "s", false)
	}
	l.CompactNow()
	var names []string
	entries, _ := os.ReadDir(l.Archives)
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if len(names) < 2 {
		t.Fatalf("esperados archivos de marzo y abril, hay %v", names)
	}
	mar := filepath.Join(l.Archives, "ledger.2026-03.jsonl")
	apr := filepath.Join(l.Archives, "ledger.2026-04.jsonl")
	if _, err := os.Stat(mar); err != nil {
		t.Fatalf("falta archivo de marzo: %v", err)
	}
	if _, err := os.Stat(apr); err != nil {
		t.Fatalf("falta archivo de abril: %v", err)
	}
}
