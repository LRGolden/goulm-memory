package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHasGitDir(t *testing.T) {
	dir := t.TempDir()
	if HasGitDir(dir) {
		t.Error("sin .git debería ser false")
	}
	os.Mkdir(filepath.Join(dir, ".git"), 0700)
	if !HasGitDir(dir) {
		t.Error("con .git debería ser true")
	}
}

func TestCurrentBranchWorktree(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real.git")
	os.MkdirAll(real, 0700)
	os.WriteFile(filepath.Join(real, "HEAD"), []byte("ref: refs/heads/wt-branch\n"), 0600)
	// Worktree: .git es un archivo con la ruta absoluta.
	gitFile := filepath.Join(dir, ".git")
	os.WriteFile(gitFile, []byte("gitdir: "+filepath.ToSlash(real)+"\n"), 0600)
	if got := CurrentBranch(dir); got != "wt-branch" {
		t.Errorf("CurrentBranch worktree = %q", got)
	}
}

func TestSanitizeCoverage(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Mi Proyecto", "mi-proyecto"},
		{"proyecto---final", "proyecto-final"},
		{"Node.js", "node-js"},
		{"  espacios  ", "espacios"},
		{"___", "proyecto"},
		{"normal-name", "normal-name"},
		{"AcentoÑ", "acento"},
	}
	for _, c := range cases {
		if got := sanitize(c.in); got != c.want {
			t.Errorf("sanitize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestShortHash(t *testing.T) {
	h := shortHash("hola")
	if len(h) != 12 {
		t.Errorf("shortHash len = %d, want 12", len(h))
	}
	if h != shortHash("hola") {
		t.Error("shortHash debería ser estable")
	}
	if h == shortHash("adios") {
		t.Error("distintos inputs no deberían colisionar")
	}
}

func TestProjectIDSanitizes(t *testing.T) {
	dir := t.TempDir()
	// Crear carpeta con nombre a sanitizar.
	base := filepath.Join(filepath.Dir(dir), "Mi Proyecto Final")
	os.MkdirAll(base, 0700)
	defer os.RemoveAll(base)
	id := ProjectID(base)
	if !strings.Contains(id, "mi-proyecto-final-") {
		t.Errorf("ProjectID = %q", id)
	}
}

func TestProjectIDRootPath(t *testing.T) {
	// name == separator → "default".
	id := ProjectID(string(filepath.Separator))
	if !strings.Contains(id, "default-") {
		t.Errorf("ProjectID raíz = %q", id)
	}
}
