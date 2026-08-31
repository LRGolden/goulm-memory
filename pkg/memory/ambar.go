package memory

import (
	"fmt"
	"strconv"
	"strings"
)

// Format identifica el formato de almacenamiento.
type Format string

const (
	FormatJSON  Format = "json"
	FormatAmbar Format = "ambar"
)

// ambarEscapes: valores de atributos y cuerpo se escapan con backslash.
var ambarEscapes = strings.NewReplacer(
	`\`, `\\`,
	`|`, `\|`,
	"\n", `\n`,
	"\r", `\r`,
)

var ambarUnescapes = strings.NewReplacer(
	`\\`, `\`,
	`\|`, `|`,
	`\n`, "\n",
	`\r`, "\r",
)

// ambarVersion es la versión del formato Ámbar.
const ambarVersion = "1"

// MarshalAmbar serializa cápsulas al formato Ámbar.
func MarshalAmbar(project string, capsules []*Capsule) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("v:%s|project:%s|updated:%s|count:%d\n",
		ambarVersion, project, nowISO(), len(capsules)))
	for _, c := range capsules {
		sb.WriteString("~\n")
		sb.WriteString(ambarAttributes(c))
		sb.WriteByte('\n')
		if c.Content != "" {
			sb.WriteString("content>" + ambarEscapes.Replace(c.Content) + "\n")
		}
		if c.File != "" {
			sb.WriteString("file>" + ambarEscapes.Replace(c.File) + "\n")
		}
		if len(c.Links) > 0 {
			sb.WriteString("links>" + ambarEscapes.Replace(strings.Join(c.Links, ";")) + "\n")
		}
		if c.PathScope != "" {
			sb.WriteString("scope>" + ambarEscapes.Replace(c.PathScope) + "\n")
		}
		if c.LastAccessed != "" {
			sb.WriteString("last>" + c.LastAccessed + "\n")
		}
		if c.SupersededOn != "" {
			sb.WriteString("superseded>" + c.SupersededOn + "\n")
		}
		if len(c.Embedding) > 0 {
			parts := make([]string, len(c.Embedding))
			for i, v := range c.Embedding {
				parts[i] = strconv.FormatFloat(float64(v), 'f', -1, 32)
			}
			sb.WriteString("embedding>" + strings.Join(parts, ",") + "\n")
		}
		if len(c.Tokens) > 0 {
			sb.WriteString("tokens>" + ambarEscapes.Replace(strings.Join(c.Tokens, ";")) + "\n")
		}
	}
	return sb.String()
}

// ambarAttributes serializa la línea de atributos de una cápsula.
func ambarAttributes(c *Capsule) string {
	parts := []string{
		"id:" + c.ID,
		"key:" + c.Key,
		"cat:" + string(c.Category),
		"date:" + c.Date,
	}
	if len(c.Tags) > 0 {
		parts = append(parts, "tags:"+ambarEscapes.Replace(strings.Join(c.Tags, ";")))
	}
	if c.TTL != "" {
		parts = append(parts, "ttl:"+c.TTL)
	}
	if c.Accessed > 0 {
		parts = append(parts, "acc:"+strconv.Itoa(c.Accessed))
	}
	parts = append(parts, fmt.Sprintf("q:%.2f", c.Quality))
	parts = append(parts, fmt.Sprintf("c:%.2f", c.Confidence))
	if c.Origin != OriginAgent {
		parts = append(parts, "origin:"+string(c.Origin))
	}
	if c.Status != StatusActive {
		parts = append(parts, "status:"+string(c.Status))
	}
	if c.Priority > 0 {
		parts = append(parts, "pri:"+strconv.Itoa(c.Priority))
	}
	return strings.Join(parts, "|")
}

// UnmarshalAmbar parsea el formato Ámbar. Devuelve las cápsulas y el
// proyecto declarado en la cabecera. Es tolerante: los campos faltantes usan
// defaults y las líneas desconocidas se ignoran.
func UnmarshalAmbar(data string) (project string, capsules []*Capsule, err error) {
	if strings.TrimSpace(data) == "" {
		return "", nil, nil
	}
	lines := strings.Split(data, "\n")
	header := strings.TrimSpace(lines[0])
	if !strings.HasPrefix(header, "v:") {
		return "", nil, fmt.Errorf("cabecera Ámbar inválida")
	}

	var cur *Capsule
	flush := func() {
		if cur != nil && cur.ID != "" && cur.Key != "" {
			// Pre-computar tokens si no existen.
			if len(cur.Tokens) == 0 {
				cur.Tokens = computeTokens(cur)
			}
			capsules = append(capsules, cur)
		}
		cur = nil
	}

	for i, raw := range lines {
		if i == 0 {
			project = ambarHeaderField(header, "project")
			continue
		}
		line := strings.TrimRight(raw, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.TrimSpace(line) == "~" {
			flush()
			cur = &Capsule{
				Category:   CategoryKnowledge,
				Origin:     OriginAgent,
				Status:     StatusActive,
				Confidence: ConfidenceFor(OriginAgent),
			}
			continue
		}
		if cur == nil {
			continue
		}
		if strings.HasPrefix(line, "content>") {
			cur.Content = ambarUnescapes.Replace(strings.TrimPrefix(line, "content>"))
			continue
		}
		if strings.HasPrefix(line, "file>") {
			cur.File = ambarUnescapes.Replace(strings.TrimPrefix(line, "file>"))
			continue
		}
		if strings.HasPrefix(line, "links>") {
			cur.Links = splitAmbarList(ambarUnescapes.Replace(strings.TrimPrefix(line, "links>")))
			continue
		}
		if strings.HasPrefix(line, "scope>") {
			cur.PathScope = ambarUnescapes.Replace(strings.TrimPrefix(line, "scope>"))
			continue
		}
		if strings.HasPrefix(line, "last>") {
			cur.LastAccessed = strings.TrimPrefix(line, "last>")
			continue
		}
		if strings.HasPrefix(line, "superseded>") {
			cur.SupersededOn = strings.TrimPrefix(line, "superseded>")
			continue
		}
		if strings.HasPrefix(line, "embedding>") {
			raw := strings.TrimPrefix(line, "embedding>")
			if raw != "" {
				parts := strings.Split(raw, ",")
				emb := make([]float32, 0, len(parts))
				for _, p := range parts {
					if f, err := strconv.ParseFloat(strings.TrimSpace(p), 32); err == nil {
						emb = append(emb, float32(f))
					}
				}
				if len(emb) > 0 {
					cur.Embedding = emb
				}
			}
			continue
		}
		if strings.HasPrefix(line, "tokens>") {
			raw := strings.TrimPrefix(line, "tokens>")
			if raw != "" {
				cur.Tokens = splitAmbarList(ambarUnescapes.Replace(raw))
			}
			continue
		}
		// Línea de atributos.
		for _, pair := range strings.Split(line, "|") {
			kv := strings.SplitN(pair, ":", 2)
			if len(kv) != 2 {
				continue
			}
			key, val := kv[0], ambarUnescapes.Replace(kv[1])
			switch key {
			case "id":
				cur.ID = val
			case "key":
				cur.Key = val
			case "cat":
				if ValidCategory(Category(val)) {
					cur.Category = Category(val)
				}
			case "tags":
				cur.Tags = splitAmbarList(val)
			case "date":
				cur.Date = val
			case "ttl":
				cur.TTL = val
			case "acc":
				cur.Accessed, _ = strconv.Atoi(val)
			case "q":
				cur.Quality = parseAmbarFloat(val)
			case "c":
				cur.Confidence = parseAmbarFloat(val)
			case "origin":
				if ValidOrigin(Origin(val)) {
					cur.Origin = Origin(val)
				}
			case "status":
				if ValidStatus(Status(val)) {
					cur.Status = Status(val)
				}
			case "pri":
				cur.Priority, _ = strconv.Atoi(val)
			}
		}
	}
	flush()
	return project, capsules, nil
}

func ambarHeaderField(header, name string) string {
	for _, pair := range strings.Split(header, "|") {
		if strings.HasPrefix(pair, name+":") {
			return strings.TrimPrefix(pair, name+":")
		}
	}
	return ""
}

func splitAmbarList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ";")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseAmbarFloat(s string) float64 {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return clamp01(f)
}
