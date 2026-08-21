package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/LRGolden/goulm-memory/pkg/memory"
)

// RegisterLedgerTools registra las herramientas de consulta del ledger de
// sucesos del proyecto. Requiere un LedgerHook ya inicializado (no-op si el
// ledger está deshabilitado).
func RegisterLedgerTools(r *Registry, hook *LedgerHook) {
	if hook == nil || hook.Ledger() == nil || !hook.Ledger().Enabled {
		return
	}

	r.Register(Tool{
		Name:        "ledger_tail",
		Description: "Consulta los últimos sucesos del proyecto (edits, commits, memoria, hitos, errores). Útil para saber qué se ha hecho recientemente, qué se tocó y qué está pendiente.",
		Category:    CategoryManage,
		RiskLevel:   RiskLow,
		IsReadOnly:  true,
		Tags:        []string{"ledger", "history", "read"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"n":               map[string]interface{}{"type": "integer", "description": "Número de eventos (default 20)"},
				"type":            map[string]interface{}{"type": "string", "description": "Filtro: edit | commit | memory | milestone | error | session"},
				"include_history": map[string]interface{}{"type": "boolean", "description": "Incluir histórico de meses anteriores (default false)"},
			},
		},
		Execute: func(ctx context.Context, input string) (string, error) {
			var p struct {
				N              int    `json:"n"`
				Type           string `json:"type"`
				IncludeHistory bool   `json:"include_history"`
			}
			if err := json.Unmarshal([]byte(input), &p); err != nil {
				return "", fmt.Errorf("parámetros inválidos: %w", err)
			}
			evs := hook.Ledger().Tail(p.N, p.Type, p.IncludeHistory)
			if len(evs) == 0 {
				return "Sin sucesos registrados.", nil
			}
			var sb strings.Builder
			fmt.Fprintf(&sb, "📜 %d últimos sucesos:\n", len(evs))
			for _, ev := range evs {
				sb.WriteString(memory.FormatEvent(ev))
				sb.WriteByte('\n')
			}
			return strings.TrimRight(sb.String(), "\n"), nil
		},
	})

	r.Register(Tool{
		Name:        "ledger_log",
		Description: "Registra un hito manual en el ledger (ej: 'release v0.3', 'deploy a prod', 'refactor de auth completado'). Los hitos quedan en el resumen de sucesos para futuras sesiones.",
		Category:    CategoryManage,
		RiskLevel:   RiskLow,
		Tags:        []string{"ledger", "milestone", "log"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"message": map[string]interface{}{"type": "string", "description": "Descripción breve del hito"},
			},
			"required": []string{"message"},
		},
		Execute: func(ctx context.Context, input string) (string, error) {
			var p struct {
				Message string `json:"message"`
			}
			if err := json.Unmarshal([]byte(input), &p); err != nil {
				return "", fmt.Errorf("parámetros inválidos: %w", err)
			}
			if strings.TrimSpace(p.Message) == "" {
				return "", fmt.Errorf("message es obligatorio")
			}
			hook.Milestone(strings.TrimSpace(p.Message))
			return fmt.Sprintf("⭐ Hito registrado: %s", p.Message), nil
		},
	})
}
