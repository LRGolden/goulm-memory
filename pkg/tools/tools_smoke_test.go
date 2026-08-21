package tools

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/LRGolden/goulm-memory/pkg/memory"
)

// TestRegistryMemoryTools verifica que las 11 tools de memoria se registran y
// se ejecutan contra un store temporal (cableado completo store → tracker → tools).
func TestRegistryMemoryTools(t *testing.T) {
	dir := t.TempDir()
	store, err := memory.NewStore(memory.Config{
		Dir:        filepath.Join(dir, "mem"),
		Project:    "smoke-test",
		MaxEntries: 100,
	})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	tracker, err := store.Sessions("goulm")
	if err != nil {
		t.Fatalf("Sessions: %v", err)
	}
	tracker.SetRoot("")
	tracker.Heartbeat("", false)

	reg := NewRegistry()
	RegisterMemoryTools(reg, store, tracker)
	if got := reg.Count(); got != 11 {
		t.Fatalf("Count = %d, want 11", got)
	}

	execTool(t, reg, "memory_remember", `{"category":"decision","key":"use-stdlib","content":"Usar stdlib puro para el parser","tags":"parser;stdlib"}`)

	out := execTool(t, reg, "memory_recall", `{"query":"parser stdlib","limit":3}`)
	if out == "" {
		t.Fatal("recall devolvió salida vacía")
	}

	out = execTool(t, reg, "memory_stats", `{"health":true}`)
	if out == "" {
		t.Fatal("stats devolvió salida vacía")
	}

	out = execTool(t, reg, "memory_suggest", `{"context":"cómo parsear el config","limit":3}`)
	if out == "" {
		t.Fatal("suggest devolvió salida vacía")
	}

	execTool(t, reg, "memory_pin", `{"key":"use-stdlib","priority":3}`)

	out = execTool(t, reg, "context_brief", `{"limit":3}`)
	if out == "" {
		t.Fatal("context_brief devolvió salida vacía")
	}
}

// TestRegistryLedgerTools verifica el registro y ejecución de ledger_tail y
// ledger_log contra un ledger aislado en el directorio temporal.
func TestRegistryLedgerTools(t *testing.T) {
	dir := t.TempDir()
	ledger, err := memory.NewLedger(dir, memory.WithHome(dir))
	if err != nil {
		t.Fatalf("NewLedger: %v", err)
	}
	if !ledger.Enabled {
		t.Skipf("ledger deshabilitado: %s", ledger.Reason)
	}
	hook := NewLedgerHook(ledger)
	defer hook.Close()
	hook.StartSession("smoke")

	store, err := memory.NewStore(memory.Config{
		Dir:        filepath.Join(dir, "mem"),
		Project:    "smoke-ledger",
		MaxEntries: 100,
	})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	tracker, err := store.Sessions("goulm")
	if err != nil {
		t.Fatalf("Sessions: %v", err)
	}
	tracker.SetRoot("")
	tracker.Heartbeat("", false)

	reg := NewRegistry()
	RegisterMemoryTools(reg, store, tracker)
	RegisterLedgerTools(reg, hook)
	if got := reg.Count(); got != 13 {
		t.Fatalf("Count = %d, want 13", got)
	}

	execTool(t, reg, "ledger_log", `{"message":"smoke test de ledger"}`)
	execTool(t, reg, "memory_remember", `{"category":"bug","key":"smoke-bug","content":"bug ficticio para el ledger"}`)

	out := execTool(t, reg, "ledger_tail", `{"n":5}`)
	if out == "" {
		t.Fatal("ledger_tail devolvió salida vacía")
	}
}

func execTool(t *testing.T, reg *Registry, name, args string) string {
	t.Helper()
	tool, ok := reg.Get(name)
	if !ok {
		t.Fatalf("tool %q no registrada", name)
	}
	out, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return out
}
