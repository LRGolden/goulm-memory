package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LRGolden/goulm-memory/pkg/memory"
	"github.com/LRGolden/goulm-memory/pkg/tools"
)

func setupTestRegistry(t *testing.T) *tools.Registry {
	t.Helper()
	dir := t.TempDir()
	store, err := memory.NewStore(memory.Config{
		Dir:     dir,
		Project: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	reg := tools.NewRegistry()
	tools.RegisterMemoryTools(reg, store, nil)
	return reg
}

func TestHealthz(t *testing.T) {
	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("healthz = %d, want %d", w.Code, http.StatusOK)
	}
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "ok" {
		t.Errorf("status = %q, want %q", resp["status"], "ok")
	}
}

func TestToolHandlerRemember(t *testing.T) {
	reg := setupTestRegistry(t)

	body := `{"key":"auth-jwt","category":"decision","content":"Usar JWT para auth"}`
	req := httptest.NewRequest("POST", "/api/v1/remember", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler := toolHandler(reg, "memory_remember")
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("remember = %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["result"] == "" {
		t.Error("remember no devolvio resultado")
	}
}

func TestToolHandlerRecall(t *testing.T) {
	reg := setupTestRegistry(t)

	// Primero recordar algo
	body := `{"key":"auth-jwt","category":"decision","content":"Usar JWT para auth"}`
	req := httptest.NewRequest("POST", "/api/v1/remember", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	toolHandler(reg, "memory_remember")(w, req)

	// Luego buscar
	body = `{"q":"auth","limit":5}`
	req = httptest.NewRequest("POST", "/api/v1/recall", bytes.NewBufferString(body))
	w = httptest.NewRecorder()
	toolHandler(reg, "memory_recall")(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("recall = %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestToolHandlerNotFound(t *testing.T) {
	reg := setupTestRegistry(t)

	req := httptest.NewRequest("POST", "/api/v1/unknown", nil)
	w := httptest.NewRecorder()

	handler := toolHandler(reg, "nonexistent_tool")
	handler(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("not found = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestAuthMiddlewareNoKey(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ok":true}`))
	})

	// Sin API key configurada = auth deshabilitada.
	handler := authMiddleware(inner, "")
	req := httptest.NewRequest("GET", "/api/v1/stats", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("no auth config = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestAuthMiddlewareMissingHeader(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ok":true}`))
	})

	handler := authMiddleware(inner, "secret-key")
	req := httptest.NewRequest("GET", "/api/v1/stats", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("missing header = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestAuthMiddlewareWrongKey(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ok":true}`))
	})

	handler := authMiddleware(inner, "secret-key")
	req := httptest.NewRequest("GET", "/api/v1/stats", nil)
	req.Header.Set("X-API-Key", "wrong-key")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("wrong key = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestAuthMiddlewareCorrectKey(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ok":true}`))
	})

	handler := authMiddleware(inner, "secret-key")
	req := httptest.NewRequest("GET", "/api/v1/stats", nil)
	req.Header.Set("X-API-Key", "secret-key")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("correct key = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestAuthMiddlewareBearerToken(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ok":true}`))
	})

	handler := authMiddleware(inner, "secret-key")
	req := httptest.NewRequest("GET", "/api/v1/stats", nil)
	req.Header.Set("Authorization", "Bearer secret-key")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("bearer token = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestAuthMiddlewareHealthzBypass(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})

	handler := authMiddleware(inner, "secret-key")
	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("healthz bypass = %d, want %d", w.Code, http.StatusOK)
	}
}
