package memory

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInferTagsBuiltIn(t *testing.T) {
	tags := InferTags("Redis connection timeout — aumentamos el pool", "redis-timeout", nil)
	if !containsStr(tags, "cache") {
		t.Errorf("tags = %v, esperaba cache", tags)
	}
	tags = InferTags("login con JWT y OAuth para la API", "auth-flow", nil)
	if !containsStr(tags, "auth") || !containsStr(tags, "api") {
		t.Errorf("tags = %v, esperaba auth y api", tags)
	}
}

func TestInferTagsProjectVocab(t *testing.T) {
	vocab := map[string][]string{"cobra": {"cobra", "github.com/spf13/cobra"}}
	tags := InferTags("los comandos usan cobra para el CLI", "cli", vocab)
	if !containsStr(tags, "cobra") {
		t.Errorf("tags = %v, esperaba cobra del vocabulario del proyecto", tags)
	}
}

func TestExtractProjectDepsGoMod(t *testing.T) {
	dir := t.TempDir()
	gomod := `module github.com/LRGolden/goulm

go 1.26.1

require (
	github.com/spf13/cobra v1.10.2
	github.com/spf13/viper v1.21.0
	golang.org/x/crypto v0.49.0
)

require github.com/atotto/clipboard v0.1.4 // indirect
`
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0600)
	vocab := ExtractProjectDeps(dir)
	if vocab["cobra"] == nil {
		t.Errorf("vocab = %v, esperaba cobra", vocab)
	}
	if vocab["viper"] == nil {
		t.Errorf("vocab = %v, esperaba viper", vocab)
	}
	// x/crypto → crypto (no common word).
	if vocab["crypto"] == nil {
		t.Errorf("vocab = %v, esperaba crypto", vocab)
	}
}

func TestExtractProjectDepsRequirements(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte("requests==2.31.0\nflask>=3.0\n"), 0600)
	vocab := ExtractProjectDeps(dir)
	if vocab["requests"] == nil || vocab["flask"] == nil {
		t.Errorf("vocab = %v, esperaba requests y flask", vocab)
	}
}

func TestExtractProjectDepsPackageJSON(t *testing.T) {
	dir := t.TempDir()
	pj := `{"dependencies": {"@charmbracelet/bubbletea": "^1.0.0", "zod": "^3.0.0"}}`
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(pj), 0600)
	vocab := ExtractProjectDeps(dir)
	if vocab["bubbletea"] == nil {
		t.Errorf("scoped dep → %v, esperaba bubbletea", vocab)
	}
	if vocab["zod"] == nil {
		t.Errorf("vocab = %v, esperaba zod", vocab)
	}
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func TestSanitize(t *testing.T) {
	if got := sanitize("Mi Proyecto! (v2)"); got != "mi-proyecto-v2" {
		t.Errorf("sanitize = %q", got)
	}
	if got := sanitize("ñandú"); got == "" {
		t.Error("caracteres no ASCII → -")
	}
}
