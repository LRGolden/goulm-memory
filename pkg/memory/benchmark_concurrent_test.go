package memory

import (
	"fmt"
	"sync"
	"testing"
)

func BenchmarkConcurrentRemember(b *testing.B) {
	for _, workers := range []int{1, 2, 4, 8} {
		b.Run(fmt.Sprintf("workers=%d", workers), func(b *testing.B) {
			dir := b.TempDir()
			s, err := NewStore(Config{
				Dir:        dir,
				Format:     FormatJSON,
				Project:    "bench",
				MaxEntries: 10000,
			})
			if err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()
			var wg sync.WaitGroup
			for i := 0; i < workers; i++ {
				wg.Add(1)
				go func(worker int) {
					defer wg.Done()
					for j := 0; j < b.N; j++ {
						s.Remember(RememberOptions{
							Category: CategoryDecision,
							Key:      fmt.Sprintf("worker-%d-item-%d", worker, j),
							Content:  fmt.Sprintf("Contenido escrito por worker %d, item %d", worker, j),
						})
					}
				}(i)
			}
			wg.Wait()
			b.ReportAllocs()
		})
	}
}

func BenchmarkConcurrentRecall(b *testing.B) {
	dir := b.TempDir()
	s, err := NewStore(Config{
		Dir:        dir,
		Format:     FormatJSON,
		Project:    "bench",
		MaxEntries: 1000,
	})
	if err != nil {
		b.Fatal(err)
	}

	// Pre-fill.
	for i := 0; i < 500; i++ {
		s.Remember(RememberOptions{
			Category: CategoryDecision,
			Key:      fmt.Sprintf("key-%d", i),
			Content:  fmt.Sprintf("Contenido de prueba %d para busqueda concurrente", i),
			Tags:     []string{"test"},
		})
	}

	b.ResetTimer()
	for _, workers := range []int{1, 2, 4, 8} {
		b.Run(fmt.Sprintf("workers=%d", workers), func(b *testing.B) {
			var wg sync.WaitGroup
			for i := 0; i < workers; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for j := 0; j < b.N; j++ {
						s.Recall("test", nil)
					}
				}()
			}
			wg.Wait()
			b.ReportAllocs()
		})
	}
}

func BenchmarkMixedWorkload(b *testing.B) {
	dir := b.TempDir()
	s, err := NewStore(Config{
		Dir:        dir,
		Format:     FormatJSON,
		Project:    "bench",
		MaxEntries: 1000,
	})
	if err != nil {
		b.Fatal(err)
	}

	// Pre-fill.
	for i := 0; i < 200; i++ {
		s.Remember(RememberOptions{
			Category: CategoryDecision,
			Key:      fmt.Sprintf("key-%d", i),
			Content:  fmt.Sprintf("Contenido %d para workload mixto", i),
			Tags:     []string{"mixed"},
		})
	}

	b.ResetTimer()
	var wg sync.WaitGroup

	// Writers (25%).
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < b.N/4+1; i++ {
			s.Remember(RememberOptions{
				Category: CategoryDecision,
				Key:      fmt.Sprintf("new-%d", i),
				Content:  "Nuevo item en workload mixto",
			})
		}
	}()

	// Readers (75%).
	for w := 0; w < 3; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < b.N/4+1; i++ {
				s.Recall("mixed", nil)
			}
		}()
	}

	wg.Wait()
	b.ReportAllocs()
}
