package memory

import (
	"math"
	"strings"
	"time"
)

// qualityMax expone el máximo por factor para documentación y tests.
const (
	qualityMaxTags    = 0.30
	qualityMaxLinks   = 0.15
	qualityMaxLength  = 0.30
	qualityMaxSpec    = 0.10
	qualityMaxFreq    = 0.10
	qualityMaxRecency = 0.10
	qualityMaxOrigin  = 0.10
	qualityStaleCap   = 0.20
)

// QualityScore calcula la calidad (0–1) de una cápsula con heurística pura.
// Factores: tags, links, longitud del contenido, especificidad, frecuencia de
// acceso, recencia de acceso, origen y penalización por obsolescencia.
// Es deliberadamente barata: se ejecuta en cada escritura y merge.
func QualityScore(c *Capsule, now time.Time) float64 {
	if c == nil {
		return 0
	}
	score := 0.0

	// Tags: min(0.30, 0.12 * n) — 3 tags alcanzan el máximo.
	if n := len(c.Tags); n > 0 {
		score += math.Min(qualityMaxTags, float64(n)*0.12)
	}

	// Links: min(0.15, 0.075 * n) — 2 links alcanzan el máximo.
	if n := len(c.Links); n > 0 {
		score += math.Min(qualityMaxLinks, float64(n)*0.075)
	}

	// Longitud del contenido: escalones de detalle.
	switch {
	case len(c.Content) > 200:
		score += qualityMaxLength
	case len(c.Content) > 80:
		score += 0.25
	case len(c.Content) > 30:
		score += 0.15
	}

	// Especificidad: ratio de palabras únicas sobre total.
	words := strings.Fields(strings.ToLower(c.Content))
	if len(words) > 0 {
		unique := make(map[string]bool, len(words))
		for _, w := range words {
			unique[w] = true
		}
		score += math.Min(qualityMaxSpec, float64(len(unique))/float64(len(words))*0.10)
	}

	// Frecuencia de acceso: min(0.10, 0.02 * accessed) — 5 accesos = máximo.
	score += math.Min(qualityMaxFreq, float64(c.Accessed)*0.02)

	// Recencia de acceso.
	if c.LastAccessed != "" {
		if t, err := time.Parse(time.RFC3339, c.LastAccessed); err == nil {
			switch days := int(now.Sub(t).Hours() / 24); {
			case days < 1:
				score += qualityMaxRecency
			case days < 7:
				score += 0.07
			case days < 30:
				score += 0.03
			}
		}
	}

	// Origen: afirmaciones humanas +0.10, inferidas −0.05.
	switch c.Origin {
	case OriginHuman:
		score += 0.10
	case OriginInferred:
		score -= 0.05
	}

	// Obsolescencia: creada y sin acceso durante >30 días → penalización en rampa.
	created, err1 := time.ParseInLocation("2006-01-02", c.Date, time.Local)
	var lastAcc time.Time
	hasLast := c.LastAccessed != ""
	if hasLast {
		lastAcc, err1 = time.Parse(time.RFC3339, c.LastAccessed)
	}
	if err1 == nil {
		daysSinceAccess := int(now.Sub(lastAcc).Hours() / 24)
		if !hasLast {
			daysSinceAccess = int(now.Sub(created).Hours() / 24)
		}
		if now.Sub(created).Hours()/24 > 30 && daysSinceAccess > 30 {
			penalty := 0.20 * math.Min(1, float64(daysSinceAccess-30)/90)
			score -= penalty
		}
	}

	return clamp01(score)
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// Importance es el ranking sin query: recencia × 0.6 + frecuencia × 0.4.
// Recencia se mide sobre la fecha de creación (30 días de ventana).
func Importance(c *Capsule, now time.Time) float64 {
	if c == nil {
		return 0
	}
	created, err := time.ParseInLocation("2006-01-02", c.Date, time.Local)
	if err != nil {
		created = now
	}
	days := now.Sub(created).Hours() / 24
	recency := (30 - days) / 30
	if recency < 0 {
		recency = 0
	}
	if recency > 1 {
		recency = 1
	}
	freq := float64(c.Accessed) / 5
	if freq > 1 {
		freq = 1
	}
	return recency*0.6 + freq*0.4
}
