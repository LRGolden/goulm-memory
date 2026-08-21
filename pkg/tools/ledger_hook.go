package tools

import (
	"encoding/json"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/LRGolden/goulm-memory/pkg/memory"
)

var (
	editTools = map[string]bool{
		"create_file": true, "edit_file": true, "file_delete": true, "file_rename": true,
	}
	memoryTools = map[string]bool{
		"memory_remember": true, "memory_archive": true, "memory_forget": true,
		"memory_pin": true, "memory_resolve": true, "memory_backup": true,
	}
	pathArgs = []string{"path", "file", "file_path", "name", "destination", "new_path", "old_path"}
	hashRE   = regexp.MustCompile(`\b[0-9a-f]{7,40}\b`)
)

type LedgerHook struct {
	ledger   *memory.Ledger
	ch       chan memory.LedgerEvent
	done     chan struct{}
	mu       sync.Mutex
	session  string
	lastHead string
	started  map[string]time.Time
	drops    int64
	writes   int64
	closeOnce sync.Once
}

func NewLedgerHook(l *memory.Ledger) *LedgerHook {
	h := &LedgerHook{
		ledger:  l,
		ch:      make(chan memory.LedgerEvent, 256),
		done:    make(chan struct{}),
		started: make(map[string]time.Time),
	}
	if l != nil && l.Enabled {
		go h.writer()
	}
	return h
}

func (h *LedgerHook) Ledger() *memory.Ledger { return h.ledger }

func (h *LedgerHook) writer() {
	for {
		select {
		case ev := <-h.ch:
			if err := h.ledger.Append(ev); err == nil {
				atomic.AddInt64(&h.writes, 1)
			}
		case <-h.done:
			for {
				select {
				case ev := <-h.ch:
					h.ledger.Append(ev)
				default:
					return
				}
			}
		}
	}
}

func (h *LedgerHook) enqueue(ev memory.LedgerEvent) {
	if h == nil || h.ledger == nil || !h.ledger.Enabled {
		return
	}
	select {
	case h.ch <- ev:
	default:
		atomic.AddInt64(&h.drops, 1)
	}
}

func (h *LedgerHook) StartSession(session string) {
	h.mu.Lock()
	h.session = session
	if h.ledger != nil && h.ledger.Root != "" {
		h.lastHead = memory.CurrentHead(h.ledger.Root)
	}
	h.mu.Unlock()
	h.enqueue(memory.LedgerEvent{Type: memory.EventSession, Action: "start", Session: session})
}

func (h *LedgerHook) EndSession() {
	h.mu.Lock()
	session := h.session
	root := ""
	if h.ledger != nil {
		root = h.ledger.Root
	}
	lastHead := h.lastHead
	h.mu.Unlock()
	if root != "" {
		for _, e := range memory.ReflogNew(root, lastHead) {
			h.enqueue(memory.LedgerEvent{Type: memory.EventCommit, Action: "commit", Hash: e.Hash, Detail: e.Subject, Session: session})
		}
	}
	h.enqueue(memory.LedgerEvent{Type: memory.EventSession, Action: "end", Session: session})
}

func (h *LedgerHook) Milestone(msg string) {
	h.mu.Lock()
	session := h.session
	h.mu.Unlock()
	h.enqueue(memory.LedgerEvent{Type: memory.EventMilestone, Action: "mark", Detail: msg, Session: session})
}

func (h *LedgerHook) OnToolStart(call *ToolCall) {
	if h == nil || call == nil || h.ledger == nil || !h.ledger.Enabled {
		return
	}
	h.mu.Lock()
	h.started[call.ID] = time.Now()
	h.mu.Unlock()
}

