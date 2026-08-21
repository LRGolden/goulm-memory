package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSessionHeartbeatLifecycle(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sessions")
	tracker, err := NewSessionTracker(dir, "test-agent")
	if err != nil {
		t.Fatal(err)
	}
	if tracker.SelfID() == "" {
		t.Fatal("selfID vacío")
	}
	if err := tracker.Touch("src/auth.ts"); err != nil {
		t.Fatal(err)
	}
	if err := tracker.Touch("src/types.ts"); err != nil {
		t.Fatal(err)
	}

	sessions, err := tracker.ActiveSessions()
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 {
		t.Fatalf("sesiones activas = %d, esperaba 1", len(sessions))
	}
	if !sessions[0].IsSelf {
		t.Error("debería marcar la propia sesión")
	}
	if len(sessions[0].Files) != 2 {
		t.Errorf("files = %v", sessions[0].Files)
	}
	if tracker.branch == "" && hasGit(t.TempDir()) {
		// rama puede estar vacía si no hay repo; solo verificamos que no falle.
	}

	files := tracker.SessionFiles()
	if !files["src/auth.ts"] {
		t.Error("SessionFiles debería contener src/auth.ts")
	}

	// Cierre de sesión.
	if err := tracker.End(); err != nil {
		t.Fatal(err)
	}
	sessions, _ = tracker.ActiveSessions()
	if len(sessions) != 0 {
		t.Error("sesión ended no debería listarse como activa")
	}
}

func TestSessionConflicts(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sessions")
	tracker, _ := NewSessionTracker(dir, "agent-a")
	tracker.Touch("src/shared.ts")

	// Sesión ajena: escribir heartbeat directamente.
	other := Heartbeat{
		ID:        "sesion-ajena",
		Agent:     "agent-b",
		PID:       os.Getpid(), // vivo
		Branch:    "main",
		StartedAt: time.Now().UTC().Format(time.RFC3339),
		LastSeen:  time.Now().UTC().Format(time.RFC3339),
		Files:     map[string]string{"src/shared.ts": time.Now().UTC().Format(time.RFC3339)},
	}
	data, _ := jsonMarshalIndent(other)
	if err := atomicWrite(filepath.Join(dir, "sesion-ajena.json"), data, 0600); err != nil {
		t.Fatal(err)
	}

	conflicts, err := tracker.Conflicts()
	if err != nil {
		t.Fatal(err)
	}
	if len(conflicts) != 1 || conflicts[0].File != "src/shared.ts" {
		t.Errorf("conflicts = %+v", conflicts)
	}
	if len(conflicts[0].Sessions) != 2 {
		t.Errorf("sessions en conflicto = %v", conflicts[0].Sessions)
	}
}

func TestSessionPruneDeadPID(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sessions")
	tracker, _ := NewSessionTracker(dir, "agent-a")

	// Heartbeat de un PID muerto y con last_seen viejo.
	stale := Heartbeat{
		ID:        "muerta",
		Agent:     "agent-x",
		PID:       999999, // improbable que exista
		Branch:    "main",
		StartedAt: time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339),
		LastSeen:  time.Now().Add(-30 * time.Minute).UTC().Format(time.RFC3339),
		Files:     map[string]string{},
	}
	data, _ := jsonMarshalIndent(stale)
	if err := atomicWrite(filepath.Join(dir, "muerta.json"), data, 0600); err != nil {
		t.Fatal(err)
	}

	removed, err := tracker.Prune()
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Errorf("pruned = %d, esperaba 1", removed)
	}
}

func TestRenderSessions(t *testing.T) {
	out := RenderSessions(
		[]ActiveSession{{ID: "a", Agent: "goulm", Branch: "main", IsSelf: true, LastSeen: time.Now()}},
		[]FileConflict{{File: "x.go", Sessions: []string{"a", "b"}}},
		false,
	)
	if !strings.Contains(out, "goulm") || !strings.Contains(out, "x.go") {
		t.Errorf("render = %q", out)
	}
	only := RenderSessions(nil, []FileConflict{{File: "x.go", Sessions: []string{"a", "b"}}}, true)
	if !strings.Contains(only, "Conflictos") {
		t.Errorf("conflictsOnly = %q", only)
	}
}

func hasGit(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}
