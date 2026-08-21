package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/LRGolden/goulm-memory/pkg/memory"
)

// RegisterMemoryTools registra las 11 herramientas de memoria persistente y el
// briefing de contexto (context_brief). Requiere un *memory.MemoryStore ya
// inicializado y, opcionalmente, el SessionTracker de la sesión actual (para
// el sesgo de sesión del ranking y context_brief).
func RegisterMemoryTools(r *Registry, store *memory.MemoryStore, tracker *memory.SessionTracker) {
	if store == nil {
		return
	}

	// ── memory_remember ────────────────────────────────────────────────
	r.Register(Tool{
		Name:        "memory_remember",
		Description: "Guarda una cápsula de memoria: decisión, patrón, bug o conocimiento del proyecto. Misma clave = merge (unión de tags, max confianza). Usa esto ANTES de implementar una decisión no trivial y al resolver bugs complejos.",
		Category:    CategoryManage,
		RiskLevel:   RiskLow,
		Tags:        []string{"memory", "save"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"category":   map[string]interface{}{"type": "string", "description": "decision | pattern | bug | knowledge"},
				"key":        map[string]interface{}{"type": "string", "description": "Clave kebab-case única (ej: use-zod)"},
				"content":    map[string]interface{}{"type": "string", "description": "Texto detallado"},
				"file":       map[string]interface{}{"type": "string", "description": "Archivo relacionado (opcional)"},
				"tags":       map[string]interface{}{"type": "string", "description": "Tags separados por ';' (vacío = auto-inferidos)"},
				"ttl":        map[string]interface{}{"type": "string", "description": "Caducidad: 7d, 30d o YYYY-MM-DD (opcional)"},
				"links":      map[string]interface{}{"type": "string", "description": "Claves relacionadas separadas por ';' (grafo)"},
				"priority":   map[string]interface{}{"type": "integer", "description": "Fijar 1-5 (0 = normal)"},
				"path_scope": map[string]interface{}{"type": "string", "description": "Glob de archivos (opcional)"},
			},
			"required": []string{"category", "key", "content"},
		},
		Execute: func(ctx context.Context, input string) (string, error) {
			var p struct {
				Category  string `json:"category"`
				Key       string `json:"key"`
				Content   string `json:"content"`
				File      string `json:"file"`
				Tags      string `json:"tags"`
				TTL       string `json:"ttl"`
				Links     string `json:"links"`
				Priority  int    `json:"priority"`
				PathScope string `json:"path_scope"`
			}
			if err := json.Unmarshal([]byte(input), &p); err != nil {
				return "", fmt.Errorf("parámetros inválidos: %w", err)
			}
			res, err := store.Remember(memory.RememberOptions{
				Category:  memory.Category(p.Category),
				Key:       p.Key,
				Content:   p.Content,
				File:      p.File,
				Tags:      splitList(p.Tags),
				TTL:       p.TTL,
				Links:     splitList(p.Links),
				Priority:  p.Priority,
				PathScope: p.PathScope,
			})
			if err != nil {
				return "", err
			}
			action := "Guardado"
			if res.Merged {
				action = "Actualizado (merge)"
			}
			out := fmt.Sprintf("🧠 %s: %s/%s (%s)\nCalidad: %.2f",
				action, res.Capsule.Category, res.Capsule.Key, res.Capsule.ID, res.Capsule.Quality)
			if len(res.Inferred) > 0 {
				out += "\n🏷️ Tags inferidos: " + strings.Join(res.Inferred, ";")
			}
			return out, nil
		},
	})

	// ── memory_recall ──────────────────────────────────────────────────
	r.Register(Tool{
		Name:        "memory_recall",
		Description: "Busca en la memoria ANTES de leer archivos. Combina BM25 + grafo de conocimiento + calidad. Con `intent` hace recall unificado para inicio de tarea (BM25+grafo+calidad+decay). Filtros: category, tags (AND), fechas, path_scope. Budget: tiny (clave+1 línea), normal, deep (todos los campos).",
		Category:    CategoryManage,
		RiskLevel:   RiskLow,
		IsReadOnly:  true,
		Tags:        []string{"memory", "search"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query":      map[string]interface{}{"type": "string", "description": "Texto a buscar"},
				"intent":     map[string]interface{}{"type": "string", "description": "Qué vas a hacer (ej: 'diseño de base de datos'). Activa recall unificado compacto"},
				"category":   map[string]interface{}{"type": "string", "description": "Filtro: decision | pattern | bug | knowledge"},
				"tags":       map[string]interface{}{"type": "string", "description": "Filtro AND, ';'-separados"},
				"from_date":  map[string]interface{}{"type": "string", "description": "YYYY-MM-DD"},
				"to_date":    map[string]interface{}{"type": "string", "description": "YYYY-MM-DD"},
				"path_scope": map[string]interface{}{"type": "string", "description": "Glob de archivos"},
				"limit":      map[string]interface{}{"type": "integer", "description": "Máx resultados (default 6)"},
				"graph":      map[string]interface{}{"type": "boolean", "description": "Expandir vecinos del grafo (default false)"},
				"hops":       map[string]interface{}{"type": "integer", "description": "1 o 2 (con graph: true)"},
				"budget":     map[string]interface{}{"type": "string", "description": "tiny | normal | deep (default normal)"},
			},
			"required": []string{},
		},
		Execute: func(ctx context.Context, input string) (string, error) {
			var p struct {
				Query     string `json:"query"`
				Intent    string `json:"intent"`
				Category  string `json:"category"`
				Tags      string `json:"tags"`
				FromDate  string `json:"from_date"`
				ToDate    string `json:"to_date"`
				PathScope string `json:"path_scope"`
				Limit     int    `json:"limit"`
				Graph     bool   `json:"graph"`
				Hops      int    `json:"hops"`
				Budget    string `json:"budget"`
			}
			if err := json.Unmarshal([]byte(input), &p); err != nil {
				return "", fmt.Errorf("parámetros inválidos: %w", err)
			}
			if p.Intent != "" {
				rs, err := store.SmartRecall(p.Intent, p.Limit, sessionFiles(tracker))
				if err != nil {
					return "", err
				}
				return memory.Render(rs, memory.BudgetNormal), nil
			}
			rs, err := store.Recall(p.Query, &memory.Query{
				Category:     memory.Category(p.Category),
				Tags:         splitList(p.Tags),
				FromDate:     p.FromDate,
				ToDate:       p.ToDate,
				PathScope:    p.PathScope,
				Limit:        p.Limit,
				Graph:        p.Graph,
				Hops:         p.Hops,
				SessionFiles: sessionFiles(tracker),
			})
			if err != nil {
				return "", err
			}
			return memory.Render(rs, budgetOf(p.Budget)), nil
		},
	})

	// ── memory_stats ───────────────────────────────────────────────────
	r.Register(Tool{
		Name:        "memory_stats",
		Description: "Estado de la memoria: total, conteos por categoría, archivadas, fijadas, expiradas, calidad media. Con `health:true` añade score de salud (0-100: links huérfanos, duplicados, TTL, posibles secretos). Con `recent` muestra qué se aprendió desde hace 24h/7d/fecha. Úsalo al inicio de sesión.",
		Category:    CategoryManage,
		RiskLevel:   RiskLow,
		IsReadOnly:  true,
		Tags:        []string{"memory", "stats"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"health": map[string]interface{}{"type": "boolean", "description": "Incluir score de salud 0-100 (default false)"},
				"recent": map[string]interface{}{"type": "string", "description": "Cápsulas nuevas desde: 24h | 7d | YYYY-MM-DD (default: no mostrar)"},
			},
			"required": []string{},
		},
		Execute: func(ctx context.Context, input string) (string, error) {
			var p struct {
				Health bool   `json:"health"`
				Recent string `json:"recent"`
			}
			_ = json.Unmarshal([]byte(input), &p)
			st, err := store.Stats()
			if err != nil {
				return "", err
			}
			var sb strings.Builder
			sb.WriteString(memory.RenderStats(st))
			if p.Health {
				rep, err := store.Health(".")
				if err != nil {
					return "", err
				}
				sb.WriteString("\n\n" + memory.RenderHealth(rep))
			}
			if p.Recent != "" {
				rep, err := store.Diff(p.Recent)
				if err != nil {
					return "", err
				}
				sb.WriteString("\n\n" + memory.RenderDiff(rep))
			}
			return sb.String(), nil
		},
	})

	// ── memory_forget ──────────────────────────────────────────────────
	r.Register(Tool{
		Name:        "memory_forget",
		Description: "Elimina una cápsula por clave. Soft-delete por defecto (status=obsolete, restaurar con memory_resolve). Usa hard:true para borrado físico.",
		Category:    CategoryManage,
		RiskLevel:   RiskLow,
		Tags:        []string{"memory", "delete"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"key":  map[string]interface{}{"type": "string", "description": "Clave de la cápsula"},
				"hard": map[string]interface{}{"type": "boolean", "description": "Borrado físico (default false)"},
			},
			"required": []string{"key"},
		},
		Execute: func(ctx context.Context, input string) (string, error) {
			var p struct {
				Key  string `json:"key"`
				Hard bool   `json:"hard"`
			}
			if err := json.Unmarshal([]byte(input), &p); err != nil {
				return "", fmt.Errorf("parámetros inválidos: %w", err)
			}
			ok, err := store.Forget(p.Key, p.Hard)
			if err != nil {
				return "", err
			}
			if !ok {
				return fmt.Sprintf("No se encontró la cápsula '%s'", p.Key), nil
			}
			if p.Hard {
				return fmt.Sprintf("🗑️ Eliminada: %s", p.Key), nil
			}
			return fmt.Sprintf("🗑️ Marcada como obsoleta: %s (memory_resolve para restaurar)", p.Key), nil
		},
	})

	// ── memory_resolve ─────────────────────────────────────────────────
	r.Register(Tool{
		Name:        "memory_resolve",
		Description: "Restaura una cápsula que fue soft-deleted (status=obsolete) a active. Revierte memory_forget.",
		Category:    CategoryManage,
		RiskLevel:   RiskLow,
		Tags:        []string{"memory", "restore"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"key": map[string]interface{}{"type": "string", "description": "Clave de la cápsula"},
			},
			"required": []string{"key"},
		},
		Execute: func(ctx context.Context, input string) (string, error) {
			var p struct {
				Key string `json:"key"`
			}
			if err := json.Unmarshal([]byte(input), &p); err != nil {
				return "", fmt.Errorf("parámetros inválidos: %w", err)
			}
			ok, err := store.Resolve(p.Key)
			if err != nil {
				return "", err
			}
			if !ok {
				return fmt.Sprintf("No se encontró la cápsula '%s'", p.Key), nil
			}
			return fmt.Sprintf("♻️ Restaurada: %s", p.Key), nil
		},
	})

	// ── memory_archive ─────────────────────────────────────────────────
	r.Register(Tool{
		Name:        "memory_archive",
		Description: "Mueve al archivo las cápsulas con >30 días o TTL expirado. Ejecútalo periódicamente para mantener la memoria limpia.",
		Category:    CategoryManage,
		RiskLevel:   RiskLow,
		Tags:        []string{"memory", "maintenance"},
		Parameters:  map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		Execute: func(ctx context.Context, input string) (string, error) {
			n, err := store.ArchiveOld()
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("📦 Archivadas %d cápsulas", n), nil
		},
	})

	// ── memory_backup ──────────────────────────────────────────────────
	r.Register(Tool{
		Name:        "memory_backup",
		Description: "Crea una copia con timestamp de la memoria (podada a las N más recientes). Úsalo antes de refactors grandes.",
		Category:    CategoryManage,
		RiskLevel:   RiskLow,
		Tags:        []string{"memory", "backup"},
		Parameters:  map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		Execute: func(ctx context.Context, input string) (string, error) {
			path, err := store.Backup()
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("💾 Backup creado: %s", path), nil
		},
	})

	// ── memory_pin ─────────────────────────────────────────────────────
	r.Register(Tool{
		Name:        "memory_pin",
		Description: "Fija una cápsula con prioridad 1-5: siempre aparece primero en los resultados de búsqueda. priority=0 la desfija.",
		Category:    CategoryManage,
		RiskLevel:   RiskLow,
		Tags:        []string{"memory", "pin"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"key":      map[string]interface{}{"type": "string", "description": "Clave de la cápsula"},
				"priority": map[string]interface{}{"type": "integer", "description": "1-5 (0 = desfijar)"},
			},
			"required": []string{"key", "priority"},
		},
		Execute: func(ctx context.Context, input string) (string, error) {
			var p struct {
				Key      string `json:"key"`
				Priority int    `json:"priority"`
			}
			if err := json.Unmarshal([]byte(input), &p); err != nil {
				return "", fmt.Errorf("parámetros inválidos: %w", err)
			}
			ok, err := store.Pin(p.Key, p.Priority)
			if err != nil {
				return "", err
			}
			if !ok {
				return fmt.Sprintf("No se encontró la cápsula '%s'", p.Key), nil
			}
			if p.Priority == 0 {
				return fmt.Sprintf("📌 Desfijada: %s", p.Key), nil
			}
			return fmt.Sprintf("📌 Fijada: %s (prioridad %d)", p.Key, p.Priority), nil
		},
	})

	// ── memory_consolidate ─────────────────────────────────────────────
	r.Register(Tool{
		Name:        "memory_consolidate",
		Description: "Fusiona duplicados: misma clave (merge), contenido idéntico y near-duplicates (similitud ≥0.7, misma categoría). Determinista, sin LLM.",
		Category:    CategoryManage,
		RiskLevel:   RiskLow,
		Tags:        []string{"memory", "maintenance"},
		Parameters:  map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		Execute: func(ctx context.Context, input string) (string, error) {
			rep, err := store.Consolidate()
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("🧹 Consolidación: %d merges, %d near-duplicates, %d eliminados (%d → %d)",
				rep.Merged, rep.NearDuplicates, rep.Removed, rep.Before, rep.After), nil
		},
	})

	// ── memory_suggest ─────────────────────────────────────────────────
	r.Register(Tool{
		Name:        "memory_suggest",
		Description: "Encuentra cápsulas relacionadas a un contexto cuando no sabes qué buscar exactamente. Úsalo durante la exploración antes de grep/read.",
		Category:    CategoryManage,
		RiskLevel:   RiskLow,
		IsReadOnly:  true,
		Tags:        []string{"memory", "search"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"context": map[string]interface{}{"type": "string", "description": "Descripción del problema o contexto"},
				"limit":   map[string]interface{}{"type": "integer", "description": "Máx resultados (default 5)"},
			},
			"required": []string{"context"},
		},
		Execute: func(ctx context.Context, input string) (string, error) {
			var p struct {
				Context string `json:"context"`
				Limit   int    `json:"limit"`
			}
			if err := json.Unmarshal([]byte(input), &p); err != nil {
				return "", fmt.Errorf("parámetros inválidos: %w", err)
			}
			rs, err := store.Suggest(p.Context, p.Limit, sessionFiles(tracker))
			if err != nil {
				return "", err
			}
			return memory.Render(rs, memory.BudgetNormal), nil
		},
	})

	// ── context_brief ──────────────────────────────────────────────────
	r.Register(Tool{
		Name:        "context_brief",
		Description: "Briefing compacto en una llamada: primer de memoria (top cápsulas + conteos por categoría) + sesiones activas + conflictos. Reemplaza 3-4 llamadas separadas.",
		Category:    CategoryManage,
		RiskLevel:   RiskLow,
		IsReadOnly:  true,
		Tags:        []string{"memory", "context"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"limit": map[string]interface{}{"type": "integer", "description": "Máx cápsulas del top (default 5)"},
			},
			"required": []string{},
		},
		Execute: func(ctx context.Context, input string) (string, error) {
			var p struct {
				Limit int `json:"limit"`
			}
			_ = json.Unmarshal([]byte(input), &p)
			primer, err := store.Primer(p.Limit)
			if err != nil {
				return "", err
			}
			if tracker == nil {
				return "", fmt.Errorf("memoria no disponible")
			}
			sessions, err := tracker.ActiveSessions()
			if err != nil {
				return "", err
			}
			conflicts, err := tracker.Conflicts()
			if err != nil {
				return "", err
			}
			var sb strings.Builder
			sb.WriteString(primer)
			sb.WriteString("\n\n" + memory.RenderSessions(sessions, conflicts, false))
			return sb.String(), nil
		},
	})
}

// splitList convierte un string ';'-separado en []string.
func splitList(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ";")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// budgetOf normaliza el presupuesto de salida.
func budgetOf(b string) memory.Budget {
	switch b {
	case "tiny":
		return memory.BudgetTiny
	case "deep":
		return memory.BudgetDeep
	default:
		return memory.BudgetNormal
	}
}

// sessionFiles devuelve los archivos tocados por la sesión actual, o nil si no
// hay tracker. Alimenta el sesgo de sesión del ranking.
func sessionFiles(tracker *memory.SessionTracker) map[string]bool {
	if tracker == nil {
		return nil
	}
	return tracker.SessionFiles()
}
