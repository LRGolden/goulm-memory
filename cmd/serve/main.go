// Command serve levanta un HTTP server que expone las operaciones esenciales
// del store de memoria via endpoints JSON.
//
// Uso:
//
//	go run ./cmd/serve                          # default :8080
//	go run ./cmd/serve -addr :9090 -dir /path
//
// Endpoints:
//
//	POST /api/v1/remember     → crear/fusionar capsula
//	POST /api/v1/recall       → buscar
//	POST /api/v1/suggest      → sugerencias
//	GET  /api/v1/stats        → estadisticas
//	GET  /api/v1/health       → health check
//	POST /api/v1/forget       → olvidar
//	POST /api/v1/resolve      → restaurar
//	POST /api/v1/pin          → fijar prioridad
//	POST /api/v1/backup       → backup
//	POST /api/v1/archive      → archivar viejas
//	POST /api/v1/consolidate  → merge duplicados
//	GET  /api/v1/capsules     → listar activas
//	GET  /healthz             → liveness
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/LRGolden/goulm-memory/pkg/memory"
	"github.com/LRGolden/goulm-memory/pkg/tools"
)

func main() {
	addr := flag.String("addr", ":8080", "direccion del servidor (host:port)")
	dir := flag.String("dir", "", "directorio de memoria (default ~/.goulm-memory/<proyecto>)")
	flag.Parse()

	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal("no se pudo obtener directorio actual:", err)
	}

	memDir := *dir
	if memDir == "" {
		home, _ := os.UserHomeDir()
		memDir = filepath.Join(home, ".goulm-memory", memory.ProjectID(cwd))
	}

	store, err := memory.NewStore(memory.Config{
		Dir:        memDir,
		Format:     memory.FormatJSON,
		Project:    memory.ProjectID(cwd),
		MaxEntries: 100,
		MaxBackups: 10,
	})
	if err != nil {
		log.Fatal("no se pudo abrir la memoria:", err)
	}
	store.SetVocab(memory.ExtractProjectDeps(cwd))

	reg := tools.NewRegistry()
	tools.RegisterMemoryTools(reg, store, nil)

	mux := http.NewServeMux()

	// Endpoint de liveness.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"ok"}`)
	})

	// Endpoints de la API.
	routes := map[string]string{
		"POST /api/v1/remember":    "memory_remember",
		"POST /api/v1/recall":      "memory_recall",
		"POST /api/v1/suggest":     "memory_suggest",
		"GET  /api/v1/stats":       "memory_stats",
		"GET  /api/v1/health":      "memory_stats",
		"POST /api/v1/forget":      "memory_forget",
		"POST /api/v1/resolve":     "memory_resolve",
		"POST /api/v1/pin":         "memory_pin",
		"POST /api/v1/backup":      "memory_backup",
		"POST /api/v1/archive":     "memory_archive",
		"POST /api/v1/consolidate": "memory_consolidate",
		"GET  /api/v1/capsules":    "context_brief",
	}

	for pattern, toolName := range routes {
		mux.HandleFunc(pattern, toolHandler(reg, toolName))
	}

	srv := &http.Server{
		Addr:         *addr,
		Handler:      corsMiddleware(mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// Graceful shutdown.
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	}()

	log.Printf("goulm-memory server en %s (dir: %s)", *addr, memDir)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

// toolHandler crea un handler HTTP que despacha a una tool del registry.
func toolHandler(reg *tools.Registry, toolName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, `{"error":"error leyendo body"}`, http.StatusBadRequest)
			return
		}

		// Para GET sin body, usar "{}"
		if len(body) == 0 {
			body = []byte("{}")
		}

		// Para /api/v1/health, agregar health=true
		if toolName == "memory_stats" && r.URL.Path == "/api/v1/health" {
			var params map[string]interface{}
			json.Unmarshal(body, &params)
			if params == nil {
				params = make(map[string]interface{})
			}
			params["health"] = true
			body, _ = json.Marshal(params)
		}

		t, ok := reg.Get(toolName)
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, `{"error":"tool %s no encontrada"}`, toolName)
			return
		}

		ctx := r.Context()
		result, err := t.Execute(ctx, string(body))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			resp, _ := json.Marshal(map[string]string{"error": err.Error()})
			w.Write(resp)
			return
		}

		// Envolver el resultado en un JSON con el campo "result".
		resp, _ := json.Marshal(map[string]string{"result": result})
		w.Write(resp)
	}
}

// corsMiddleware agrega headers CORS basicos.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
