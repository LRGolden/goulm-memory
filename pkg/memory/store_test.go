package memory

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"
)

func TestLockFileAcquireRelease(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "memory.lock")
	release, _, err := lockFile(lockPath)
	if err != nil {
		t.Fatalf("lock: %v", err)
	}
	release()

	// Re-adquirir tras liberar debe funcionar.
	release2, _, err := lockFile(lockPath)
	if err != nil {
		t.Fatalf("re-lock: %v", err)
	}
	release2()
}

func TestLockFileContention(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "memory.lock")
	release, _, err := lockFile(lockPath)
	if err != nil {
		t.Fatal(err)
	}

	// Segundo lock debe esperar y fallar con timeout corto... o simplemente
	// bloquearse hasta que el primero libere. Usamos goroutine + liberación.
	done := make(chan error, 1)
	go func() {
		rel, _, err := lockFile(lockPath)
		if err != nil {
			done <- err
			return
		}
		rel()
		done <- nil
	}()
	release()
	if err := <-done; err != nil {
		t.Fatalf("lock en contención: %v", err)
	}
}

func TestAtomicWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.txt")
	if err := atomicWrite(path, []byte("hola"), 0600); err != nil {
		t.Fatal(err)
	}
	release, _, err := lockFile(path + ".lock")
	if err != nil {
		t.Fatal(err)
	}
	release()
}

// TestConcurrentWrites ejercita el almacén con varias goroutines para
// detectar condiciones de carrera (race detector en CI).
func TestConcurrentWrites(t *testing.T) {
	s := newTestStore(t, FormatJSON)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_, err := s.Remember(RememberOptions{
				Category: CategoryKnowledge,
				Key:      fmt.Sprintf("k-%d", n%5),
				Content:  fmt.Sprintf("Contenido %d", n),
			})
			if err != nil {
				t.Errorf("write: %v", err)
			}
		}(i)
	}
	wg.Wait()
	if len(s.ListActive(0)) == 0 {
		t.Fatal("debería haber cápsulas tras las escrituras concurrentes")
	}
}
