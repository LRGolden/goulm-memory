package memory

import (
	"context"
	"fmt"
	"testing"
)

func BenchmarkBM25Scores(b *testing.B) {
	for _, n := range []int{10, 100, 500, 1000} {
		b.Run(fmt.Sprintf("N=%d", n), func(b *testing.B) {
			dir := b.TempDir()
			s, err := NewStore(Config{
				Dir:        dir,
				Format:     FormatJSON,
				Project:    "bench",
				MaxEntries: n + 100,
			})
			if err != nil {
				b.Fatal(err)
			}

			for i := 0; i < n; i++ {
				s.Remember(RememberOptions{
					Category: CategoryDecision,
					Key:      fmt.Sprintf("key-%d", i),
					Content:  fmt.Sprintf("Decidimos usar Go para el backend por su rendimiento y simplicidad %d", i),
					Tags:     []string{"go", "backend", "decision"},
				})
			}

			docs := s.ListActive(0)
			query := "Go backend rendimiento"

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				BM25Scores(query, docs)
			}
			b.ReportAllocs()
		})
	}
}

func BenchmarkVectorScores(b *testing.B) {
	embedder := &mockEmbedder{dim: 8}

	for _, n := range []int{10, 100, 500, 1000} {
		b.Run(fmt.Sprintf("N=%d", n), func(b *testing.B) {
			dir := b.TempDir()
			s, err := NewStore(Config{
				Dir:        dir,
				Format:     FormatJSON,
				Project:    "bench",
				MaxEntries: n + 100,
			})
			if err != nil {
				b.Fatal(err)
			}
			s.SetEmbedder(embedder)

			for i := 0; i < n; i++ {
				emb, _ := embedder.Embed(context.Background(), fmt.Sprintf("contenido %d", i))
				s.Remember(RememberOptions{
					Category:  CategoryDecision,
					Key:       fmt.Sprintf("key-%d", i),
					Content:   fmt.Sprintf("Contenido de prueba %d", i),
					Embedding: emb,
				})
			}

			docs := s.ListActive(0)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				VectorScores(embedder, "test query", docs)
			}
			b.ReportAllocs()
		})
	}
}

func BenchmarkBuildGraph(b *testing.B) {
	for _, n := range []int{10, 100, 500} {
		b.Run(fmt.Sprintf("N=%d", n), func(b *testing.B) {
			dir := b.TempDir()
			s, err := NewStore(Config{
				Dir:        dir,
				Format:     FormatJSON,
				Project:    "bench",
				MaxEntries: n + 100,
			})
			if err != nil {
				b.Fatal(err)
			}

			for i := 0; i < n; i++ {
				opts := RememberOptions{
					Category: CategoryDecision,
					Key:      fmt.Sprintf("key-%d", i),
					Content:  fmt.Sprintf("Decision %d: implementar modulo de autenticacion", i),
					Tags:     []string{"auth", "security"},
				}
				if i > 0 {
					opts.Links = []string{fmt.Sprintf("key-%d", i-1)}
				}
				s.Remember(opts)
			}

			capsules := s.ListActive(0)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				BuildGraph(capsules)
			}
			b.ReportAllocs()
		})
	}
}

func BenchmarkCentrality(b *testing.B) {
	dir := b.TempDir()
	s, err := NewStore(Config{
		Dir:        dir,
		Format:     FormatJSON,
		Project:    "bench",
		MaxEntries: 500,
	})
	if err != nil {
		b.Fatal(err)
	}

	for i := 0; i < 200; i++ {
		opts := RememberOptions{
			Category: CategoryDecision,
			Key:      fmt.Sprintf("key-%d", i),
			Content:  fmt.Sprintf("Nodo %d del grafo de conocimiento", i),
			Tags:     []string{"graph"},
		}
		if i > 0 {
			opts.Links = []string{fmt.Sprintf("key-%d", i-1)}
		}
		s.Remember(opts)
	}

	g := BuildGraph(s.ListActive(0))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.Centrality()
	}
	b.ReportAllocs()
}
