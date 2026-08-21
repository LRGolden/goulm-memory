package memory

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const SummaryBudget = 3200

type agg struct {
	edits      map[string]bool
	commits    []string
	memories   map[string]int
	milestones []string
	errors     int
	tests      int
	total      int
	durationMs int64
	costUSD    float64
}

func newAgg() *agg {
	return &agg{edits: map[string]bool{}, memories: map[string]int{}}
}

func (a *agg) add(ev LedgerEvent) {
	// El digest solo agrega *sucesos*; lecturas (tool), aprobaciones y eventos
	// de sistema quedan en el raw auditable.
	switch ev.Type {
	case EventTool, EventApproval, EventSystem:
		return
	}
	a.total++
	a.durationMs += ev.DurationMs
	a.costUSD += ev.CostUSD
	switch ev.Type {
	case EventEdit:
		if ev.Path != "" {
			a.edits[ev.Path] = true
		}
	case EventCommit:
		if ev.Hash != "" {
			a.commits = append(a.commits, ev.Hash)
		}
	case EventMemory:
		if ev.Detail != "" {
			a.memories[ev.Detail]++
		}
	case EventMilestone:
		a.milestones = append(a.milestones, ev.Detail)
	case EventError:
		a.errors++
	case EventTest:
		a.tests++
	}
}

func (a *agg) merge(b *agg) {
	a.total += b.total
	a.durationMs += b.durationMs
	a.costUSD += b.costUSD
	for k := range b.edits {
		a.edits[k] = true
	}
	a.commits = append(a.commits, b.commits...)
	for k, v := range b.memories {
		a.memories[k] += v
	}
	a.milestones = append(a.milestones, b.milestones...)
	a.errors += b.errors
	a.tests += b.tests
}

func (a *agg) files() []string {
	out := make([]string, 0, len(a.edits))
	for f := range a.edits {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

func (a *agg) memoryLine() string {
	if len(a.memories) == 0 {
		return ""
	}
	var parts []string
	for _, cat := range []string{"decision", "pattern", "bug", "knowledge"} {
		if n := a.memories[cat]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, cat))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " · ")
}

func (a *agg) milestoneLine() string {
	if len(a.milestones) == 0 {
		return ""
	}
	out := make([]string, 0, len(a.milestones))
	for _, m := range a.milestones {
		out = append(out, truncateRunes(m, 60))
	}
	return strings.Join(out, ", ")
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func (l *Ledger) Summary() string {
	if l == nil || !l.Enabled {
		return ""
	}
	events := l.allEvents()

	now := time.Now()
	days := map[string]*agg{}
	weeks := map[string]*agg{}
	months := map[string]*agg{}
	total := newAgg()

	for _, ev := range events {
		if ev.Test && ev.Type != EventTest {
			continue
		}
		t := parseTS(ev.TS, now)
		key := t.Format("2006-01-02")
		if days[key] == nil {
			days[key] = newAgg()
		}
		days[key].add(ev)

		y, w := t.ISOWeek()
		wkey := fmt.Sprintf("%04d-W%02d", y, w)
		if weeks[wkey] == nil {
			weeks[wkey] = newAgg()
		}
		weeks[wkey].add(ev)

		mkey := t.Format("2006-01")
		if months[mkey] == nil {
			months[mkey] = newAgg()
		}
		months[mkey].add(ev)

		total.add(ev)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "# Sucesos — %s (hasta %s)\n", l.Project, now.Format("2006-01-02 15:04"))

	sb.WriteString("\n## Últimos 7 días\n")
	var dayKeys []string
	for k := range days {
		if k >= now.AddDate(0, 0, -6).Format("2006-01-02") {
			dayKeys = append(dayKeys, k)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dayKeys)))
	for i, k := range dayKeys {
		if i >= 7 {
			break
		}
		renderDay(&sb, k, days[k])
	}

	sb.WriteString("\n## 90 días\n")
	var weekKeys []string
	for k := range weeks {
		wk := parseWKey(k)
		if now.Sub(wk) <= 90*24*time.Hour {
			weekKeys = append(weekKeys, k)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(weekKeys)))
	for i, k := range weekKeys {
		if i >= 13 {
			break
		}
		fmt.Fprintf(&sb, "- %s: %s\n", k, compactLine(weeks[k]))
	}

	sb.WriteString("\n## Histórico\n")
	var monthKeys []string
	for k := range months {
		monthKeys = append(monthKeys, k)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(monthKeys)))
	for i, k := range monthKeys {
		if i >= 24 {
			break
		}
		fmt.Fprintf(&sb, "- %s: %s\n", k, compactLine(months[k]))
	}
	fmt.Fprintf(&sb, "- Total: %s\n", compactLine(total))

	out := sb.String()
	if len([]rune(out)) <= SummaryBudget {
		return strings.TrimRight(out, "\n")
	}
	return strings.TrimRight(trimToBudget(out), "\n")
}

