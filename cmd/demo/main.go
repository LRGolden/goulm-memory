// Command demo es un CLI mínimo para probar el ecosistema de memoria de
// goulm-memory: cablea store + session tracker + ledger + las 13 tools y las
// ejecuta vía Registry.
//
// Uso:
//
//	go run ./cmd/demo                    # alias de "demo"
//	go run ./cmd/demo demo
//	go run ./cmd/demo remember -key auth-jwt -category decision "Usar JWT para auth"
//	go run ./cmd/demo recall -q "autenticación"
//	go run ./cmd/demo stats --health
//	go run ./cmd/demo ledger-tail
//
// La memoria se guarda en ~/.goulm-memory/<Proyecto> por defecto; usa -dir
// para cambiarlo. Exit codes: 0 éxito, 1 error, 2 uso incorrecto.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LRGolden/goulm-memory/pkg/memory"
	"github.com/LRGolden/goulm-memory/pkg/tools"
)

func main() {
	dir := flag.String("dir", "", "directorio de memoria (default ~/.goulm-memory/<proyecto>)")
	flag.Usage = usage
	flag.Parse()

	cwd, err := os.Getwd()
	if err != nil {
		fail(1, "no se pudo obtener el directorio actual: %v", err)
	}

	memDir := *dir
	if memDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			fail(1, "sin directorio home: %v", err)
		}
		memDir = filepath.Join(home, ".goulm-memory", memory.ProjectID(cwd))
	}

	store, err := memory.NewStore(memory.Config{
		Dir:        memDir,
		Format:     memory.FormatJSON,
		Project:    memory.ProjectID(cwd),
		MaxEntries: 100,
	})
	if err != nil {
		fail(1, "no se pudo abrir la memoria en %s: %v", memDir, err)
	}
	store.SetVocab(memory.ExtractProjectDeps(cwd))

	tracker, err := store.Sessions("goulm")
	if err == nil {
		tracker.SetRoot("")
		tracker.Heartbeat("", false)
	} else {
		tracker = nil
	}

	// El ledger es best-effort: si falla o está deshabilitado, las tools de
	// ledger simplemente no se registran. Se aísla dentro del directorio de
	// memoria del demo para no escribir en ~/.goulm/ledger.
	ledger, lerr := memory.NewLedger(cwd, memory.WithHome(filepath.Join(memDir, "ledger")))
	if lerr != nil {
		ledger = nil
	}
	hook := tools.NewLedgerHook(ledger)
	defer hook.Close()
	hook.StartSession("demo")

	reg := tools.NewRegistry()
	tools.RegisterMemoryTools(reg, store, tracker)
	tools.RegisterLedgerTools(reg, hook)

	args := flag.Args()
	cmd := "demo"
	if len(args) > 0 {
		cmd = args[0]
	}
	rest := args[1:]

	switch cmd {
	case "demo":
		os.Exit(runDemo(reg))
	case "remember":
		os.Exit(cmdRemember(reg, rest))
	case "recall":
		os.Exit(cmdRecall(reg, rest))
	case "stats":
		os.Exit(cmdStats(reg, rest))
	case "suggest":
		os.Exit(cmdSuggest(reg, rest))
	case "brief":
		os.Exit(cmdBrief(reg, rest))
	case "pin":
		os.Exit(cmdPin(reg, rest))
	case "forget":
		os.Exit(cmdForget(reg, rest))
	case "resolve":
		os.Exit(cmdResolve(reg, rest))
	case "backup":
		os.Exit(cmdSimple(reg, "memory_backup"))
	case "archive":
		os.Exit(cmdSimple(reg, "memory_archive"))
	case "consolidate":
		os.Exit(cmdSimple(reg, "memory_consolidate"))
	case "ledger-tail":
		os.Exit(cmdLedgerTail(reg, rest))
	case "ledger-log":
		os.Exit(cmdLedgerLog(reg, rest))
	case "tools":
		listTools(reg)
	case "help", "-h", "--help":
		usage()
	default:
		fail(2, "comando desconocido: %s", cmd)
	}
}

// ─── Comandos ───────────────────────────────────────────────

