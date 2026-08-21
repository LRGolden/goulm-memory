package memory

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Vocabulario built-in: tags canónicas con sus keywords de coincidencia
// (substring, case-insensitive, sobre key + content).
var builtInVocab = map[string][]string{
	"auth":        {"auth", "login", "token", "jwt", "session", "oauth", "password"},
	"api":         {"api", "endpoint", "rest", "graphql", "rpc"},
	"db":          {"database", "sql", "postgres", "mysql", "sqlite", "mongo", "query", "schema"},
	"cache":       {"cache", "redis", "memcached", "in-memory"},
	"security":    {"security", "vulnerab", "cve", "sanitiz", "xss", "injection", "exploit"},
	"error":       {"error", "panic", "exception", "fail", "crash"},
	"logging":     {"log", "logger", "trace", "debug", "observability"},
	"config":      {"config", "env", "yaml", "toml", "settings"},
	"performance": {"performance", "slow", "latency", "benchmark", "optimiz", "throughput"},
	"testing":     {"test", "spec", "unit test", "integration test", "coverage"},
	"deploy":      {"deploy", "ci", "cd", "pipeline", "release", "build"},
	"refactor":    {"refactor", "cleanup", "restructure"},
	"types":       {"types", "struct", "schema", "interface", "validation"},
	"async":       {"async", "concurrency", "goroutine", "channel", "parallel", "thread"},
	"state":       {"state", "store", "stateful", "stateless", "mutation"},
	"ui":          {"ui", "component", "view", "render", "css", "html", "bubbletea"},
	"storage":     {"storage", "filesystem", "file system", "persistence", "disk"},
	"email":       {"email", "mail", "smtp", "notification"},
	"webhook":     {"webhook", "callback", "event"},
	"git":         {"git", "commit", "branch", "merge", "rebase", "pr", "pull request"},
	"docs":        {"docs", "documentation", "readme", "manual"},
}

// manifestREs detecta dependencias en manifests comunes.
var (
	goModRE          = regexp.MustCompile(`(?m)^\s*require\s*\(?.*$`)
	requirementsRE   = regexp.MustCompile(`(?m)^\s*([a-zA-Z0-9_\-\.]+)\s*[=<>!~]?`)
	packageJSONDepRE = regexp.MustCompile(`"([@a-zA-Z0-9_\-\./]+)"\s*:\s*"[^"]+"`)
)

// ExtractProjectDeps detecta dependencias del proyecto leyendo manifests
// comunes en el directorio dado: go.mod, requirements.txt, package.json.
// Devuelve un mapa tag → keywords para inferencia de tags.
func ExtractProjectDeps(dir string) map[string][]string {
	out := make(map[string][]string)

	// go.mod: el último segmento del path del módulo es el tag.
	if data, err := os.ReadFile(filepath.Join(dir, "go.mod")); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || line == "require (" || line == ")" || strings.HasPrefix(line, "//") {
				continue
			}
			if strings.HasPrefix(line, "require ") {
				line = strings.TrimPrefix(line, "require ")
			}
			if strings.HasPrefix(line, "module ") || strings.HasPrefix(line, "go ") {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) == 0 {
				continue
			}
			modPath := strings.Trim(fields[0], `"'`)
			tag := pathBase(modPath)
			if tag != "" && !isCommonWord(tag) {
				out[tag] = []string{tag, modPath}
			}
		}
	}

	// requirements.txt / pyproject: nombres antes de la especificación.
	if data, err := os.ReadFile(filepath.Join(dir, "requirements.txt")); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			m := requirementsRE.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			name := strings.TrimSpace(m[1])
			if name != "" {
				out[name] = []string{name}
			}
		}
	}

	// package.json: dependencias y devDependencies.
	if data, err := os.ReadFile(filepath.Join(dir, "package.json")); err == nil {
		for _, m := range packageJSONDepRE.FindAllStringSubmatch(string(data), -1) {
			name := m[1]
			tag := name
			if i := strings.LastIndex(name, "/"); i >= 0 && strings.HasPrefix(name, "@") {
				tag = name[i+1:] // scoped: @org/pkg → pkg
			}
			if tag != "" && !isCommonWord(tag) {
				out[tag] = []string{tag, name}
			}
		}
	}
	return out
}

// pathBase extrae el último segmento de un path de módulo.
func pathBase(modPath string) string {
	modPath = strings.Trim(modPath, "/")
	if i := strings.LastIndex(modPath, "/"); i >= 0 {
		return modPath[i+1:]
	}
	return modPath
}

var commonWords = map[string]bool{
	"go": true, "golang": true, "github": true, "gopkg": true,
	"com": true, "org": true, "io": true, "x": true, "v2": true, "v3": true,
}

func isCommonWord(s string) bool { return commonWords[s] }

// InferTags detecta tags del vocabulario (built-in + proyecto) presentes como
// palabra (con límites, no substring) en key + content. Devuelve todos los
// que coinciden (unión).
func InferTags(content, key string, projectVocab map[string][]string) []string {
	haystack := strings.ToLower(key + " " + content)
	found := make(map[string]bool)

	check := func(tag string, keywords []string) {
		if found[tag] {
			return
		}
		for _, kw := range keywords {
			if matchKeyword(haystack, kw) {
				found[tag] = true
				return
			}
		}
	}
	for tag, kws := range builtInVocab {
		check(tag, kws)
	}
	for tag, kws := range projectVocab {
		check(tag, kws)
	}

	out := make([]string, 0, len(found))
	for t := range found {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}
