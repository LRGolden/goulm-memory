package memory

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type ReflogEntry struct {
	Hash    string
	Subject string
	TS      string
}

func gitDirOf(repoDir string) string {
	gitDir := filepath.Join(repoDir, ".git")
	if fi, err := os.Stat(gitDir); err == nil && !fi.IsDir() {
		data, err := os.ReadFile(gitDir)
		if err == nil {
			line := strings.TrimSpace(string(data))
			if strings.HasPrefix(line, "gitdir:") {
				return strings.TrimSpace(strings.TrimPrefix(line, "gitdir:"))
			}
		}
	}
	return gitDir
}

// CurrentHead devuelve el hash corto del commit actual sin ejecutar git.
func CurrentHead(repoDir string) string {
	head, err := os.ReadFile(filepath.Join(gitDirOf(repoDir), "HEAD"))
	if err != nil {
		return ""
	}
	ref := strings.TrimSpace(string(head))
	if strings.HasPrefix(ref, "ref: ") {
		branch := strings.TrimPrefix(ref, "ref: ")
		data, err := os.ReadFile(filepath.Join(gitDirOf(repoDir), filepath.FromSlash(branch)))
		if err == nil {
			hash := strings.TrimSpace(string(data))
			if len(hash) >= 8 && isHex(hash) {
				return hash[:8]
			}
		}
		return ""
	}
	if len(ref) >= 8 && isHex(ref) {
		return ref[:8]
	}
	return ""
}

// ReflogNew devuelve las entradas del reflog posteriores a fromHash
// ("" = todas). El reflog es append-only: los commits nuevos están al final.
func ReflogNew(repoDir, fromHash string) []ReflogEntry {
	data, err := os.ReadFile(filepath.Join(gitDirOf(repoDir), "logs", "HEAD"))
	if err != nil {
		return nil
	}
	var out []ReflogEntry
	seen := false
	for _, line := range strings.Split(string(data), "\n") {
		ev, ok := parseReflogLine(line)
		if !ok {
			continue
		}
		if ev.Hash == fromHash {
			seen = true
			continue
		}
		if fromHash == "" || seen {
			out = append(out, ev)
		}
	}
	return out
}

func parseReflogLine(line string) (ReflogEntry, bool) {
	tab := strings.IndexByte(line, '\t')
	if tab < 0 {
		return ReflogEntry{}, false
	}
	meta, subject := line[:tab], line[tab+1:]
	fields := strings.Fields(meta)
	if len(fields) < 4 {
		return ReflogEntry{}, false
	}
	newHash := fields[1]
	if len(newHash) < 8 || !isHex(newHash) {
		return ReflogEntry{}, false
	}
	ts := ""
	if sec, err := strconv.ParseInt(fields[3], 10, 64); err == nil {
		ts = time.Unix(sec, 0).UTC().Format(time.RFC3339)
	}
	subject = strings.SplitN(subject, "\n", 2)[0]
	return ReflogEntry{Hash: newHash[:8], Subject: subject, TS: ts}, true
}