func runDemo(reg *tools.Registry) int {
	fmt.Println("== goulm-memory demo ==")
	seq := [][2]string{
		{"memory_remember", `{"category":"decision","key":"use-stdlib","content":"Usar stdlib puro para el parser de config","tags":"parser;stdlib"}`},
		{"memory_remember", `{"category":"pattern","key":"http-timeout","content":"Siempre definir timeout explícito en clientes HTTP","tags":"http;timeout"}`},
		{"memory_recall", `{"query":"parser config","limit":3}`},
		{"memory_stats", `{"health":true}`},
		{"ledger_log", `{"message":"demo de goulm-memory"}`},
		{"ledger_tail", `{"n":4}`},
	}
	for _, s := range seq {
		fmt.Printf("\n--- %s ---\n", s[0])
		runTool(reg, s[0], s[1])
	}
	return 0
}

func cmdRemember(reg *tools.Registry, args []string) int {
	fs := newFlagSet("remember")
	key := fs.String("key", "", "clave kebab-case (obligatoria)")
	category := fs.String("category", "knowledge", "decision | pattern | bug | knowledge")
	content := fs.String("content", "", "texto de la cápsula (obligatorio)")
	tags := fs.String("tags", "", "tags separados por ';'")
	ttl := fs.String("ttl", "", "7d | 30d | YYYY-MM-DD")
	file := fs.String("file", "", "archivo relacionado")
	links := fs.String("links", "", "claves relacionadas ';'")
	priority := fs.Int("priority", 0, "fijar 1-5 (0 = normal)")
	fs.Parse(args)
	if *key == "" || *content == "" {
		return errUsage("remember", "se requieren -key y -content")
	}
	return runTool(reg, "memory_remember", obj{
		"key": *key, "category": *category, "content": *content,
		"tags": *tags, "ttl": *ttl, "file": *file, "links": *links, "priority": *priority,
	})
}

func cmdRecall(reg *tools.Registry, args []string) int {
	fs := newFlagSet("recall")
	query := fs.String("q", "", "texto a buscar")
	intent := fs.String("intent", "", "qué vas a hacer (recall unificado)")
	category := fs.String("category", "", "filtro de categoría")
	tags := fs.String("tags", "", "filtro AND ';'")
	limit := fs.Int("limit", 6, "máx resultados")
	budget := fs.String("budget", "normal", "tiny | normal | deep")
	fs.Parse(args)
	if *query == "" && *intent == "" {
		return errUsage("recall", "se requiere -q o -intent")
	}
	return runTool(reg, "memory_recall", obj{
		"query": *query, "intent": *intent, "category": *category,
		"tags": *tags, "limit": *limit, "budget": *budget,
	})
}

func cmdStats(reg *tools.Registry, args []string) int {
	fs := newFlagSet("stats")
	health := fs.Bool("health", false, "incluir score de salud")
	recent := fs.String("recent", "", "24h | 7d | YYYY-MM-DD")
	fs.Parse(args)
	return runTool(reg, "memory_stats", obj{"health": *health, "recent": *recent})
}

func cmdSuggest(reg *tools.Registry, args []string) int {
	fs := newFlagSet("suggest")
	context := fs.String("context", "", "descripción del contexto (obligatorio)")
	limit := fs.Int("limit", 5, "máx resultados")
	fs.Parse(args)
	if *context == "" {
		return errUsage("suggest", "se requiere -context")
	}
	return runTool(reg, "memory_suggest", obj{"context": *context, "limit": *limit})
}

func cmdBrief(reg *tools.Registry, args []string) int {
	fs := newFlagSet("brief")
	limit := fs.Int("limit", 5, "máx cápsulas del top")
	fs.Parse(args)
	return runTool(reg, "context_brief", obj{"limit": *limit})
}

func cmdPin(reg *tools.Registry, args []string) int {
	fs := newFlagSet("pin")
	key := fs.String("key", "", "clave (obligatoria)")
	priority := fs.Int("priority", 3, "1-5 (0 = desfijar)")
	fs.Parse(args)
	if *key == "" {
		return errUsage("pin", "se requiere -key")
	}
	return runTool(reg, "memory_pin", obj{"key": *key, "priority": *priority})
}

