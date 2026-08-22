package memory

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const (
	DefaultLedgerWindow    = 200
	defaultMaxDetail       = 300
	defaultMaxRootDepth    = 10
	defaultCompactSizeHint = 48 * 1024
)

var ErrLedgerDisabled = errors.New("ledger deshabilitado")

// EventVersion es la versión del esquema de eventos (v2 = activity log estándar).
const EventVersion = 2

// Status de ejecución de un evento (activity log estándar).
const (
	StatusOK      = "ok"
	StatusError   = "error"
	StatusBlocked = "blocked"
	StatusDenied  = "denied"
)

// Approved refleja la decisión de aprobación de una tool.
const (
	ApprovedYes = "yes"
	ApprovedNo  = "no"
	ApprovedNA  = "na"
)

type LedgerEvent struct {
	V          int     `json:"v"`
	ID         string  `json:"id,omitempty"`
	TS         string  `json:"ts"`
	Type       string  `json:"type"`
	Action     string  `json:"action,omitempty"`
	Path       string  `json:"path,omitempty"`
	Detail     string  `json:"detail,omitempty"`
	Hash       string  `json:"hash,omitempty"`
	Status     string  `json:"status,omitempty"`
	DurationMs int64   `json:"duration_ms,omitempty"`
	Risk       string  `json:"risk,omitempty"`
	Tokens     int     `json:"tokens,omitempty"`
	CostUSD    float64 `json:"cost_usd,omitempty"`
	Turn       int     `json:"turn,omitempty"`
	Approved   string  `json:"approved,omitempty"`
	Session    string  `json:"session,omitempty"`
	Test       bool    `json:"test,omitempty"`
}

const (
	EventEdit      = "edit"
	EventCommit    = "commit"
	EventBranch    = "branch"    // reservado para futura integracion con git
	EventCheckout  = "checkout"  // reservado para futura integracion con git
	EventMemory    = "memory"
	EventSession   = "session"
	EventMilestone = "milestone"
	EventTest      = "test"
	EventError     = "error"
	EventTool      = "tool"
	EventApproval  = "approval"
	EventSystem    = "system"
)

type ledgerOptions struct {
	window   int
	home     string
	maxDepth int
}

type Option func(*ledgerOptions)

func WithWindow(n int) Option {
	return func(o *ledgerOptions) { o.window = n }
}

func WithHome(dir string) Option {
	return func(o *ledgerOptions) { o.home = dir }
}

func WithMaxDepth(d int) Option {
	return func(o *ledgerOptions) { o.maxDepth = d }
}

type Ledger struct {
	mu              sync.Mutex
	Dir             string
	Active          string
	Archives        string
	Lock            string
	Root            string
	Project         string
	Window          int
	Enabled         bool
	Reason          string
	compactSizeHint int64
}

type LedgerStats struct {
	Enabled      bool           `json:"enabled"`
	Reason       string         `json:"reason,omitempty"`
	Dir          string         `json:"dir,omitempty"`
	Project      string         `json:"project,omitempty"`
	Total        int            `json:"total"`
	ActiveLines  int            `json:"active_lines"`
	ArchiveFiles int            `json:"archive_files"`
	ArchiveLines int            `json:"archive_lines"`
	ByType       map[string]int `json:"by_type,omitempty"`
}

func NewLedger(cwd string, opts ...Option) (*Ledger, error) {
	o := ledgerOptions{window: DefaultLedgerWindow, maxDepth: defaultMaxRootDepth}
	for _, fn := range opts {
		fn(&o)
	}
	if strings.EqualFold(os.Getenv("GOULM_LEDGER"), "off") {
		return &Ledger{Enabled: false, Reason: "GOULM_LEDGER=off"}, nil
	}

	root := DetectRoot(cwd, o.maxDepth)
	project := ""
	if root != "" {
		project = filepath.Base(root)
		dir := filepath.Join(root, ".goulm")
		if writableDir(dir) {
			return newLedgerAt(dir, root, project, o.window)
		}
	}

	home := o.home
	if home == "" {
		hd, err := os.UserHomeDir()
		if err != nil {
			return &Ledger{Enabled: false, Reason: "sin directorio home: " + err.Error()}, nil
		}
		home = filepath.Join(hd, ".goulm", "ledger")
	}
	pid := ProjectID(cwd)
	if project == "" {
		project = pid
	}
	dir := filepath.Join(home, pid)
	if writableDir(dir) {
		return newLedgerAt(dir, "", project, o.window)
	}
	return &Ledger{Enabled: false, Reason: fmt.Sprintf("sin permisos de escritura en %s ni %s", filepath.Join(root, ".goulm"), dir)}, nil
}