func parseTS(ts string, fallback time.Time) time.Time {
	if t, err := time.Parse(time.RFC3339, ts); err == nil {
		return t
	}
	if t, err := time.Parse("2006-01-02T15:04:05", ts); err == nil {
		return t
	}
	return fallback
}

func parseWKey(key string) time.Time {
	var year, week int
	if _, err := fmt.Sscanf(key, "%04d-W%02d", &year, &week); err != nil {
		return time.Time{}
	}
	jan4 := time.Date(year, 1, 4, 0, 0, 0, 0, time.Local)
	monday := jan4.AddDate(0, 0, -int(jan4.Weekday())+1)
	return monday.AddDate(0, 0, (week-1)*7)
}

func renderDay(sb *strings.Builder, key string, a *agg) {
	if a == nil || a.total == 0 {
		return
	}
	fmt.Fprintf(sb, "### %s (%d eventos)\n", key, a.total)
	if files := a.files(); len(files) > 0 {
		shown := files
		if len(shown) > 5 {
			shown = shown[:5]
		}
		fmt.Fprintf(sb, "- ✏️ %d archivos: %s\n", len(files), strings.Join(shown, ", "))
	}
	for _, c := range lastN(a.commits, 3) {
		fmt.Fprintf(sb, "- 🔀 commit %s\n", c)
	}
	if m := a.memoryLine(); m != "" {
		fmt.Fprintf(sb, "- 🧠 %s\n", m)
	}
	if ml := a.milestoneLine(); ml != "" {
		fmt.Fprintf(sb, "- ⭐ hito: %s\n", ml)
	}
	var flags []string
	if a.errors > 0 {
		flags = append(flags, fmt.Sprintf("❌ %d errores", a.errors))
	}
	if a.tests > 0 {
		flags = append(flags, fmt.Sprintf("🧪 %d tests", a.tests))
	}
	if len(flags) > 0 {
		fmt.Fprintf(sb, "- %s\n", strings.Join(flags, " · "))
	}
}

func compactLine(a *agg) string {
	var parts []string
	if len(a.edits) > 0 {
		parts = append(parts, fmt.Sprintf("%d archivos", len(a.edits)))
	}
	if len(a.commits) > 0 {
		parts = append(parts, fmt.Sprintf("%d commits", len(a.commits)))
	}
	if m := a.memoryLine(); m != "" {
		parts = append(parts, "🧠 "+m)
	}
	if len(a.milestones) > 0 {
		parts = append(parts, fmt.Sprintf("%d hitos", len(a.milestones)))
	}
	if a.errors > 0 {
		parts = append(parts, fmt.Sprintf("%d errores", a.errors))
	}
	if a.tests > 0 {
		parts = append(parts, fmt.Sprintf("%d tests", a.tests))
	}
	base := fmt.Sprintf("%d eventos", a.total)
	if d := formatDuration(a.durationMs); d != "" {
		base += " · " + d
	}
	if a.costUSD > 0 {
		base += fmt.Sprintf(" · $%.4f", a.costUSD)
	}
	return base + " · " + strings.Join(parts, " · ")
}

func lastN(items []string, n int) []string {
	if len(items) <= n {
		return items
	}
	return items[len(items)-n:]
}

func trimToBudget(s string) string {
	r := []rune(s)
	if len(r) <= SummaryBudget {
		return s
	}
	out := string(r[:SummaryBudget])
	if idx := strings.LastIndex(out, "\n"); idx > len(out)/2 {
		return out[:idx]
	}
	return out + "\n… (resumen truncado)"
}
