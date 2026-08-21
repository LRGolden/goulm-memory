//go:build windows

package memory

import "syscall"

// pidAlive comprueba si el proceso existe abriendo su handle con
// PROCESS_QUERY_INFORMATION (equivalente de "signal 0" en Unix).
func pidAlive(pid int) bool {
	h, err := syscall.OpenProcess(syscall.PROCESS_QUERY_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	syscall.CloseHandle(h)
	return true
}

// isSharingViolation detecta errores de compartición de Windows: al crear
// con O_EXCL un archivo que otro hilo está creando o eliminando a la vez,
// Windows devuelve ERROR_SHARING_VIOLATION (32) o ERROR_ACCESS_DENIED (5)
// en lugar de ERROR_FILE_EXISTS. Ambos se tratan como "el archivo existe"
// para reintentar el lock.
func isSharingViolation(err error) bool {
	if errno, ok := err.(syscall.Errno); ok {
		return errno == syscall.Errno(32) || errno == syscall.Errno(5)
	}
	return false
}
