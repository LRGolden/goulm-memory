package memory

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

// HealthReport es el resultado de la auditoría de salud.
type HealthReport struct {
	Score           int      `json:"score"`
	Entries         int      `json:"entries"`
	AvgQuality      float64  `json:"avg_quality"`
	OrphanLinks     []string `json:"orphan_links"`
	ExactDuplicates int      `json:"exact_duplicates"`
	ExpiredTTL      []string `json:"expired_ttl"`
	BrokenFiles     []string `json:"broken_files"`
	MissingEvidence []string `json:"missing_evidence"`
	StaleClaims     []string `json:"stale_claims"`
	Secrets         []string `json:"secrets"`
	Warnings        int      `json:"warnings"`
}

// secretRE detecta patrones de secretos comunes (solo para avisar).
// Nota: las claves Anthropic llevan guiones (sk-ant-api03-...).
var secretRE = regexp.MustCompile(`(?i)\b(sk-[a-z0-9-]{20,}|AKIA[0-9A-Z]{16}|ghp_[a-zA-Z0-9]{20,}|xox[baprs]-[a-zA-Z0-9-]{10,}|-----BEGIN [A-Z]{0,50}PRIVATE KEY-----|Bearer [a-zA-Z0-9._-]{20,})\b`)

// Health audita el almacén: links huérfanos, duplicados exactos, TTL
// expirados, referencias a archivos inexistentes, path_scope sin archivo,
// claims solapados y posibles secretos. Score 0-100.
func (s *MemoryStore) Health(cwd string) (HealthReport, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	rep := HealthReport{Score: 100}

	// TTL expirados (-3): se cuentan antes de filtrar la visibilidad.
	active := make([]*Capsule, 0, len(s.entries))
	for _, c := range s.entries {
		if c.TTL != "" && c.Status == StatusActive && c.IsExpired(now) {
			rep.ExpiredTTL = append(rep.ExpiredTTL, c.Key)
			continue
		}
		if c.IsVisible(now, "") {
			active = append(active, c)
		}
	}
	rep.Entries = len(active)
	rep.Warnings += len(rep.ExpiredTTL)
	rep.Score -= 3 * len(rep.ExpiredTTL)

	var qSum float64
	for _, c := range active {
		qSum += c.Quality
	}
	if len(active) > 0 {
		rep.AvgQuality = qSum / float64(len(active))
	}

	keys := make(map[string]bool, len(active))
	for _, c := range active {
		keys[c.Key] = true
	}

	// Links huérfanos (-5).
	for _, c := range active {
		for _, link := range c.Links {
			if !keys[LinkKey(link)] {
				rep.OrphanLinks = append(rep.OrphanLinks, fmt.Sprintf("%s → %s", c.Key, LinkKey(link)))
			}
		}
	}
	rep.Warnings += len(rep.OrphanLinks)
	rep.Score -= 5 * len(rep.OrphanLinks)

	// Duplicados exactos (-5).
	seen := make(map[string]bool)
	for _, c := range active {
		norm := c.Normalized()
		if norm == "" {
			continue
		}
		if seen[norm] {
			rep.ExactDuplicates++
		} else {
			seen[norm] = true
		}
	}
	if rep.ExactDuplicates > 0 {
		rep.Score -= 5
		rep.Warnings++
	}

	// Archivos rotos y path_scope sin archivo (-5 / -3).
	for _, c := range active {
		if c.File == "" {
			if c.PathScope != "" {
				rep.MissingEvidence = append(rep.MissingEvidence, c.Key)
			}
			continue
		}
		path := c.File
		if i := strings.IndexByte(path, ':'); i >= 0 {
			// "file:line" del agente: recortar solo si el path completo no
			// existe (un archivo llamado "a:b.txt" no se malinterpreta).
			if !fileExists(path, cwd) {
				path = path[:i]
			}
		}
		path = strings.TrimPrefix(path, "/")
		if !fileExists(path, cwd) {
			rep.BrokenFiles = append(rep.BrokenFiles, fmt.Sprintf("%s (%s)", c.Key, c.File))
		}
	}
	rep.Warnings += len(rep.BrokenFiles)
	rep.Score -= 5 * len(rep.BrokenFiles)
	rep.Warnings += len(rep.MissingEvidence)
	rep.Score -= 3 * len(rep.MissingEvidence)

	// Claims solapados: Jaccard > 0.4 en la misma categoría (-3 por par).
	for i := 0; i < len(active); i++ {
		for j := i + 1; j < len(active); j++ {
			a, b := active[i], active[j]
			if a.Category != b.Category || a.Key == b.Key {
				continue
			}
			if Jaccard(a.Content, b.Content) > 0.4 {
				rep.StaleClaims = append(rep.StaleClaims, fmt.Sprintf("%s ~ %s", a.Key, b.Key))
			}
		}
	}
	rep.Warnings += len(rep.StaleClaims)
	rep.Score -= 3 * len(rep.StaleClaims)

	// Posibles secretos (aviso, sin penalización fuerte).
	for _, c := range active {
		if secretRE.MatchString(c.Content) {
			rep.Secrets = append(rep.Secrets, c.Key)
		}
	}

	// Más de 80 cápsulas activas (-3).
	if len(active) > 80 {
		rep.Score -= 3
		rep.Warnings++
	}

	if rep.Score < 0 {
		rep.Score = 0
	}
	return rep, nil
}