func newLedgerAt(dir, root, project string, window int) (*Ledger, error) {
	if window <= 0 {
		window = DefaultLedgerWindow
	}
	archives := filepath.Join(dir, "archives")
	if err := os.MkdirAll(archives, 0700); err != nil {
		return nil, err
	}
	return &Ledger{
		Dir:             dir,
		Active:          filepath.Join(dir, "ledger.jsonl"),
		Archives:        archives,
		Lock:            filepath.Join(dir, "ledger.lock"),
		Root:            root,
		Project:         project,
		Window:          window,
		Enabled:         true,
		compactSizeHint: defaultCompactSizeHint,
	}, nil
}

func writableDir(dir string) bool {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return false
	}
	probe, err := os.CreateTemp(dir, ".probe-*")
	if err != nil {
		return false
	}
	name := probe.Name()
	probe.Close()
	os.Remove(name)
	return true
}

func DetectRoot(cwd string, maxDepth int) string {
	abs, err := filepath.Abs(cwd)
	if err != nil {
		abs = cwd
	}
	vol := filepath.VolumeName(abs)
	dir := filepath.Clean(abs)
	for depth := 0; depth < maxDepth; depth++ {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		if vol != "" && parent == vol+"\\" {
			break
		}
		if parent == vol+string(filepath.Separator) {
			break
		}
		dir = parent
	}
	return ""
}

func (l *Ledger) Append(ev LedgerEvent) error {
	if l == nil || !l.Enabled {
		return ErrLedgerDisabled
	}
	if ev.V == 0 {
		ev.V = EventVersion
	}
	if ev.ID == "" {
		ev.ID = NewID()
	}
	if ev.TS == "" {
		ev.TS = nowISO()
	}
	if ev.Status == "" {
		switch ev.Type {
		case EventError:
			ev.Status = StatusError
		default:
			ev.Status = StatusOK
		}
	}
	if ev.Approved == "" {
		ev.Approved = ApprovedNA
	}
	ev.Detail = sanitizeDetail(ev.Detail)
	ev.Path = normalizePath(ev.Path, l.Root)
	line, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := appendLine(l.Active, line); err != nil {
		return err
	}
	if fi, err := os.Stat(l.Active); err == nil && fi.Size() > l.compactSizeHint {
		l.compactLocked()
	}
	return nil
}

func appendLine(path string, line []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	_, werr := f.Write(append(line, '\n'))
	cerr := f.Close()
	if werr != nil {
		return werr
	}
	return cerr
}

func sanitizeDetail(s string) string {
	s = secretRE.ReplaceAllString(s, "***")
	r := []rune(s)
	if len(r) > defaultMaxDetail {
		return string(r[:defaultMaxDetail]) + "…"
	}
	return s
}

func normalizePath(p, root string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	p = strings.ReplaceAll(p, "\\", "/")
	for strings.HasPrefix(p, "./") {
		p = strings.TrimPrefix(p, "./")
	}
	if root != "" && filepathIsAbs(p) {
		rel, err := filepath.Rel(root, filepath.FromSlash(p))
		if err == nil {
			return strings.ReplaceAll(rel, "\\", "/")
		}
	}
	return p
}

func parseEvent(line string) (LedgerEvent, bool) {
	if line == "" {
		return LedgerEvent{}, false
	}
	var ev LedgerEvent
	if err := json.Unmarshal([]byte(line), &ev); err != nil {
		return LedgerEvent{}, false
	}
	if ev.V != 1 && ev.V != EventVersion {
		return LedgerEvent{}, false
	}
	if ev.TS == "" || ev.Type == "" {
		return LedgerEvent{}, false
	}
	return ev, true
}

func readEvents(path string) []LedgerEvent {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []LedgerEvent
	for _, line := range strings.Split(string(data), "\n") {
		if ev, ok := parseEvent(line); ok {
			out = append(out, ev)
		}
	}
	return out
}

func (l *Ledger) archivePaths() []string {
	entries, err := os.ReadDir(l.Archives)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "ledger.") && strings.HasSuffix(e.Name(), ".jsonl") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	out := make([]string, 0, len(names))
	for _, n := range names {
		out = append(out, filepath.Join(l.Archives, n))
	}
	return out
}

func (l *Ledger) allEvents() []LedgerEvent {
	var out []LedgerEvent
	for _, p := range l.archivePaths() {
		out = append(out, readEvents(p)...)
	}
	out = append(out, readEvents(l.Active)...)
	return out
}

func (l *Ledger) Tail(n int, typ string, includeHistory bool) []LedgerEvent {
	if l == nil || !l.Enabled {
		return nil
	}
	if n <= 0 {
		n = 20
	}
	var src []LedgerEvent
	if includeHistory {
		src = l.allEvents()
	} else {
		src = readEvents(l.Active)
	}
	out := make([]LedgerEvent, 0, n)
	for i := len(src) - 1; i >= 0 && len(out) < n; i-- {
		if typ != "" && src[i].Type != typ {
			continue
		}
		out = append(out, src[i])
	}
	return out
}

func (l *Ledger) AppendMilestone(msg, session string) error {
	return l.Append(LedgerEvent{Type: EventMilestone, Action: "mark", Detail: msg, Session: session})
}

