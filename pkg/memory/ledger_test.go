package memory

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func newTestLedger(t *testing.T, withGit bool) (*Ledger, string) {
	t.Helper()
	root := t.TempDir()
	if withGit {
		os.MkdirAll(filepath.Join(root, ".git"), 0700)
		os.MkdirAll(filepath.Join(root, ".git", "logs"), 0700)
	}
	home := t.TempDir()
	l, err := NewLedger(root, WithHome(home), WithWindow(20), WithMaxDepth(10))
	if err != nil {
		t.Fatalf("NewLedger: %v", err)
	}
	if !l.Enabled {
		t.Fatalf("ledger deshabilitado: %s", l.Reason)
	}
	if withGit && l.Root != root {
		t.Fatalf("raíz esperada %s, obtenida %q", root, l.Root)
	}
	return l, home
}

func TestLedgerAppendAndTail(t *testing.T) {
	l, _ := newTestLedger(t, true)
	for i := 0; i < 5; i++ {
		err := l.Append(LedgerEvent{
			TS:      time.Now().UTC().Add(time.Duration(i) * time.Second).Format(time.RFC3339),
			Type:    EventEdit,
			Action:  "edit_file",
			Path:    "src/auth.go",
			Detail:  "cambio de auth",
			Session: "s1",
		})
		if err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}
	evs := l.Tail(20, "", false)
	if len(evs) != 5 {
		t.Fatalf("esperados 5 eventos, obtenidos %d", len(evs))
	}
	if evs[0].ID == "" {
		t.Fatalf("falta ID autogenerado: %+v", evs[0])
	}
	for i := 1; i < len(evs); i++ {
		if evs[i-1].TS <= evs[i].TS {
			t.Fatalf("tail no invertido: %s antes que %s", evs[i-1].TS, evs[i].TS)
		}
	}
	if evs[0].Path != "src/auth.go" {
		t.Fatalf("path esperado src/auth.go, obtenido %q", evs[0].Path)
	}
}

func TestLedgerTailLimitAndFilter(t *testing.T) {
	l, _ := newTestLedger(t, true)
	for i := 0; i < 10; i++ {
		l.AppendEdit("edit_file", "a.go", "x", "s1", false)
	}
	l.AppendCommit("abc12345", "fix: algo", "main", "s1")
	evs := l.Tail(3, "", false)
	if len(evs) != 3 {
		t.Fatalf("esperados 3, obtenidos %d", len(evs))
	}
	commits := l.Tail(10, EventCommit, false)
	if len(commits) != 1 || commits[0].Hash != "abc12345" {
		t.Fatalf("filtro commit falló: %+v", commits)
	}
}

func TestLedgerTornLine(t *testing.T) {
	l, _ := newTestLedger(t, true)
	l.AppendEdit("edit_file", "a.go", "x", "s1", false)
	f, _ := os.OpenFile(l.Active, os.O_APPEND|os.O_WRONLY, 0600)
	f.WriteString(`{"v":1,"id":"zzz","ts":"2026-07-31T10:00:00Z","type":"edit","action":"edit_fi`)
	f.Close()
	evs := l.Tail(20, "", false)
	if len(evs) != 1 {
		t.Fatalf("la línea rota debe saltarse, quedan %d eventos", len(evs))
	}
}

func TestLedgerSanitizeSecrets(t *testing.T) {
	l, _ := newTestLedger(t, true)
	l.Append(LedgerEvent{Type: EventError, Action: "run_command", Detail: "clave sk-ant-api03-abcdefghijklmnopqrstuvwxyz123456 en error"})
	l.Append(LedgerEvent{Type: EventError, Action: "run_command", Detail: "token Bearer abcdefghijklmnopqrstuvwxyz1234567890!"})
	l.Append(LedgerEvent{Type: EventEdit, Action: "create_file", Detail: "config con ghp_abcdefghijklmnopqrstuvwxyz123456"})
	evs := l.Tail(20, "", false)
	for _, ev := range evs {
		if secretRE.MatchString(ev.Detail) {
			t.Fatalf("secreto sin censurar en detail: %q", ev.Detail)
		}
		if ev.Detail == "" {
			t.Fatalf("detail vacío tras sanitizar")
		}
	}
}