func (h *LedgerHook) OnToolResult(call *ToolCall, result string, err error) {
	if h == nil || call == nil || h.ledger == nil || !h.ledger.Enabled {
		return
	}
	h.mu.Lock()
	session := h.session
	start, hasStart := h.started[call.ID]
	delete(h.started, call.ID)
	h.mu.Unlock()
	isTest := strings.HasPrefix(session, "test-")

	var durationMs int64
	if hasStart {
		durationMs = time.Since(start).Milliseconds()
	}

	if err != nil {
		h.enqueue(memory.LedgerEvent{
			Type: memory.EventError, Action: call.Name, Detail: err.Error(),
			Status: memory.StatusError, DurationMs: durationMs,
			Session: session, Test: isTest,
		})
		return
	}

	status := memory.StatusOK
	switch {
	case editTools[call.Name]:
		h.enqueue(memory.LedgerEvent{Type: memory.EventEdit, Action: call.Name, Path: extractPath(call.Arguments), Status: status, DurationMs: durationMs, Session: session, Test: isTest})
	case call.Name == "git_commit":
		hash := firstHash(result)
		if hash == "" {
			hash = firstHash(call.Arguments)
		}
		subject := firstLine(result)
		if subject != "" && hash != "" {
			subject = strings.TrimPrefix(subject, hash+" ")
		}
		h.enqueue(memory.LedgerEvent{Type: memory.EventCommit, Action: "commit", Hash: hash, Detail: truncateRunes(subject, 80), Status: status, DurationMs: durationMs, Session: session})
	case memoryTools[call.Name]:
		h.enqueue(memory.LedgerEvent{Type: memory.EventMemory, Action: call.Name, Path: argKey(call.Arguments), Detail: argCategory(call.Arguments), Status: status, DurationMs: durationMs, Session: session})
	default:
		// Resto de tools (lecturas incluidas): solo nombre + path extraíble,
		// nunca argumentos (anti-secretos). Tipo `tool` (excluido del digest IA).
		h.enqueue(memory.LedgerEvent{Type: memory.EventTool, Action: call.Name, Path: extractPath(call.Arguments), Status: status, DurationMs: durationMs, Session: session, Test: isTest})
	}
}

// Approval registra la decisión del usuario sobre una tool (allow/deny/always).
func (h *LedgerHook) Approval(call *ToolCall, approved, action string) {
	if h == nil || call == nil || h.ledger == nil || !h.ledger.Enabled {
		return
	}
	h.mu.Lock()
	session := h.session
	h.mu.Unlock()
	isTest := strings.HasPrefix(session, "test-")
	approved = strings.ToLower(strings.TrimSpace(approved))
	if approved != memory.ApprovedYes && approved != memory.ApprovedNo {
		approved = memory.ApprovedNA
	}
	if action == "" {
		action = call.Name
	}
	h.enqueue(memory.LedgerEvent{Type: memory.EventApproval, Action: action, Approved: approved, Session: session, Test: isTest})
}

func (h *LedgerHook) Wrap(sink *EventSink) *EventSink {
	if sink == nil {
		return &EventSink{OnToolStart: h.OnToolStart, OnToolResult: h.OnToolResult}
	}
	return &EventSink{
		OnToolStart: func(call *ToolCall) {
			h.OnToolStart(call)
			if sink.OnToolStart != nil {
				sink.OnToolStart(call)
			}
		},
		OnToolResult: func(call *ToolCall, result string, err error) {
			h.OnToolResult(call, result, err)
			if sink.OnToolResult != nil {
				sink.OnToolResult(call, result, err)
			}
		},
	}
}

func (h *LedgerHook) Stats() (drops, writes int64) {
	return atomic.LoadInt64(&h.drops), atomic.LoadInt64(&h.writes)
}

func (h *LedgerHook) Close() {
	if h == nil {
		return
	}
	h.closeOnce.Do(func() {
		h.EndSession()
		close(h.done)
	})
}

func argMap(args string) map[string]interface{} {
	var m map[string]interface{}
	if json.Unmarshal([]byte(args), &m) != nil {
		return nil
	}
	return m
}

func extractPath(args string) string {
	m := argMap(args)
	if m == nil {
		return ""
	}
	for _, k := range pathArgs {
		if v, ok := m[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

func argKey(args string) string {
	m := argMap(args)
	if m == nil {
		return ""
	}
	if v, ok := m["key"].(string); ok {
		return v
	}
	return ""
}

func argCategory(args string) string {
	m := argMap(args)
	if m == nil {
		return ""
	}
	if v, ok := m["category"].(string); ok {
		return v
	}
	return ""
}

func firstHash(s string) string {
	if h := hashRE.FindString(s); h != "" {
		if len(h) > 8 {
			return h[:8]
		}
		return h
	}
	return ""
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
