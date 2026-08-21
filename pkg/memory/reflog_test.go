package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseReflogLine(t *testing.T) {
	line := "0000000000000000000000000000000000000000 1234567890abcdef1234567890abcdef1234567890 U 1700000000 +0000\tHEAD@{0}: commit: Fix bug\n"
	ev, ok := parseReflogLine(line)
	if !ok {
		t.Fatal("parse debería tener éxito")
	}
	if ev.Hash != "12345678" {
		t.Errorf("hash = %s, want 12345678", ev.Hash)
	}
	if ev.Subject != "HEAD@{0}: commit: Fix bug" {
		t.Errorf("subject = %q", ev.Subject)
	}
	if ev.TS == "" {
		t.Errorf("ts no debería estar vacío")
	}
}

func TestParseReflogLine_WithTimestamp(t *testing.T) {
	// El parser toma fields[3] como timestamp Unix.
	line := "0000000000000000000000000000000000000000 aaaa1111bbbb2222cccc3333dddd4444eeee5555 U 1700000000 +0000\tHEAD@{0}: commit: Con ts\n"
	ev, ok := parseReflogLine(line)
	if !ok {
		t.Fatal("parse debería tener éxito")
	}
	if ev.Hash != "aaaa1111" {
		t.Errorf("hash = %s", ev.Hash)
	}
	if ev.TS == "" {
		t.Error("ts debería poblarse")
	}
}

func TestParseReflogLine_Bad(t *testing.T) {
	bad := []string{
		"sin tab",
		"a\tb",
		"0000000000000000000000000000000000000000 12345 U 1700000000 +0000\tcorto", // new hash <8
		"nothex nothex nothex nothex\tcommit: x",
		"aaaaaaaa 1234567890abcdef1234567890abcdef1234567890\tcommit: x", // solo 2 campos meta
	}
	for _, l := range bad {
		if _, ok := parseReflogLine(l); ok {
			t.Errorf("parse(%q) debería fallar", l)
		}
	}
}

func TestGitDirOfAndCurrentHead(t *testing.T) {
	dir := t.TempDir()
	// Sin .git → devuelve <dir>/.git y CurrentHead vacío.
	if got := gitDirOf(dir); got != filepath.Join(dir, ".git") {
		t.Errorf("gitDirOf sin git = %q", got)
	}
	if h := CurrentHead(dir); h != "" {
		t.Errorf("CurrentHead sin git = %q, want vacío", h)
	}

	// Crear .git/HEAD apuntando a una rama.
	gitDir := filepath.Join(dir, ".git")
	os.MkdirAll(filepath.Join(gitDir, "refs", "heads"), 0755)
	hash := "abcdef1234567890abcdef1234567890abcdef12"
	os.WriteFile(filepath.Join(gitDir, "refs", "heads", "main"), []byte(hash), 0644)
	os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0644)
	if h := CurrentHead(dir); h != "abcdef12" {
		t.Errorf("CurrentHead = %q, want abcdef12", h)
	}

	// HEAD directo (detached).
	os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte(hash), 0644)
	if h := CurrentHead(dir); h != "abcdef12" {
		t.Errorf("CurrentHead detached = %q, want abcdef12", h)
	}

	// HEAD no hex.
	os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0644)
	os.WriteFile(filepath.Join(gitDir, "refs", "heads", "main"), []byte("nothex\n"), 0644)
	if h := CurrentHead(dir); h != "" {
		t.Errorf("CurrentHead nonhex = %q, want vacío", h)
	}
}

func TestGitDirOfWorktreeFile(t *testing.T) {
	dir := t.TempDir()
	gitFile := filepath.Join(dir, ".git")
	os.WriteFile(gitFile, []byte("gitdir: ../otro.git\n"), 0644)
	// gitDirOf devuelve el path relativo tal cual viene en el archivo gitdir.
	if got := gitDirOf(dir); got != "../otro.git" {
		t.Errorf("gitDirOf worktree = %q", got)
	}
}

func TestReflogNewFromHash(t *testing.T) {
	dir := t.TempDir()
	gitDir := filepath.Join(dir, ".git")
	logsDir := filepath.Join(gitDir, "logs")
	os.MkdirAll(logsDir, 0755)

	reflog := strings.Join([]string{
		"0000000000000000000000000000000000000000 1111111111111111111111111111111111111111 U 1700000000 +0000\tHEAD@{0}: commit: first",
		"1111111111111111111111111111111111111111 2222222222222222222222222222222222222222 U 1700000001 +0000\tHEAD@{1}: commit: second",
		"2222222222222222222222222222222222222222 3333333333333333333333333333333333333333 U 1700000002 +0000\tHEAD@{2}: commit: third",
	}, "\n") + "\n"
	os.WriteFile(filepath.Join(logsDir, "HEAD"), []byte(reflog), 0644)

	all := ReflogNew(dir, "")
	if len(all) != 3 {
		t.Fatalf("ReflogNew('') = %d, want 3", len(all))
	}
	after := ReflogNew(dir, "22222222")
	if len(after) != 1 || after[0].Hash != "33333333" {
		t.Fatalf("ReflogNew(from) = %v", after)
	}
	if !strings.Contains(after[0].Subject, "commit: third") {
		t.Errorf("subject = %q", after[0].Subject)
	}

	// Sin reflog → nil.
	emptyDir := t.TempDir()
	if got := ReflogNew(emptyDir, ""); got != nil {
		t.Errorf("ReflogNew sin reflog = %v, want nil", got)
	}
}

func TestReflogNew_FromHashNotFound(t *testing.T) {
	dir := t.TempDir()
	gitDir := filepath.Join(dir, ".git")
	os.MkdirAll(filepath.Join(gitDir, "logs"), 0755)
	reflog := "0000000000000000000000000000000000000000 1111111111111111111111111111111111111111 U 1700000000 +0000\tHEAD@{0}: commit: only\n"
	os.WriteFile(filepath.Join(gitDir, "logs", "HEAD"), []byte(reflog), 0644)

	// fromHash que no aparece: `seen` nunca se activa → sin resultados.
	if got := ReflogNew(dir, "ffffffff"); len(got) != 0 {
		t.Errorf("ReflogNew con fromHash inexistente = %v, want vacío", got)
	}
}
