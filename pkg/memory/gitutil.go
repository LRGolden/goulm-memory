package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// CurrentBranch devuelve la rama git actual sin ejecutar ningún comando:
// lee .git/HEAD directamente, soportando worktrees (.git como archivo
// "gitdir: <ruta>") y HEAD detached (hash corto).
func CurrentBranch(repoDir string) string {
	headPath := filepath.Join(repoDir, ".git", "HEAD")
	gitDir := filepath.Join(repoDir, ".git")

	// Worktree: .git es un archivo con la ruta real.
	if fi, err := os.Stat(gitDir); err == nil && !fi.IsDir() {
		data, err := os.ReadFile(gitDir)
		if err == nil {
			line := strings.TrimSpace(string(data))
			if strings.HasPrefix(line, "gitdir:") {
				gitDir = strings.TrimSpace(strings.TrimPrefix(line, "gitdir:"))
				headPath = filepath.Join(gitDir, "HEAD")
			}
		}
	}

	data, err := os.ReadFile(headPath)
	if err != nil {
		return ""
	}
	ref := strings.TrimSpace(string(data))
	if strings.HasPrefix(ref, "ref: refs/heads/") {
		return strings.TrimPrefix(ref, "ref: refs/heads/")
	}
	// HEAD detached: hash corto.
	if len(ref) >= 8 && isHex(ref) {
		return ref[:8]
	}
	return ""
}

var hexRE = regexp.MustCompile(`^[0-9a-fA-F]+$`)

func isHex(s string) bool { return hexRE.MatchString(s) }

// HasGitDir indica si el directorio es un repo git.
func HasGitDir(repoDir string) bool {
	_, err := os.Stat(filepath.Join(repoDir, ".git"))
	return err == nil
}

// ProjectID deriva el identificador estable de un proyecto a partir del cwd:
// basename + hash corto (SHA-256, 6 bytes) del path absoluto. Así, dos
// carpetas con el mismo nombre no comparten memoria.
func ProjectID(cwd string) string {
	abs, err := filepath.Abs(cwd)
	if err != nil {
		abs = cwd
	}
	name := filepath.Base(abs)
	if name == "." || name == string(filepath.Separator) || name == "" {
		name = "default"
	}
	return fmt.Sprintf("%s-%s", sanitize(name), shortHash(abs))
}

func sanitize(name string) string {
	var sb strings.Builder
	prevDash := false
	write := func(r rune) {
		if r == '-' {
			if prevDash {
				return // colapsar guiones consecutivos
			}
			prevDash = true
		} else {
			prevDash = false
		}
		sb.WriteRune(r)
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			write(r)
		case r >= 'A' && r <= 'Z':
			write(r + 32) // lowercase
		case r == '-' || r == '_' || r == '.':
			write('-')
		default:
			write('-')
		}
	}
	out := strings.Trim(sb.String(), "-")
	if out == "" {
		return "proyecto"
	}
	return out
}

// shortHash genera un hash corto estable (12 hex chars) de un string.
func shortHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:6])
}