// healthProblemCount calcula los problemas reales; si el reporte no los
// contó (Warnings == 0), los deduce de las listas para renders coherentes.
func healthProblemCount(rep HealthReport) int {
	if rep.Warnings > 0 {
		return rep.Warnings
	}
	return len(rep.OrphanLinks) + rep.ExactDuplicates + len(rep.ExpiredTTL) +
		len(rep.BrokenFiles) + len(rep.MissingEvidence) + len(rep.StaleClaims) +
		len(rep.Secrets)
}

// RenderHealth formatea la auditoría para el agente.
func RenderHealth(rep HealthReport) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Memoria — Salud (score: %d/100)\n\n", rep.Score))
	sb.WriteString(fmt.Sprintf("## Resumen\n- %d cápsulas activas\n- Calidad media: %.0f%%\n",
		rep.Entries, rep.AvgQuality*100))
	problems := healthProblemCount(rep)
	if problems == 0 {
		sb.WriteString("\nSin problemas detectados. ✅\n")
		return strings.TrimRight(sb.String(), "\n")
	}
	sb.WriteString(fmt.Sprintf("\n## Problemas (%d)\n", problems))
	if len(rep.OrphanLinks) > 0 {
		sb.WriteString(fmt.Sprintf("- Links huérfanos (%d): %s\n", len(rep.OrphanLinks), strings.Join(rep.OrphanLinks, ", ")))
	}
	if rep.ExactDuplicates > 0 {
		sb.WriteString(fmt.Sprintf("- Duplicados exactos: %d\n", rep.ExactDuplicates))
	}
	if len(rep.ExpiredTTL) > 0 {
		sb.WriteString(fmt.Sprintf("- TTL expirado (%d): %s\n", len(rep.ExpiredTTL), strings.Join(rep.ExpiredTTL, ", ")))
	}
	if len(rep.BrokenFiles) > 0 {
		sb.WriteString(fmt.Sprintf("- Archivos inexistentes (%d): %s\n", len(rep.BrokenFiles), strings.Join(rep.BrokenFiles, ", ")))
	}
	if len(rep.MissingEvidence) > 0 {
		sb.WriteString(fmt.Sprintf("- path_scope sin archivo (%d): %s\n", len(rep.MissingEvidence), strings.Join(rep.MissingEvidence, ", ")))
	}
	if len(rep.StaleClaims) > 0 {
		sb.WriteString(fmt.Sprintf("- Claims solapados (%d): %s\n", len(rep.StaleClaims), strings.Join(rep.StaleClaims, ", ")))
	}
	if len(rep.Secrets) > 0 {
		sb.WriteString(fmt.Sprintf("- ⚠️ Posibles secretos (%d): %s\n", len(rep.Secrets), strings.Join(rep.Secrets, ", ")))
	}
	sb.WriteString("\nSugerencias: memory_consolidate para duplicados, memory_archive para expiradas, memory_forget para huérfanas.\n")
	return strings.TrimRight(sb.String(), "\n")
}

// fileExists comprueba si un path existe (relativo al cwd). Sin cwd, un path
// relativo se considera no verificable (comportamiento previo: no reportar).
func fileExists(p, cwd string) bool {
	if filepathIsAbs(p) {
		_, err := os.Stat(p)
		return err == nil
	}
	if cwd == "" {
		return true
	}
	full := strings.TrimRight(cwd, `/\`) + "/" + p
	_, err := os.Stat(full)
	return err == nil
}

func filepathIsAbs(p string) bool {
	if len(p) >= 3 && p[1] == ':' && (p[2] == '/' || p[2] == '\\') {
		return true
	}
	if len(p) >= 1 && (p[0] == '/' || p[0] == '\\') {
		return true
	}
	return false
}
