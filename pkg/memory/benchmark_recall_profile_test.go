package memory

import (
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"testing"
)

// BenchmarkRecallProfile genera un memprofile.out para análisis con:
//
//	go tool pprof -alloc_objects -inuse_space memprofile.out
//
// Esto permite verificar que BuildGraph/sharedCount son las causas
// principales de las 214MB en Recall N=1000.
func BenchmarkRecallProfile(b *testing.B) {
	for _, n := range []int{100, 500, 1000} {
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

			// Pre-fill con tags genéricos (reproduce el problema O(N²)).
			for i := 0; i < n; i++ {
				s.Remember(RememberOptions{
					Category: CategoryDecision,
					Key:      fmt.Sprintf("key-%d", i),
					Content:  fmt.Sprintf("Contenido de prueba para capsula %d con suficiente texto para BM25 y ranking", i),
					Tags:     []string{"test", "bench"},
				})
			}

			// Forzar build del grafo (primer Recall lo construye).
			s.Recall("test", nil)

			// Guardar memprofile después del primer Recall.
			if n == 1000 {
				runtime.GC()
				f, err := os.Create(dir + "/memprofile.out")
				if err == nil {
					pprof.WriteHeapProfile(f)
					f.Close()
					b.Logf("memprofile guardado en %s/memprofile.out", dir)
				}
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				s.Recall("test", nil)
			}
			b.ReportAllocs()
		})
	}
}
