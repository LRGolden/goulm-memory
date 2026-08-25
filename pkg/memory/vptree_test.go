package memory

import (
	"fmt"
	"math"
	"testing"
)

func TestVPTreeBuildAndSearch(t *testing.T) {
	caps := []*Capsule{
		{Key: "a", Embedding: []float64{1, 0, 0}},
		{Key: "b", Embedding: []float64{0, 1, 0}},
		{Key: "c", Embedding: []float64{0, 0, 1}},
		{Key: "d", Embedding: []float64{1, 1, 0}},
	}

	tree := BuildVPTree(caps)
	if tree == nil {
		t.Fatal("BuildVPTree devolvió nil")
	}
	if tree.Len() != 4 {
		t.Errorf("Len() = %d, esperaba 4", tree.Len())
	}

	// Buscar nearest neighbor a [1,0,0] → debería ser "a" (dist=0).
	results := tree.Search([]float64{1, 0, 0}, 1, 0)
	if len(results) != 1 {
		t.Fatalf("Search devolvió %d resultados, esperaba 1", len(results))
	}
	if results[0].Key != "a" {
		t.Errorf("nearest neighbor = %q, esperaba %q", results[0].Key, "a")
	}
	if results[0].Distance != 0 {
		t.Errorf("distancia = %f, esperaba 0", results[0].Distance)
	}
}

func TestVPTreeSearchKNearest(t *testing.T) {
	// Crear 10 cápsulas en espacio 2D.
	caps := make([]*Capsule, 10)
	for i := range caps {
		angle := float64(i) * 2 * math.Pi / 10
		caps[i] = &Capsule{
			Key:       fmt.Sprintf("key-%d", i),
			Embedding: []float64{math.Cos(angle), math.Sin(angle)},
		}
	}

	tree := BuildVPTree(caps)
	if tree == nil {
		t.Fatal("BuildVPTree devolvió nil")
	}

	// Buscar 3 nearest neighbors a [1,0] (0°).
	results := tree.Search([]float64{1, 0}, 3, 0)
	if len(results) != 3 {
		t.Fatalf("Search devolvió %d resultados, esperaba 3", len(results))
	}

	// El más cercano debería ser key-0 (0°, dist≈0).
	if results[0].Key != "key-0" {
		t.Errorf("nearest = %q, esperaba %q", results[0].Key, "key-0")
	}

	// Scores deberían estar en [0,1].
	for _, r := range results {
		if r.Score < 0 || r.Score > 1 {
			t.Errorf("score = %f, esperaba [0,1]", r.Score)
		}
	}
}

func TestVPTreeSearchWithMaxDist(t *testing.T) {
	caps := []*Capsule{
		{Key: "a", Embedding: []float64{0, 0}},
		{Key: "b", Embedding: []float64{1, 0}},
		{Key: "c", Embedding: []float64{10, 0}},
	}

	tree := BuildVPTree(caps)

	// Buscar con radio máximo = 2. Solo "a" y "b" deberían aparecer.
	results := tree.Search([]float64{0, 0}, 10, 2)
	if len(results) != 2 {
		t.Errorf("Search con maxDist=2 devolvió %d resultados, esperaba 2", len(results))
	}
	for _, r := range results {
		if r.Key == "c" {
			t.Error("key-c no debería aparecer con maxDist=2")
		}
	}
}

func TestVPTreeNoEmbeddings(t *testing.T) {
	caps := []*Capsule{
		{Key: "a"}, // sin embedding
		{Key: "b"}, // sin embedding
	}

	tree := BuildVPTree(caps)
	if tree != nil {
		t.Error("BuildVPTree sin embeddings válidos debería devolver nil")
	}
}

func TestVPTreePartialEmbeddings(t *testing.T) {
	caps := []*Capsule{
		{Key: "a"},                          // sin embedding
		{Key: "b", Embedding: []float64{1}}, // con embedding
	}

	tree := BuildVPTree(caps)
	if tree == nil {
		t.Fatal("BuildVPTree con 1 embedding válido no debería devolver nil")
	}
	if tree.Len() != 1 {
		t.Errorf("Len() = %d, esperaba 1", tree.Len())
	}
}

func TestVPTreeEmpty(t *testing.T) {
	tree := BuildVPTree(nil)
	if tree != nil {
		t.Error("BuildVPTree(nil) debería devolver nil")
	}
}

func TestVPTreeSearchEmpty(t *testing.T) {
	tree := &VPTree{}
	results := tree.Search([]float64{1, 0}, 5, 0)
	if len(results) != 0 {
		t.Errorf("Search en árbol vacío devolvió %d resultados", len(results))
	}
}

func TestEuclideanDist(t *testing.T) {
	tests := []struct {
		a, b []float64
		want float64
	}{
		{[]float64{0, 0}, []float64{0, 0}, 0},
		{[]float64{0, 0}, []float64{3, 4}, 5},
		{[]float64{1, 2, 3}, []float64{4, 6, 3}, 5},
	}
	for _, tt := range tests {
		got := euclideanDist(tt.a, tt.b)
		if math.Abs(got-tt.want) > 1e-10 {
			t.Errorf("euclideanDist(%v, %v) = %f, esperaba %f", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestDistToScore(t *testing.T) {
	if s := distToScore(0); s != 1.0 {
		t.Errorf("distToScore(0) = %f, esperaba 1.0", s)
	}
	if s := distToScore(1); s >= 1.0 || s <= 0 {
		t.Errorf("distToScore(1) = %f, esperaba (0,1)", s)
	}
	if s := distToScore(10); s >= 0.01 {
		t.Errorf("distToScore(10) = %f, esperaba ~0", s)
	}
}
