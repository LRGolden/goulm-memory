package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func monthOf(ts string) string {
	if len(ts) >= 7 {
		return ts[:7]
	}
	return "unknown"
}

func (l *Ledger) compactLocked() {
	lines := readEvents(l.Active)
	if len(lines) <= l.Window {
		return
	}
	before := len(lines)
	overflow := lines[:before-l.Window]
	keep := lines[before-l.Window:]

	release, err := lockFile(l.Lock)
	if err != nil {
		return
	}
	defer release()

	byMonth := map[string][]LedgerEvent{}
	for _, ev := range overflow {
		m := monthOf(ev.TS)
		byMonth[m] = append(byMonth[m], ev)
	}
	for month, evs := range byMonth {
		path := filepath.Join(l.Archives, "ledger."+month+".jsonl")
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			continue
		}
		for _, ev := range evs {
			if line, err := marshalEvent(ev); err == nil {
				f.Write(append(line, '\n'))
			}
		}
		f.Close()
	}

	nowLines := readEvents(l.Active)
	if len(nowLines) > before {
		keep = append(keep, nowLines[before:]...)
	}

	var sb strings.Builder
	for _, ev := range keep {
		if line, err := marshalEvent(ev); err == nil {
			sb.Write(line)
			sb.WriteByte('\n')
		}
	}
	atomicWrite(l.Active, []byte(sb.String()), 0600)
}

func marshalEvent(ev LedgerEvent) ([]byte, error) {
	if ev.V == 0 {
		ev.V = 1
	}
	if ev.TS == "" {
		ev.TS = nowISO()
	}
	return json.Marshal(ev)
}

func (l *Ledger) CompactNow() error {
	if l == nil || !l.Enabled {
		return ErrLedgerDisabled
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.compactLocked()
	after := len(readEvents(l.Active))
	if after > l.Window {
		return fmt.Errorf("compactación no redujo la ventana: %d > %d", after, l.Window)
	}
	return nil
}
