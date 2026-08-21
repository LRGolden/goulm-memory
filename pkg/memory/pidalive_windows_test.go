//go:build windows

package memory

import (
	"os"
	"syscall"
	"testing"
)

func TestPidAlive(t *testing.T) {
	// El proceso actual siempre está vivo.
	if !pidAlive(os.Getpid()) {
		t.Error("pidAlive(pid actual) debería ser true")
	}
	// PID inexistente → false (la mayoría de procesos no existen).
	if pidAlive(99999999) {
		t.Log("pid 99999999 existe (raro); se omite la aserción")
	}
}

func TestIsSharingViolation(t *testing.T) {
	if !isSharingViolation(syscall.Errno(32)) {
		t.Error("Errno 32 (sharing violation) debería ser true")
	}
	if !isSharingViolation(syscall.Errno(5)) {
		t.Error("Errno 5 (access denied) debería ser true")
	}
	if isSharingViolation(syscall.Errno(2)) {
		t.Error("Errno 2 no debería ser sharing violation")
	}
	if isSharingViolation(errNoNotErrno{}) {
		t.Error("error que no es Errno debería ser false")
	}
}

// errNoNotErrno es un error que no implementa syscall.Errno.
type errNoNotErrno struct{}

func (errNoNotErrno) Error() string { return "no errno" }