func TestLedgerDetailTruncated(t *testing.T) {
	l, _ := newTestLedger(t, true)
	long := strings.Repeat("é", defaultMaxDetail+500)
	l.Append(LedgerEvent{Type: EventMilestone, Action: "mark", Detail: long})
	evs := l.Tail(20, "", false)
	if len(evs) != 1 {
		t.Fatalf("esperado 1 evento, obtenidos %d", len(evs))
	}
	if len([]rune(evs[0].Detail)) > defaultMaxDetail+2 {
		t.Fatalf("detail no truncado: %d runes", len([]rune(evs[0].Detail)))
	}
}

func TestLedgerConcurrentAppend(t *testing.T) {
	l, _ := newTestLedger(t, true)
	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				if err := l.AppendEdit("edit_file", "f.go", "x", "s", false); err != nil {
					t.Errorf("append: %v", err)
					return
				}
			}
		}(g)
	}
	wg.Wait()
	evs := l.Tail(1000, "", false)
	if len(evs) != 200 {
		t.Fatalf("esperados 200 eventos (4×50), obtenidos %d", len(evs))
	}
}

func TestLedgerNoGitFallback(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()
	l, err := NewLedger(dir, WithHome(home), WithWindow(20))
	if err != nil {
		t.Fatalf("NewLedger: %v", err)
	}
	if !l.Enabled {
		t.Fatalf("ledger deshabilitado sin .git: %s", l.Reason)
	}
	if l.Root != "" {
		t.Fatalf("sin .git no debe haber raíz: %q", l.Root)
	}
	if !filepathHasPrefix(l.Dir, home) {
		t.Fatalf("debe usar el fallback del home: %q", l.Dir)
	}
	l.AppendEdit("edit_file", "a.go", "x", "s", false)
	if len(l.Tail(10, "", false)) != 1 {
		t.Fatalf("el ledger de respaldo no funciona")
	}
}

func filepathHasPrefix(p, prefix string) bool {
	rel, err := filepath.Rel(prefix, p)
	if err != nil {
		return false
	}
	return rel != ".." && !filepathIsAbs(rel)
}

func TestLedgerEnvOff(t *testing.T) {
	t.Setenv("GOULM_LEDGER", "off")
	l, err := NewLedger(t.TempDir(), WithHome(t.TempDir()))
	if err != nil {
		t.Fatalf("NewLedger: %v", err)
	}
	if l.Enabled {
		t.Fatal("GOULM_LEDGER=off debe deshabilitar el ledger")
	}
	if err := l.AppendEdit("edit_file", "a.go", "x", "s", false); err != ErrLedgerDisabled {
		t.Fatalf("esperado ErrLedgerDisabled, obtenido %v", err)
	}
}

