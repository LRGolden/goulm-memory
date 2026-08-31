package memory

import (
	"context"
	"math"
	"time"
)

// DefaultEmbedTimeout es el timeout por defecto para llamadas Embed.
const DefaultEmbedTimeout = 5 * time.Second

// EmbeddingProvider es la interfaz que deben implementar los proveedores
// de embeddings (OpenAI, Cohere, modelos locales, etc.).
//
// Las implementaciones deben ser seguras para uso concurrente por multiples
// goroutines (el store puede llamar Embed desde multiples Recall simultaneos).
//
// La libreria no importa ningun proveedor. El usuario trae el suyo:
//
//	type MiProvider struct{ apiKey string }
//
//	func (p *MiProvider) Embed(ctx context.Context, text string) ([]float64, error) {
//	    // llamada a la API de embeddings, respetando ctx
//	}
//	func (p *MiProvider) Dimension() int { return 1536 }
type EmbeddingProvider interface {
	// Embed genera un vector de embeddings para el texto dado.
	// El contexto puede cancelarse si el proveedor tarda demasiado.
	Embed(ctx context.Context, text string) ([]float64, error)

	// Dimension devuelve la dimensionalidad del vector.
	Dimension() int
}

// VectorScores calcula similitud coseno entre el query y cada capsula.
// El query se embebe una vez; cada capsula usa su embedding pre-calculado.
// Devuelve un mapa key→score normalizado a [0,1].
// Valida que las dimensiones coincidan entre query y capsulas almacenadas.
func VectorScores(provider EmbeddingProvider, query string, docs []*Capsule) map[string]float64 {
	out := make(map[string]float64, len(docs))
	if provider == nil || query == "" {
		return out
	}

	ctx, cancel := context.WithTimeout(context.Background(), DefaultEmbedTimeout)
	defer cancel()
	qEmb, err := provider.Embed(ctx, query)
	if err != nil || len(qEmb) == 0 {
		return out
	}

	expectedDim := provider.Dimension()
	for _, c := range docs {
		if len(c.Embedding) == 0 {
			continue
		}
		// Validar dimension: si no coincide con el provider actual,
		// saltar la capsula (evita resultados erroneos de cosineSim).
		if expectedDim > 0 && len(c.Embedding) != expectedDim {
			continue
		}
		out[c.Key] = cosineSim(qEmb, c.Embedding)
	}
	return out
}

// vectorScoresVP calcula scores vectoriales usando un VP-Tree pre-construido.
// Devuelve un mapa key→score normalizado a [0,1]. Para cápsulas no encontradas
// en el tree (fuera del radio), asigna score 0.
func vectorScoresVP(provider EmbeddingProvider, query string, tree *VPTree, docs []*Capsule) map[string]float64 {
	out := make(map[string]float64, len(docs))
	if provider == nil || query == "" || tree == nil {
		return out
	}

	ctx, cancel := context.WithTimeout(context.Background(), DefaultEmbedTimeout)
	defer cancel()
	qEmb, err := provider.Embed(ctx, query)
	if err != nil || len(qEmb) == 0 {
		return out
	}

	// Buscar los k más cercanos usando VP-Tree.
	// Usar un radio amplio para cubrir la mayoría de cápsulas.
	results := tree.Search(qEmb, len(docs), 10.0)
	for _, r := range results {
		out[r.Key] = r.Score
	}
	return out
}

// cosineSim calcula la similitud coseno entre dos vectores.
// Utiliza loop unrolling para permitir vectorización SIMD por el compilador.
func cosineSim(a, b []float64) float64 {
	n := len(a)
	if n != len(b) || n == 0 {
		return 0
	}
	var dot, normA, normB float64
	var i int
	// Procesar de a 4 elementos
	for i = 0; i <= n-4; i += 4 {
		dot += a[i]*b[i] + a[i+1]*b[i+1] + a[i+2]*b[i+2] + a[i+3]*b[i+3]
		normA += a[i]*a[i] + a[i+1]*a[i+1] + a[i+2]*a[i+2] + a[i+3]*a[i+3]
		normB += b[i]*b[i] + b[i+1]*b[i+1] + b[i+2]*b[i+2] + b[i+3]*b[i+3]
	}
	// Elementos restantes
	for ; i < n; i++ {
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
