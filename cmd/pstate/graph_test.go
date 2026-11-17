package pstate

import (
	"slices"
	"testing"
)

func testNodes() []node {
	return []node{
		{ID: "checkout"},
		{ID: "lint", Deps: []string{"checkout"}},
		{ID: "build", Deps: []string{"checkout"}, Priority: 1},
		{ID: "test", Deps: []string{"build"}, Priority: 2},
		{ID: "deploy", Deps: []string{"test", "lint"}, Priority: 3},
	}
}

func TestBuildGraphColumnsAndDummies(t *testing.T) {
	g := buildGraph(testNodes(), LayoutDependency)
	if !slices.Equal(g.Titles, []string{"stage 1", "stage 2", "stage 3", "stage 4"}) {
		t.Fatalf("titles %v", g.Titles)
	}
	cols := map[string]int{}
	dummies := 0
	for _, s := range g.Slots {
		if s.ID == "" {
			dummies++
			continue
		}
		cols[s.ID] = s.Col
	}
	if cols["deploy"] != 3 || cols["test"] != 2 {
		t.Fatalf("cols %v", cols)
	}
	// lint (stage 2) → deploy (stage 4) 跨過 stage 3, 需要一個 dummy
	if dummies != 1 {
		t.Fatalf("dummies %d want 1", dummies)
	}
}

func TestBuildGraphOrdersToReduceCrossings(t *testing.T) {
	nodes := []node{
		{ID: "a"}, {ID: "b"},
		{ID: "from-b", Deps: []string{"b"}}, // 登記在前, 但上游在下方
		{ID: "from-a", Deps: []string{"a"}},
	}
	g := buildGraph(nodes, LayoutDependency)
	i, _ := g.find("from-a")
	j, _ := g.find("from-b")
	if g.Slots[i].Row > g.Slots[j].Row {
		t.Fatal("from-a should be placed above from-b to avoid a crossing")
	}
}

func TestGraphHitAndNeighbor(t *testing.T) {
	g := buildGraph(testNodes(), LayoutDependency)
	i, _ := g.find("build")
	x, y, w := g.rect(i)
	if g.hit(x, y) != "build" || g.hit(x+w-1, y+BOX_HEIGHT-1) != "build" {
		t.Fatal("hit inside box should return build")
	}
	if g.hit(x+w+1, y) != "" {
		t.Fatal("hit in the channel should return nothing")
	}
	if got := g.neighbor("checkout", 1, 0); got != "lint" && got != "build" {
		t.Fatalf("right of checkout: %q", got)
	}
	if got := g.neighbor("test", 1, 0); got != "deploy" {
		t.Fatalf("right of test: %q", got)
	}
	if got := g.neighbor("deploy", 1, 0); got != "deploy" {
		t.Fatalf("rightmost stays: %q", got)
	}
}

func TestPriorityLayoutSkipsBackwardEdges(t *testing.T) {
	nodes := []node{{ID: "a", Priority: 5}, {ID: "b", Deps: []string{"a"}, Priority: 1}}
	g := buildGraph(nodes, LayoutPriority)
	for _, s := range g.Slots {
		if len(s.Preds) > 0 {
			t.Fatal("edge from a later column must not be drawn")
		}
	}
}
