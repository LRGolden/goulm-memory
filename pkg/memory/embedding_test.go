package memory

import (
	"encoding/json"
	"math"
	"testing"
)

// mockEmbedder es un proveedor de embeddings ficticio para tests.
type mockEmbedder struct {
	dim int
}

func (m *mockEmbedder) Embed(text string) ([]float64, error) {
	emb := make([]float64, m.dim)
	for i := range emb {
		emb[i] = float64(len(text)) * float64(i+1) / float64(m.dim)
	}
	return emb, nil
}

func (m *mockEmbedder) Dimension() int { return m.dim }

func TestVectorScores(t *testing.T) {
	provider := &mockEmbedder{dim: 3}

	docs := []*Capsule{
		{Key: "a", Embedding: []float64{1, 0, 0}},
		{Key: "b", Embedding: []float64{0, 1, 0}},
		{Key: "c"}, // sin embedding
	}

	scores := VectorScores(provider, "test", docs)

	if _, ok := scores["c"]; ok {
		t.Error("la capsula sin embedding no deberia tener score")
	}
	if scores["a"] == 0 && scores["b"] == 0 {
		t.Error("al menos una capsula deberia tener score > 0")
	}
	for _, sc := range scores {
		if sc < 0 || sc > 1 {
			t.Errorf("score fuera de rango: %f", sc)
		}
	}
}

func TestVectorScoresNilProvider(t *testing.T) {
	docs := []*Capsule{{Key: "a", Embedding: []float64{1, 0}}}
	scores := VectorScores(nil, "test", docs)
	if len(scores) != 0 {
		t.Error("con provider nil, scores debe estar vacio")
	}
}

func TestVectorScoresEmptyQuery(t *testing.T) {
	provider := &mockEmbedder{dim: 2}
	docs := []*Capsule{{Key: "a", Embedding: []float64{1, 0}}}
	scores := VectorScores(provider, "", docs)
	if len(scores) != 0 {
		t.Error("con query vacio, scores debe estar vacio")
	}
}

func TestCosineSim(t *testing.T) {
	tests := []struct {
		a, b []float64
		want float64
	}{
		{[]float64{1, 0}, []float64{1, 0}, 1.0},
		{[]float64{1, 0}, []float64{0, 1}, 0.5},
		{[]float64{1, 0}, []float64{-1, 0}, 0.0},
		{[]float64{}, []float64{}, 0.0},
		{[]float64{1}, []float64{0, 1}, 0.0},
	}

	for _, tc := range tests {
		got := cosineSim(tc.a, tc.b)
		if math.Abs(got-tc.want) > 1e-10 {
			t.Errorf("cosineSim(%v, %v) = %f, want %f", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestCapsuleEmbeddingField(t *testing.T) {
	caps := &Capsule{
		Key:       "test",
		Embedding: []float64{0.1, 0.2, 0.3},
	}

	clone := caps.Clone()
	if len(clone.Embedding) != 3 {
		t.Fatalf("Clone no copio embedding: len=%d", len(clone.Embedding))
	}
	clone.Embedding[0] = 999
	if caps.Embedding[0] == 999 {
		t.Error("Clone no es deep copy del embedding")
	}
}

func TestCapsuleEmbeddingJSON(t *testing.T) {
	caps := &Capsule{Key: "test", Embedding: []float64{0.1, 0.2}}

	data, err := json.Marshal(caps)
	if err != nil {
		t.Fatal(err)
	}

	var out Capsule
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}

	if len(out.Embedding) != 2 {
		t.Fatalf("embedding no persistio: len=%d", len(out.Embedding))
	}
	if out.Embedding[0] != 0.1 || out.Embedding[1] != 0.2 {
		t.Errorf("embedding value mismatch: %v", out.Embedding)
	}
}

func TestCapsuleEmbeddingJSONEmpty(t *testing.T) {
	caps := &Capsule{Key: "test"}

	data, err := json.Marshal(caps)
	if err != nil {
		t.Fatal(err)
	}

	if string(data) != "" {
		var m map[string]json.RawMessage
		json.Unmarshal(data, &m)
		if _, exists := m["embedding"]; exists {
			t.Error("embedding vacio no deberia serializarse")
		}
	}
}

func TestAmbarEmbeddingRoundtrip(t *testing.T) {
	original := []*Capsule{
		{
			ID: "abc123", Key: "test-key", Category: CategoryDecision,
			Content: "test content", Date: "2026-01-01",
			Embedding: []float64{0.123, 0.456, 0.789},
		},
		{
			ID: "def456", Key: "test-key-2", Category: CategoryPattern,
			Content: "other content", Date: "2026-01-02",
		},
	}

	data := MarshalAmbar("test-project", original)
	project, restored, err := UnmarshalAmbar(data)
	if err != nil {
		t.Fatalf("UnmarshalAmbar: %v", err)
	}
	if project != "test-project" {
		t.Errorf("project = %q, want %q", project, "test-project")
	}
	if len(restored) != 2 {
		t.Fatalf("restored %d capsulas, want 2", len(restored))
	}

	if len(restored[0].Embedding) != 3 {
		t.Fatalf("embedding no persistido: len=%d", len(restored[0].Embedding))
	}
	if math.Abs(restored[0].Embedding[0]-0.123) > 1e-10 {
		t.Errorf("embedding[0] = %f, want 0.123", restored[0].Embedding[0])
	}

	if len(restored[1].Embedding) != 0 {
		t.Error("capsula sin embedding no deberia tener embedding restaurado")
	}
}

func TestRememberWithEmbedding(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(Config{Dir: dir, Project: "test"})
	if err != nil {
		t.Fatal(err)
	}

	emb := []float64{0.1, 0.2, 0.3}
	res, err := store.Remember(RememberOptions{
		Key:       "auth-jwt",
		Category:  CategoryDecision,
		Content:   "Usar JWT",
		Embedding: emb,
		Verbatim:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Capsule.Embedding) != 3 {
		t.Fatalf("embedding no guardado: len=%d", len(res.Capsule.Embedding))
	}

	data, _ := store.ExportJSON()
	var caps []*Capsule
	json.Unmarshal(data, &caps)
	if len(caps) == 0 {
		t.Fatal("no se exportaron capsulas")
	}
	if len(caps[0].Embedding) != 3 {
		t.Error("embedding no persistido en JSON")
	}
}

func TestSetEmbedder(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(Config{Dir: dir, Project: "test"})
	if err != nil {
		t.Fatal(err)
	}

	if store.Embedder() != nil {
		t.Error("embedder debe ser nil por defecto")
	}

	provider := &mockEmbedder{dim: 3}
	store.SetEmbedder(provider)

	if store.Embedder() == nil {
		t.Error("embedder debe estar configurado")
	}
}