func cmdForget(reg *tools.Registry, args []string) int {
	fs := newFlagSet("forget")
	key := fs.String("key", "", "clave (obligatoria)")
	hard := fs.Bool("hard", false, "borrado físico")
	fs.Parse(args)
	if *key == "" {
		return errUsage("forget", "se requiere -key")
	}
	return runTool(reg, "memory_forget", obj{"key": *key, "hard": *hard})
}

func cmdResolve(reg *tools.Registry, args []string) int {
	fs := newFlagSet("resolve")
	key := fs.String("key", "", "clave (obligatoria)")
	fs.Parse(args)
	if *key == "" {
		return errUsage("resolve", "se requiere -key")
	}
	return runTool(reg, "memory_resolve", obj{"key": *key})
}

func cmdSimple(reg *tools.Registry, name string) int {
	return runTool(reg, name, "{}")
}

func cmdLedgerTail(reg *tools.Registry, args []string) int {
	fs := newFlagSet("ledger-tail")
	n := fs.Int("n", 20, "número de eventos")
	typ := fs.String("type", "", "edit | commit | memory | milestone | error | session")
	fs.Parse(args)
	return runTool(reg, "ledger_tail", obj{"n": *n, "type": *typ})
}

func cmdLedgerLog(reg *tools.Registry, args []string) int {
	fs := newFlagSet("ledger-log")
	message := fs.String("message", "", "descripción del hito (obligatorio)")
	fs.Parse(args)
	if *message == "" && len(fs.Args()) > 0 {
		*message = strings.Join(fs.Args(), " ")
	}
	if *message == "" {
		return errUsage("ledger-log", "se requiere -message")
	}
	return runTool(reg, "ledger_log", obj{"message": *message})
}

func listTools(reg *tools.Registry) {
	fmt.Println("Herramientas registradas:", reg.Count())
	for _, name := range reg.Names() {
		t, _ := reg.Get(name)
		fmt.Printf("  %-18s [%s] %s\n", name, t.RiskLevel, t.Description)
	}
}

// ─── Helpers ────────────────────────────────────────────────

type obj map[string]interface{}

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	fs.Usage = func() { fmt.Fprintf(os.Stderr, "Uso: goulm-memory %s [flags]\n", name); fs.PrintDefaults() }
	return fs
}

func runTool(reg *tools.Registry, name string, args interface{}) int {
	t, ok := reg.Get(name)
	if !ok {
		fail(1, "tool %q no registrada (¿ledger deshabilitado?)", name)
	}
	var raw string
	switch v := args.(type) {
	case string:
		raw = v
	default:
		b, err := json.Marshal(v)
		if err != nil {
			fail(2, "argumentos inválidos: %v", err)
		}
		raw = string(b)
	}
	out, err := t.Execute(context.Background(), raw)
	if err != nil {
		fail(1, "%s: %v", name, err)
	}
	fmt.Println(out)
	return 0
}

func errUsage(cmd, msg string) int {
	fmt.Fprintln(os.Stderr, "uso:", msg)
	return 2
}

func fail(code int, format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(code)
}

func usage() {
	fmt.Fprintf(os.Stderr, `goulm-memory — ecosistema de memoria persistente (demo)

Uso: goulm-memory [flags] <comando> [args]

Flags:
  -dir <ruta>   directorio de memoria (default ~/.goulm-memory/<proyecto>)

Comandos:
  demo            secuencia guionizada (default)
  remember        -key K -category C -content T [-tags -ttl -file -links -priority]
  recall          -q Q | -intent I [-category -tags -limit -budget]
  stats           [-health] [-recent 24h|7d|YYYY-MM-DD]
  suggest         -context C [-limit]
  brief           [-limit]
  pin             -key K [-priority N]
  forget          -key K [-hard]
  resolve         -key K
  backup          crear copia con timestamp
  archive         archivar cápsulas antiguas
  consolidate     fusionar duplicados
  ledger-tail     [-n N] [-type T]
  ledger-log      -message M
  tools           listar las tools registradas
  help            esta ayuda
`)
}