func TestLedgerStatsAndExport(t *testing.T) {
	l, _ := newTestLedger(t, true)
	l.AppendCommit("aaaa1111", "fix", "main", "s1")
	l.AppendMemory("remember", "use-zod", "decision", "s1")
	l.AppendMilestone("release v0.3", "s1")
	st := l.Stats()
	if st.Total != 3 {
		t.Fatalf("esperados 3 en stats, obtenido %d", st.Total)
	}
	if st.ByType[EventCommit] != 1 {
		t.Fatalf("stats por tipo incorrectas: %+v", st.ByType)
	}
	exp, err := l.Export("", "")
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	lines := 0
	for _, ln := range splitLines(exp) {
		if ln != "" {
			lines++
		}
	}
	if lines != 3 {
		t.Fatalf("export debe tener 3 líneas, tiene %d", lines)
	}
	future := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	past := time.Now().AddDate(0, 0, -2).Format("2006-01-02")
	none, _ := l.Export(past, past)
	if stringsTrim(none) != "" {
		t.Fatalf("export fuera de rango debe estar vacío, %q", none)
	}
	all, _ := l.Export("", future)
	if stringsTrim(all) == "" {
		t.Fatal("export con to futuro debe incluir eventos")
	}
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func stringsTrim(s string) string {
	start := 0
	for start < len(s) && (s[start] == '\n' || s[start] == '\r' || s[start] == ' ') {
		start++
	}
	end := len(s)
	for end > start && (s[end-1] == '\n' || s[end-1] == '\r' || s[end-1] == ' ') {
		end--
	}
	return s[start:end]
}

func TestReflogNew(t *testing.T) {
	root := t.TempDir()
	git := filepath.Join(root, ".git")
	os.MkdirAll(filepath.Join(git, "logs"), 0700)
	log := `0000000000000000000000000000000000000000 aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa User <u@x> 1780000000 +0000	commit (initial): primer commit
aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb User <u@x> 1780000100 +0000	commit: fix: pool de redis
`
	os.WriteFile(filepath.Join(git, "logs", "HEAD"), []byte(log), 0600)

	all := ReflogNew(root, "")
	if len(all) != 2 {
		t.Fatalf("esperadas 2 entradas, obtenidas %d", len(all))
	}
	if all[1].Hash != "bbbbbbbb" {
		t.Fatalf("hash esperado bbbbbbbb, obtenido %s", all[1].Hash)
	}
	if all[1].Subject != "commit: fix: pool de redis" {
		t.Fatalf("subject inesperado: %q", all[1].Subject)
	}
	after := ReflogNew(root, "aaaaaaaa")
	if len(after) != 1 || after[0].Hash != "bbbbbbbb" {
		t.Fatalf("ReflogNew(fromHash) incorrecto: %+v", after)
	}
}

func TestLedgerAppendToolV2(t *testing.T) {
	l, _ := newTestLedger(t, true)
	err := l.AppendTool("read_file", "src/auth.go", StatusOK, "Bajo", 42, "s1", false)
	if err != nil {
		t.Fatalf("AppendTool: %v", err)
	}
	err = l.AppendTool("run_command", "", StatusError, "Critico", 1500, "s1", false)
	if err != nil {
		t.Fatalf("AppendTool error: %v", err)
	}
	evs := l.Tail(10, EventTool, false)
	if len(evs) != 2 {
		t.Fatalf("esperados 2 eventos tool, obtenidos %d", len(evs))
	}
	// Tail es más-reciente-primero: [0]=run_command, [1]=read_file
	if evs[0].V != EventVersion || evs[1].V != EventVersion {
		t.Fatalf("V esperado %d, obtenidos %d/%d", EventVersion, evs[0].V, evs[1].V)
	}
	if evs[1].Status != StatusOK || evs[1].DurationMs != 42 || evs[1].Risk != "Bajo" {
		t.Fatalf("campos tool ok incorrectos: %+v", evs[1])
	}
	if evs[0].Status != StatusError || evs[0].DurationMs != 1500 {
		t.Fatalf("campos tool error incorrectos: %+v", evs[0])
	}
}

func TestLedgerAppendApproval(t *testing.T) {
	l, _ := newTestLedger(t, true)
	if err := l.AppendApproval("edit_file", ApprovedYes, "s1", false); err != nil {
		t.Fatalf("AppendApproval: %v", err)
	}
	if err := l.AppendApproval("run_command", ApprovedNo, "s1", false); err != nil {
		t.Fatalf("AppendApproval no: %v", err)
	}
	evs := l.Tail(10, EventApproval, false)
	if len(evs) != 2 {
		t.Fatalf("esperados 2 approvals, obtenidos %d", len(evs))
	}
	// Tail es más-reciente-primero: [0]=run_command(no), [1]=edit_file(yes)
	if evs[0].Approved != ApprovedNo || evs[1].Approved != ApprovedYes {
		t.Fatalf("approved incorrecto: %+v %+v", evs[0], evs[1])
	}
	if evs[0].Type != EventApproval {
		t.Fatalf("tipo esperado approval, obtenido %q", evs[0].Type)
	}
}

func TestLedgerParseEventV1AndV2(t *testing.T) {
	l, _ := newTestLedger(t, true)
	// evento v1 legacy (V fijado explícitamente)
	l.Append(LedgerEvent{V: 1, Type: EventEdit, Action: "edit_file", Path: "a.go", Detail: "legacy", Session: "s1"})
	// evento v2 (default)
	l.AppendTool("read_file", "b.go", StatusOK, "Bajo", 10, "s1", false)
	evs := l.Tail(20, "", false)
	if len(evs) != 2 {
		t.Fatalf("esperados 2 eventos, obtenidos %d", len(evs))
	}
	// Tail es más-reciente-primero: [0]=read_file(v2), [1]=edit(v1)
	if evs[1].V != 1 {
		t.Fatalf("evento v1 esperado, obtenido V=%d", evs[1].V)
	}
	if evs[0].V != EventVersion {
		t.Fatalf("evento v2 esperado, obtenido V=%d", evs[0].V)
	}
	// parseEvent acepta ambos
	if _, ok := parseEvent(`{"v":1,"ts":"2026-01-01T00:00:00Z","type":"edit"}`); !ok {
		t.Fatal("parseEvent debe aceptar v1")
	}
	if _, ok := parseEvent(`{"v":2,"ts":"2026-01-01T00:00:00Z","type":"tool"}`); !ok {
		t.Fatal("parseEvent debe aceptar v2")
	}
	if _, ok := parseEvent(`{"v":3,"ts":"2026-01-01T00:00:00Z","type":"tool"}`); ok {
		t.Fatal("parseEvent debe rechazar v3")
	}
}

func TestLedgerAppendDefaults(t *testing.T) {
	l, _ := newTestLedger(t, true)
	l.Append(LedgerEvent{Type: EventTool, Action: "read_file"})
	l.Append(LedgerEvent{Type: EventError, Action: "run_command", Detail: "falló"})
	evs := l.Tail(10, "", false)
	// Tail es más-reciente-primero: [0]=error, [1]=tool
	if evs[0].V != EventVersion || evs[1].V != EventVersion {
		t.Fatalf("Append debe fijar V=%d, obtenidos %d/%d", EventVersion, evs[0].V, evs[1].V)
	}
	if evs[0].Status != StatusError {
		t.Fatalf("status default error para tipo error, obtenido %q", evs[0].Status)
	}
	if evs[1].Status != StatusOK {
		t.Fatalf("status default ok esperado, obtenido %q", evs[1].Status)
	}
	if evs[1].Approved != ApprovedNA {
		t.Fatalf("approved default na esperado, obtenido %q", evs[1].Approved)
	}
}

func TestFormatEventStatusAndDuration(t *testing.T) {
	ok := FormatEvent(LedgerEvent{TS: "2026-08-15T10:30:00Z", Type: EventTool, Action: "read_file", Path: "a.go", Status: StatusOK, DurationMs: 250})
	if !strings.Contains(ok, "✓") {
		t.Fatalf("formato ok debe tener marca ✓: %q", ok)
	}
	if !strings.Contains(ok, "250ms") {
		t.Fatalf("formato ok debe incluir duración: %q", ok)
	}
	if !strings.Contains(ok, "10:30") {
		t.Fatalf("formato corto debe incluir hora: %q", ok)
	}
	if strings.Contains(ok, "Z") {
		t.Fatalf("formato corto no debe incluir segundos/zona: %q", ok)
	}
	full := FormatEventFull(LedgerEvent{TS: "2026-08-15T10:30:00Z", Type: EventTool, Action: "read_file", Status: StatusError})
	if !strings.Contains(full, "2026-08-15") {
		t.Fatalf("formato full debe incluir fecha: %q", full)
	}
	if !strings.Contains(full, "✗") {
		t.Fatalf("formato error debe tener marca ✗: %q", full)
	}
}
