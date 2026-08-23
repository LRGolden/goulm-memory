package memory

import (
	"fmt"
	"testing"
)

func BenchmarkRemember(b *testing.B) {
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

			// Pre-fill with n capsules.
			for i := 0; i < n; i++ {
				s.Remember(RememberOptions{
					Category: CategoryDecision,
					Key:      fmt.Sprintf("key-%d", i),
					Content:  fmt.Sprintf("Contenido de prueba para capsula %d con suficiente texto para BM25", i),
					Tags:     []string{"test", "bench"},
				})
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				s.Remember(RememberOptions{
					Category: CategoryDecision,
					Key:      fmt.Sprintf("bench-%d", i),
					Content:  "Contenido de benchmark para medir latencia de escritura",
					Tags:     []string{"bench"},
				})
			}
			b.ReportAllocs()
		})
	}
}

func BenchmarkRecall(b *testing.B) {
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

			// Pre-fill with n capsules.
			for i := 0; i < n; i++ {
				s.Remember(RememberOptions{
					Category: CategoryDecision,
					Key:      fmt.Sprintf("key-%d", i),
					Content:  fmt.Sprintf("Contenido de prueba para capsula %d con suficiente texto para BM25 y ranking", i),
					Tags:     []string{"test", "bench"},
				})
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				s.Recall("test", nil)
			}
			b.ReportAllocs()
		})
	}
}

func BenchmarkSmartRecall(b *testing.B) {
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

			// Pre-fill with linked capsules.
			for i := 0; i < n; i++ {
				opts := RememberOptions{
					Category: CategoryDecision,
					Key:      fmt.Sprintf("key-%d", i),
					Content:  fmt.Sprintf("Decision tecnica %d: usar patron observador para eventos", i),
					Tags:     []string{"test", "pattern"},
				}
				if i > 0 {
					opts.Links = []string{fmt.Sprintf("supersedes:key-%d", i-1)}
				}
				s.Remember(opts)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				s.SmartRecall("patron observador", 5)
			}
			b.ReportAllocs()
		})
	}
}

func BenchmarkForget(b *testing.B) {
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

			// Pre-fill.
			for i := 0; i < n; i++ {
				s.Remember(RememberOptions{
					Category: CategoryDecision,
					Key:      fmt.Sprintf("key-%d", i),
					Content:  fmt.Sprintf("Contenido %d", i),
				})
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				s.Forget(fmt.Sprintf("key-%d", i%n), false)
			}
			b.ReportAllocs()
		})
	}
}
