package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// SessionTTL define cuándo una sesión se considera inactiva (10 min).
const (
	SessionTTL        = 10 * time.Minute
	maxHeartbeatFiles = 200 // tope de archivos rastreados por sesión
)

// Heartbeat es el estado que cada sesión escribe en su propio archivo.
type Heartbeat struct {
	ID        string            `json:"id"`
	Agent     string            `json:"agent"`
	PID       int               `json:"pid"`
	Branch    string            `json:"branch"`
	StartedAt string            `json:"started_at"`
	LastSeen  string            `json:"last_seen"`
	Ended     bool              `json:"ended"`
	Files     map[string]string `json:"files"` // path → ISO de último toque
}

// ActiveSession es la vista pública de una sesión activa.
type ActiveSession struct {
	ID       string
	Agent    string
	Branch   string
	IsSelf   bool
	LastSeen time.Time
	Files    []string
	Conflict bool
}

// FileConflict es un archivo tocado por ≥2 sesiones activas.
type FileConflict struct {
	File     string
	Sessions []string
}

// SessionTracker gestiona los heartbeats de sesión en sessions/<id>.json.
// Cada proceso escribe solo su propio archivo → sin locks distribuidos.
type SessionTracker struct {
	dir     string
	root    string // raíz del proyecto para CurrentBranch ("" = cwd)
	selfID  string
	selfPID int
	agent   string
	branch  string
}

// NewSessionTracker crea el tracker. selfID se resuelve desde GOULM_SESSION_ID
// o se genera como proc-<pid>-<aleatorio>. agent es el nombre del agente.
func NewSessionTracker(dir, agent string) (*SessionTracker, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	id := os.Getenv("GOULM_SESSION_ID")
	if id == "" {
		id = fmt.Sprintf("proc-%d-%s", os.Getpid(), NewID())
	}
	return &SessionTracker{
		dir:     dir,
		selfID:  id,
		selfPID: os.Getpid(),
		agent:   agent,
		branch:  "",
	}, nil
}

// SetRoot fija la raíz del proyecto para resolver la rama git. Si no se fija,
// se usa el cwd del proceso.
func (t *SessionTracker) SetRoot(root string) { t.root = root }

// SelfID devuelve el ID de la sesión actual.
func (t *SessionTracker) SelfID() string { return t.selfID }

// Heartbeat escribe/actualiza el propio heartbeat. Con file != "" registra un
// archivo tocado; con ended marca el cierre de la sesión.
func (t *SessionTracker) Heartbeat(file string, ended bool) error {
	root := t.root
	if root == "" {
		root = "."
	}
	t.branch = CurrentBranch(root)
	hb := Heartbeat{
		ID:        t.selfID,
		Agent:     t.agent,
		PID:       t.selfPID,
		Branch:    t.branch,
		StartedAt: nowISO(),
		LastSeen:  nowISO(),
		Ended:     ended,
		Files:     make(map[string]string),
	}
	// Conservar archivos previos del propio heartbeat.
	if data, err := os.ReadFile(t.path(t.selfID)); err == nil {
		var prev Heartbeat
		if json.Unmarshal(data, &prev) == nil {
			hb.StartedAt = prev.StartedAt
			if prev.Files != nil {
				hb.Files = prev.Files
			}
			if ended {
				hb.Files = prev.Files
			}
		}
	}
	if file != "" {
		hb.Files[filepath.ToSlash(file)] = nowISO()
	}
	if ended {
		hb.Ended = true
	}
	pruneHeartbeatFiles(hb.Files)
	data, err := json.MarshalIndent(hb, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(t.path(t.selfID), data, 0600)
}

// pruneHeartbeatFiles limita el mapa de archivos a los maxHeartbeatFiles más
// recientes (las fechas RFC3339 ordenan lexicográficamente).
func pruneHeartbeatFiles(files map[string]string) {
	if len(files) <= maxHeartbeatFiles {
		return
	}
	type kv struct {
		path string
		ts   string
	}
	all := make([]kv, 0, len(files))
	for p, ts := range files {
		all = append(all, kv{p, ts})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ts > all[j].ts })
	for _, item := range all[maxHeartbeatFiles:] {
		delete(files, item.path)
	}
}

// Touch registra un archivo tocado (heartbeat implícito).
func (t *SessionTracker) Touch(file string) error {
	if file == "" {
		return nil
	}
	return t.Heartbeat(file, false)
}

// End marca el fin de sesión.
func (t *SessionTracker) End() error {
	return t.Heartbeat("", true)
}

func (t *SessionTracker) path(id string) string {
	return filepath.Join(t.dir, id+".json")
}