func (l *Ledger) AppendSessionStart(session string) error {
	return l.Append(LedgerEvent{Type: EventSession, Action: "start", Session: session})
}

func (l *Ledger) AppendSessionEnd(session string) error {
	return l.Append(LedgerEvent{Type: EventSession, Action: "end", Session: session})
}

func (l *Ledger) AppendCommit(hash, subject, branch, session string) error {
	return l.Append(LedgerEvent{Type: EventCommit, Action: "commit", Hash: hash, Detail: subject, Session: session, Path: branch})
}

func (l *Ledger) AppendMemory(action, key, category, session string) error {
	return l.Append(LedgerEvent{Type: EventMemory, Action: action, Path: key, Detail: category, Session: session})
}

func (l *Ledger) AppendEdit(action, path, detail, session string, isTest bool) error {
	return l.Append(LedgerEvent{Type: EventEdit, Action: action, Path: path, Detail: detail, Session: session, Test: isTest})
}

func (l *Ledger) AppendTool(action, path, status, risk string, durationMs int64, session string, isTest bool) error {
	return l.Append(LedgerEvent{Type: EventTool, Action: action, Path: path, Status: status, Risk: risk, DurationMs: durationMs, Session: session, Test: isTest})
}

func (l *Ledger) AppendError(action, detail, session string, isTest bool) error {
	return l.Append(LedgerEvent{Type: EventError, Action: action, Detail: detail, Session: session, Test: isTest})
}

// AppendApproval registra una decisión de aprobación (allow/deny/always).
func (l *Ledger) AppendApproval(action string, approved string, session string, isTest bool) error {
	return l.Append(LedgerEvent{Type: EventApproval, Action: action, Approved: approved, Session: session, Test: isTest})
}

func (l *Ledger) Stats() LedgerStats {
	if l == nil || !l.Enabled {
		return LedgerStats{Enabled: false, Reason: l.Reason}
	}
	st := LedgerStats{Enabled: true, Dir: l.Dir, Project: l.Project, ByType: map[string]int{}}
	for _, ev := range l.allEvents() {
		st.Total++
		st.ByType[ev.Type]++
	}
	st.ActiveLines = len(readEvents(l.Active))
	for _, p := range l.archivePaths() {
		st.ArchiveFiles++
		st.ArchiveLines += len(readEvents(p))
	}
	return st
}

func (l *Ledger) Export(since, to string) (string, error) {
	if l == nil || !l.Enabled {
		return "", ErrLedgerDisabled
	}
	var sb strings.Builder
	for _, ev := range l.allEvents() {
		if len(ev.TS) < 10 {
			continue
		}
		if since != "" && ev.TS[:10] < since {
			continue
		}
		if to != "" && ev.TS[:10] > to {
			continue
		}
		line, err := json.Marshal(ev)
		if err != nil {
			continue
		}
		sb.Write(line)
		sb.WriteByte('\n')
	}
	return sb.String(), nil
}

// statusMark devuelve el símbolo de estado para la vista CLI.
func statusMark(status string) string {
	switch status {
	case StatusError:
		return "✗"
	case StatusBlocked:
		return "⛔"
	case StatusDenied:
		return "⛔"
	default:
		return "✓"
	}
}

// formatDuration devuelve la duración en ms (<1s) o segundos (>=1s).
func formatDuration(ms int64) string {
	if ms <= 0 {
		return ""
	}
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	return fmt.Sprintf("%.1fs", float64(ms)/1000)
}

// FormatEvent renderiza un evento en una línea legible (CLI tail).
// Layout: HH:MM:SS [type] [✓] action path (dur) — detail
func FormatEvent(ev LedgerEvent) string {
	return formatEvent(ev, false)
}

// FormatEventFull es como FormatEvent pero con la fecha completa (ISO).
func FormatEventFull(ev LedgerEvent) string {
	return formatEvent(ev, true)
}

func formatEvent(ev LedgerEvent, full bool) string {
	var sb strings.Builder
	ts := ev.TS
	if !full && len(ts) >= 16 {
		ts = ts[:16]
	}
	mark := statusMark(ev.Status)
	fmt.Fprintf(&sb, "%s [%s] [%s] %s", ts, ev.Type, mark, ev.Action)
	if ev.Path != "" {
		sb.WriteString(" " + ev.Path)
	}
	if d := formatDuration(ev.DurationMs); d != "" {
		sb.WriteString(" (" + d + ")")
	}
	if ev.Hash != "" {
		sb.WriteString(" (" + ev.Hash + ")")
	}
	if ev.Approved != "" && ev.Approved != ApprovedNA {
		sb.WriteString(" [" + ev.Approved + "]")
	}
	if ev.Detail != "" {
		sb.WriteString(" — " + ev.Detail)
	}
	if ev.Test {
		sb.WriteString(" [test]")
	}
	return sb.String()
}
