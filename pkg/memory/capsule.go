package memory

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Categoría de una cápsula.
type Category string

const (
	CategoryDecision  Category = "decision"
	CategoryPattern   Category = "pattern"
	CategoryBug       Category = "bug"
	CategoryKnowledge Category = "knowledge"
)

// Estado de ciclo de vida de una cápsula.
type Status string

const (
	StatusActive   Status = "active"
	StatusObsolete Status = "obsolete"
)

// Origen de la información de una cápsula.
type Origin string

const (
	OriginHuman    Origin = "human"
	OriginAgent    Origin = "agent"
	OriginInferred Origin = "inferred"
)

// Capsule es la unidad de memoria: un fragmento de conocimiento del proyecto
// preservado entre sesiones (la "cápsula de ámbar").
type Capsule struct {
	ID           string   `json:"id"`
	Category     Category `json:"category"`
	Key          string   `json:"key"`
	Content      string   `json:"content"`
	File         string   `json:"file,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	Date         string   `json:"date"`
	TTL          string   `json:"ttl,omitempty"`
	Accessed     int      `json:"accessed"`
	Links        []string `json:"links,omitempty"`
	Quality      float64  `json:"quality"`
	Confidence   float64  `json:"confidence"`
	LastAccessed string   `json:"last_accessed,omitempty"`
	Priority     int      `json:"priority"`
	PathScope    string   `json:"path_scope,omitempty"`
	Origin       Origin   `json:"origin"`
	Status       Status   `json:"status"`
	SupersededOn string    `json:"superseded_on,omitempty"`
	Embedding    []float64 `json:"embedding,omitempty"`
}

// keyRE valida claves kebab-case (sin colones: están reservados para typed links).
var keyRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// NewCapsule construye una cápsula con defaults seguros.
func NewCapsule(cat Category, key, content string) (*Capsule, error) {
	if !ValidCategory(cat) {
		return nil, fmt.Errorf("categoría inválida: %q (usa decision, pattern, bug o knowledge)", cat)
	}
	if !keyRE.MatchString(key) {
		return nil, fmt.Errorf("clave inválida: %q (kebab-case, solo a-z0-9-)", key)
	}
	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("el contenido no puede estar vacío")
	}
	return &Capsule{
		ID:         NewID(),
		Category:   cat,
		Key:        key,
		Content:    strings.TrimSpace(content),
		Date:       time.Now().Format("2006-01-02"),
		Confidence: ConfidenceFor(OriginAgent),
		Origin:     OriginAgent,
		Status:     StatusActive,
	}, nil
}

// ValidCategory comprueba que la categoría sea una de las 4 canónicas.
func ValidCategory(c Category) bool {
	switch c {
	case CategoryDecision, CategoryPattern, CategoryBug, CategoryKnowledge:
		return true
	}
	return false
}

// ValidStatus comprueba que el estado sea uno de los canónicos.
func ValidStatus(s Status) bool {
	switch s {
	case StatusActive, StatusObsolete:
		return true
	}
	return false
}

// ValidOrigin comprueba que el origen sea uno de los canónicos.
func ValidOrigin(o Origin) bool {
	switch o {
	case OriginHuman, OriginAgent, OriginInferred:
		return true
	}
	return false
}

// ConfidenceFor devuelve la confianza por defecto según el origen.
func ConfidenceFor(o Origin) float64 {
	switch o {
	case OriginHuman:
		return 1.0
	case OriginInferred:
		return 0.6
	default:
		return 0.8
	}
}

// NewID genera un identificador hex aleatorio de 8 caracteres (4 bytes).
func NewID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return strings.ToLower(time.Now().Format("150405"))
	}
	return hex.EncodeToString(b)
}

// IsExpired indica si la cápsula superó su TTL en la fecha dada.
// El día del TTL es el último día válido (no está expirada).
func (c *Capsule) IsExpired(now time.Time) bool {
	if c.TTL == "" {
		return false
	}
	return c.TTL < now.Format("2006-01-02")
}

// IsVisible indica si la cápsula debe aparecer en recalls normales.
// asOf permite la vista temporal (p. ej. cápsulas superseded después de una fecha).
func (c *Capsule) IsVisible(now time.Time, asOf string) bool {
	if c.Status == StatusObsolete {
		if asOf != "" && c.SupersededOn != "" && c.SupersededOn > asOf {
			return true
		}
		return false
	}
	if c.IsExpired(now) {
		return false
	}
	if asOf != "" && c.Date > asOf {
		return false
	}
	return true
}

// FullText devuelve el texto completo sobre el que se hace match y BM25.
func (c *Capsule) FullText() string {
	return strings.Join([]string{
		c.ID, string(c.Category), c.Key, c.Content, c.File,
		strings.Join(c.Tags, " "), c.PathScope,
	}, " ")
}

// BumpAccess registra un acceso y devuelve true si cambió algo.
func (c *Capsule) BumpAccess(now time.Time) {
	c.Accessed++
	c.LastAccessed = now.UTC().Format(time.RFC3339)
}

// Normalized devuelve el contenido normalizado (lowercase, whitespace simple)
// usado para detección de duplicados exactos.
func (c *Capsule) Normalized() string {
	return normalizeSpaces(strings.ToLower(c.Content))
}

func normalizeSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// ResolveTTL convierte un TTL relativo (7d, 30d) o absoluto (YYYY-MM-DD) en
// fecha absoluta YYYY-MM-DD. Devuelve vacío si el formato es inválido.
func ResolveTTL(ttl string, now time.Time) string {
	ttl = strings.TrimSpace(ttl)
	if ttl == "" {
		return ""
	}
	if strings.HasSuffix(ttl, "d") {
		n := 0
		if _, err := fmt.Sscanf(ttl, "%dd", &n); err == nil && n > 0 {
			return now.AddDate(0, 0, n).Format("2006-01-02")
		}
		return ""
	}
	if _, err := time.Parse("2006-01-02", ttl); err == nil {
		return ttl
	}
	return ""
}

// ApplyTTL resuelve y asigna el TTL de la cápsula.
func (c *Capsule) ApplyTTL(ttl string, now time.Time) {
	c.TTL = ResolveTTL(ttl, now)
}

// Clone devuelve una copia profunda (slices incluidos).
func (c *Capsule) Clone() *Capsule {
	out := *c
	out.Tags = append([]string(nil), c.Tags...)
	out.Links = append([]string(nil), c.Links...)
	out.Embedding = append([]float64(nil), c.Embedding...)
	return &out
}