// ActiveSessions lista las sesiones activas (last_seen ≤ TTL, ended=false)
// ordenadas por actividad, marcando la propia.
func (t *SessionTracker) ActiveSessions() ([]ActiveSession, error) {
	now := time.Now()
	entries, err := os.ReadDir(t.dir)
	if err != nil {
		return nil, err
	}
	var out []ActiveSession
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(t.dir, e.Name()))
		if err != nil {
			continue
		}
		var hb Heartbeat
		if json.Unmarshal(data, &hb) != nil || hb.ID == "" {
			continue
		}
		if hb.Ended {
			continue
		}
		lastSeen, err := time.Parse(time.RFC3339, hb.LastSeen)
		if err != nil || now.Sub(lastSeen) > SessionTTL {
			continue
		}
		files := make([]string, 0, len(hb.Files))
		for f := range hb.Files {
			files = append(files, f)
		}
		sort.Strings(files)
		out = append(out, ActiveSession{
			ID:       hb.ID,
			Agent:    hb.Agent,
			Branch:   hb.Branch,
			IsSelf:   hb.ID == t.selfID,
			LastSeen: lastSeen,
			Files:    files,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].LastSeen.After(out[j].LastSeen)
	})
	return out, nil
}

// Conflicts detecta archivos tocados por ≥2 sesiones activas.
func (t *SessionTracker) Conflicts() ([]FileConflict, error) {
	sessions, err := t.ActiveSessions()
	if err != nil {
		return nil, err
	}
	byFile := make(map[string][]string)
	for _, s := range sessions {
		for _, f := range s.Files {
			byFile[f] = append(byFile[f], s.ID)
		}
	}
	var out []FileConflict
	for f, ids := range byFile {
		if len(ids) >= 2 {
			sort.Strings(ids)
			out = append(out, FileConflict{File: f, Sessions: ids})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].File < out[j].File })
	return out, nil
}

// Prune elimina heartbeats obsoletos: sin señales > TTL y PID muerto.
// Devuelve el número de archivos borrados.
func (t *SessionTracker) Prune() (int, error) {
	now := time.Now()
	entries, err := os.ReadDir(t.dir)
	if err != nil {
		return 0, err
	}
	removed := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(t.dir, e.Name()))
		if err != nil {
			continue
		}
		var hb Heartbeat
		if json.Unmarshal(data, &hb) != nil || hb.PID <= 0 {
			continue
		}
		lastSeen, err := time.Parse(time.RFC3339, hb.LastSeen)
		if err != nil {
			continue
		}
		if now.Sub(lastSeen) > SessionTTL && !pidAlive(hb.PID) {
			if os.Remove(filepath.Join(t.dir, e.Name())) == nil {
				removed++
			}
		}
	}
	return removed, nil
}

// SessionFiles devuelve el mapa path → true de archivos tocados por la
// sesión actual (para el sesgo de sesión en el ranking).
func (t *SessionTracker) SessionFiles() map[string]bool {
	data, err := os.ReadFile(t.path(t.selfID))
	if err != nil {
		return nil
	}
	var hb Heartbeat
	if json.Unmarshal(data, &hb) != nil {
		return nil
	}
	out := make(map[string]bool, len(hb.Files))
	for f := range hb.Files {
		out[f] = true
	}
	return out
}

// RenderSessions formatea la vista de coordinación para el agente.
func RenderSessions(sessions []ActiveSession, conflicts []FileConflict, conflictsOnly bool) string {
	var sb strings.Builder
	if conflictsOnly {
		if len(conflicts) == 0 {
			return "Sin conflictos suaves."
		}
		sb.WriteString(fmt.Sprintf("🔥 Conflictos suaves (%d):\n", len(conflicts)))
		for _, c := range conflicts {
			sb.WriteString(fmt.Sprintf("  ⚠️ %s\n     ↔ %s\n", c.File, strings.Join(c.Sessions, ", ")))
		}
		return strings.TrimRight(sb.String(), "\n")
	}
	sb.WriteString(fmt.Sprintf("🧭 Sesiones activas (%d) — ventana %d min:\n", len(sessions), int(SessionTTL.Minutes())))
	for _, s := range sessions {
		marker := ""
		if s.IsSelf {
			marker = " (tú)"
		}
		sb.WriteString(fmt.Sprintf("  • %s @ %s%s\n", s.Agent, s.Branch, marker))
		sb.WriteString(fmt.Sprintf("    hace %d min\n", int(time.Since(s.LastSeen).Minutes())))
		for _, f := range s.Files {
			sb.WriteString("      • " + f + "\n")
		}
	}
	if len(conflicts) > 0 {
		sb.WriteString(fmt.Sprintf("🔥 Conflictos suaves (%d):\n", len(conflicts)))
		for _, c := range conflicts {
			sb.WriteString(fmt.Sprintf("  ⚠️ %s ↔ %s\n", c.File, strings.Join(c.Sessions, ", ")))
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}
