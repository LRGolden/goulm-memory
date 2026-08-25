package memory

import (
	"fmt"
	"testing"
)

func mkCapsule(key, content string, links ...string) *Capsule {
	return &Capsule{ID: NewID(), Category: CategoryKnowledge, Key: key, Content: content, Links: links}
}

func TestBuildGraphLinksAndRefs(t *testing.T) {
	caps := []*Capsule{
		mkCapsule("engine-arch", "Arquitectura del motor"),
		mkCapsule("risk-spec", "Especificación (ver [[engine-arch]])"),
		mkCapsule("risk-engine", "Motor de riesgo", "engine-arch"),
		mkCapsule("solo", "Aislada"),
	}
	g := BuildGraph(caps)
	if !g.HasKey("engine-arch") {
		t.Fatal("engine-arch debería existir")
	}
	// risk-engine → engine-arch (link explícito).
	if !hasNeighbor(g, "risk-engine", "engine-arch") {
		t.Error("link explícito no creó edge")
	}
	// risk-spec → engine-arch (ref [[engine-arch]]).
	if !hasNeighbor(g, "risk-spec", "engine-arch") {
		t.Error("ref [[engine-arch]] no creó edge")
	}
	// solo no tiene edges.
	if g.Degree("solo") != 0 {
		t.Error("solo debería estar aislada")
	}
}

func TestBuildGraphSharedTags(t *testing.T) {
	caps := []*Capsule{
		{ID: "a", Key: "x", Category: CategoryPattern, Tags: []string{"db", "go"}},
		{ID: "b", Key: "y", Category: CategoryPattern, Tags: []string{"db", "go"}},
		{ID: "c", Key: "z", Category: CategoryPattern, Tags: []string{"db"}},
	}
	g := BuildGraph(caps)
	if !hasNeighbor(g, "x", "y") {
		t.Error("2 tags compartidas deberían crear edge")
	}
	if hasNeighbor(g, "x", "z") {
		t.Error("1 tag compartida no debería crear edge")
	}
}

func hasNeighbor(g *Graph, a, b string) bool {
	for _, nb := range g.Neighbors(a) {
		if nb == b {
			return true
		}
	}
	return false
}

func TestEgoExpand(t *testing.T) {
	caps := []*Capsule{
		mkCapsule("a", "A"),
		mkCapsule("b", "B"),
		mkCapsule("c", "C"),
		mkCapsule("d", "D"),
	}
	caps[0].Links = []string{"b"}
	caps[1].Links = []string{"c"}
	caps[2].Links = []string{"d"}
	g := BuildGraph(caps)
	dist := g.EgoExpand([]string{"a"}, 2, nil)
	if dist["a"] != 0 {
		t.Errorf("dist a = %d", dist["a"])
	}
	if dist["b"] != 1 || dist["c"] != 2 {
		t.Errorf("dist b/c = %d/%d, esperaba 1/2", dist["b"], dist["c"])
	}
	if _, ok := dist["d"]; ok {
		t.Error("d está a 3 saltos, no debería aparecer con hops=2")
	}
}

func TestShortestPath(t *testing.T) {
	caps := []*Capsule{
		mkCapsule("a", "A"),
		mkCapsule("b", "B"),
		mkCapsule("c", "C"),
		mkCapsule("d", "D"),
	}
	caps[0].Links = []string{"b"}
	caps[1].Links = []string{"c", "d"}
	caps[2].Links = []string{"d"}
	g := BuildGraph(caps)
	path := g.ShortestPath("a", "d")
	if len(path) != 3 || path[0] != "a" || path[2] != "d" {
		t.Errorf("camino = %v, esperaba [a b d]", path)
	}
	if got := g.ShortestPath("a", "zzz"); got != nil {
		t.Errorf("camino inexistente → %v, esperaba nil", got)
	}
}

func TestCentrality(t *testing.T) {
	caps := []*Capsule{
		mkCapsule("hub", "H"),
		mkCapsule("n1", "N1"),
		mkCapsule("n2", "N2"),
	}
	caps[0].Links = []string{"n1", "n2"}
	g := BuildGraph(caps)
	cent := g.Centrality()
	if cent["hub"] != 1.0 {
		t.Errorf("hub centrality = %.2f, esperaba 1", cent["hub"])
	}
	if cent["n1"] != 0.5 {
		t.Errorf("n1 centrality = %.2f, esperaba 0.5", cent["n1"])
	}
}

func TestLinkKey(t *testing.T) {
	if got := LinkKey("supersedes:engine-arch"); got != "engine-arch" {
		t.Errorf("typed link → %q", got)
	}
	if got := LinkKey("engine-arch"); got != "engine-arch" {
		t.Errorf("plain link → %q", got)
	}
}

// TestBuildGraphSharedTagsManyCapsules verifica que tags genéricos (>50
// capsules) no crean edges sintéticos, pero links explícitos sí.
func TestBuildGraphSharedTagsManyCapsules(t *testing.T) {
	caps := make([]*Capsule, 100)
	for i := range caps {
		caps[i] = &Capsule{
			ID:       fmt.Sprintf("id-%d", i),
			Key:      fmt.Sprintf("key-%d", i),
			Category: CategoryDecision,
			Tags:     []string{"common", "test"},
		}
	}
	// Un link explícito.
	caps[0].Links = []string{"supersedes:key-5"}

	g := BuildGraph(caps)

	// Tags compartidos con 100 capsules (>50) no crean edges.
	if hasNeighbor(g, "key-0", "key-1") {
		t.Error("tags compartidos no deberían crear edges con >50 capsules")
	}
	// Link explícito SÍ crea edge.
	if !hasNeighbor(g, "key-0", "key-5") {
		t.Error("links explícitos deberían crear edges")
	}
}

// TestBuildGraphSharedTagsBelowThreshold verifica que tags específicos
// (≤50 capsules) SÍ crean edges cuando comparten ≥2 tags.
func TestBuildGraphSharedTagsBelowThreshold(t *testing.T) {
	caps := make([]*Capsule, 30)
	for i := range caps {
		caps[i] = &Capsule{
			ID:       fmt.Sprintf("id-%d", i),
			Key:      fmt.Sprintf("key-%d", i),
			Category: CategoryDecision,
			Tags:     []string{"specific-a", "specific-b"},
		}
	}

	g := BuildGraph(caps)

	// Tags específicos con 30 capsules (≤50) y 2 tags compartidos crean edges.
	if !hasNeighbor(g, "key-0", "key-1") {
		t.Error("tags específicos deberían crear edges con ≤50 capsules")
	}
}
