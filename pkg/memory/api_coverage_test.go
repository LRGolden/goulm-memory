package memory

import (
	"strings"
	"testing"
)

func TestSmartRecallUnified(t *testing.T) {
	s := newTestStore(t, FormatJSON)
	if _, err := s.Remember(RememberOptions{Category: CategoryKnowledge, Key: "auth-flow", Content: "El flujo de autenticación valida tokens JWT"}); err != nil {
		t.Fatalf("Remember: %v", err)
	}

	rs, err := s.SmartRecall("autenticación", 3)
	if err != nil {
		t.Fatalf("SmartRecall: %v", err)
	}
	if len(rs) == 0 {
		t.Fatal("SmartRecall debería devolver resultados")
	}
	if rs[0].Capsule.Key != "auth-flow" {
		t.Errorf("top result = %s, esperaba auth-flow", rs[0].Capsule.Key)
	}

	rs2, err := s.SmartRecall("autenticación", 3, map[string]bool{"src/auth.go": true})
	if err != nil {
		t.Fatalf("SmartRecall con sessionFiles: %v", err)
	}
	if len(rs2) == 0 {
		t.Fatal("SmartRecall con sessionFiles debería devolver resultados")
	}
}

func TestSuggestDefaultsAndLimit(t *testing.T) {
	s := newTestStore(t, FormatJSON)
	for i := 0; i < 3; i++ {
		if _, err := s.Remember(RememberOptions{
			Category: CategoryKnowledge,
			Key:      "cap-" + string(rune('a'+i)),
			Content:  "cápsula relacionada con el contexto de búsqueda",
		}); err != nil {
			t.Fatalf("Remember %d: %v", i, err)
		}
	}

	// limit <= 0 aplica el default 5.
	rs, err := s.Suggest("contexto", 0)
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}
	if len(rs) != 3 {
		t.Errorf("sugerencias = %d, esperaba 3", len(rs))
	}

	// limit explícito mayor al número real de cápsulas.
	rs2, err := s.Suggest("contexto", 10, map[string]bool{"x.go": true})
	if err != nil {
		t.Fatalf("Suggest con sessionFiles: %v", err)
	}
	if len(rs2) != 3 {
		t.Errorf("sugerencias con sessionFiles = %d, esperaba 3", len(rs2))
	}
}

func TestVocabRoundTripAndNil(t *testing.T) {
	s := newTestStore(t, FormatJSON)

	if err := s.SetVocab(map[string][]string{"go": {"token", "handler"}}); err != nil {
		t.Fatalf("SetVocab: %v", err)
	}
	v := s.Vocab()
	if len(v["go"]) != 2 || v["go"][0] != "token" {
		t.Errorf("vocab = %v", v)
	}

	// La copia debe aislar al llamador.
	v["go"][0] = "mutado"
	if s.Vocab()["go"][0] != "token" {
		t.Error("Vocab devolvió una referencia interna (falta copia)")
	}

	// nil vacía el vocabulario y persiste.
	if err := s.SetVocab(nil); err != nil {
		t.Fatalf("SetVocab(nil): %v", err)
	}
	if len(s.Vocab()) != 0 {
		t.Errorf("vocab tras nil = %v, esperaba vacío", s.Vocab())
	}
}

func TestClearBacksUpAndEmpties(t *testing.T) {
	s := newTestStore(t, FormatJSON)
	for i := 0; i < 2; i++ {
		if _, err := s.Remember(RememberOptions{
			Category: CategoryPattern,
			Key:      "p" + string(rune('0'+i)),
			Content:  "patrón de ejemplo",
		}); err != nil {
			t.Fatalf("Remember: %v", err)
		}
	}

	n, err := s.Clear()
	if err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if n != 2 {
		t.Errorf("Clear devolvió %d, esperaba 2", n)
	}
	if len(s.ListActive(0)) != 0 {
		t.Error("el almacén debería quedar vacío tras Clear")
	}

	// Clear sobre almacén vacío no debe fallar.
	n, err = s.Clear()
	if err != nil || n != 0 {
		t.Errorf("Clear vacío = (%d, %v), esperaba (0, nil)", n, err)
	}
}

func TestSessionsGetterWiresTracker(t *testing.T) {
	s := newTestStore(t, FormatJSON)

	tracker, err := s.Sessions("api-test-agent")
	if err != nil {
		t.Fatalf("Sessions: %v", err)
	}
	if tracker == nil {
		t.Fatal("Sessions devolvió nil")
	}
	if err := tracker.Touch("src/handler.go"); err != nil {
		t.Fatalf("Touch: %v", err)
	}
	files := tracker.SessionFiles()
	if !files["src/handler.go"] {
		t.Error("SessionFiles debería contener src/handler.go")
	}
	if err := tracker.End(); err != nil {
		t.Fatalf("End: %v", err)
	}
}

func TestLedgerSessionAndErrorAppends(t *testing.T) {
	l, _ := newTestLedger(t, true)

	if err := l.AppendSessionStart("s-session-a"); err != nil {
		t.Fatalf("AppendSessionStart: %v", err)
	}
	if err := l.AppendError("run_command", "salida no esperada", "s-session-a", true); err != nil {
		t.Fatalf("AppendError: %v", err)
	}
	if err := l.AppendSessionEnd("s-session-a"); err != nil {
		t.Fatalf("AppendSessionEnd: %v", err)
	}

	evs := l.Tail(10, "", false)
	if len(evs) != 3 {
		t.Fatalf("eventos = %d, esperaba 3", len(evs))
	}
	if evs[2].Type != EventSession || evs[2].Action != "start" {
		t.Errorf("primer evento = %+v", evs[2])
	}
	if evs[0].Type != EventSession || evs[0].Action != "end" {
		t.Errorf("último evento = %+v", evs[0])
	}
}

func TestFormatEvent(t *testing.T) {
	ev := LedgerEvent{
		TS:     "2026-11-05T15:41:00Z",
		Type:   EventEdit,
		Action: "edit_file",
		Path:   "src/auth.go",
		Detail: "refactor de login",
	}
	out := FormatEvent(ev)
	if !strings.Contains(out, "[edit]") {
		t.Errorf("FormatEvent debería incluir el tipo: %q", out)
	}
	if !strings.Contains(out, "src/auth.go") {
		t.Errorf("FormatEvent debería incluir el path: %q", out)
	}
	if !strings.Contains(out, "refactor de login") {
		t.Errorf("FormatEvent debería incluir el detalle: %q", out)
	}
}
