//go:build !windows

package memory

import (
	"os"
	"syscall"
)

// pidAlive comprueba si el proceso existe con signal 0.
func pidAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

// isSharingViolation: en Unix el modo exclusivo ya devuelve EEXIST, no hay
// condición equivalente a la violación de compartición de Windows.
func isSharingViolation(err error) bool { return false }
