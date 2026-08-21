package memory

import "math"

// EmbeddingProvider es la interfaz que deben implementar los proveedores
// de embeddings (OpenAI, Cohere, modelos locales, etc.).
//
// La libreria no importa ningun proveedor. El usuario trae el suyo:
//
//	type MiProvider struct{ apiKey string }
//
//	func (p *MiProvider) Embed(text string) ([]float64, error) {
//	    // llamada a la API de embeddings
//	}
//	func (p *MiProvider) Dimension() int { return 1536 }
type EmbeddingProvider interface {
	// Embed genera un vector de embeddings para el texto dado.
	Embed(text string) ([]float64, error)

	// Dimension devuelve la dimensionality del vector.
	Dimension() int
}

// VectorScores calcula similitud coseno entre el query y cada capsula.
// El query se embebe una vez; cada capsula usa su embedding pre-calculado.
// Devuelve un mapa key→score normalizado a [0,1].
func VectorScores(provider EmbeddingProvider, query string, docs []*Capsule) map[string]float64 {
	out := make(map[string]float64, len(docs))
	if provider == nil || query == "" {
		return out
	}

	qEmb, err := provider.Embed(query)
	if err != nil || len(qEmb) == 0 {
		return out
	}

	for _, c := range docs {
		if len(c.Embedding) == 0 {
			continue
		}
		out[c.Key] = cosineSim(qEmb, c.Embedding)
	}
	return out
}

// cosineSim calcula la similitud coseno entre dos vectores.
func cosineSim(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return (dot/denom + 1) / 2 // normalizar a [0,1]
}
