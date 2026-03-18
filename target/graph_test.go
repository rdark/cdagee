package target_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/rdark/cdagee/target"
)

func TestBuildGraphSimple(t *testing.T) {
	root := filepath.Join(testdataDir(t), "simple")
	result, err := target.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	targets := result.Targets
	g, err := target.BuildGraph(targets)
	if err != nil {
		t.Fatal(err)
	}
	if g.Len() != 2 {
		t.Errorf("expected 2 nodes, got %d", g.Len())
	}
	order, err := g.TopologicalSort()
	if err != nil {
		t.Fatal(err)
	}
	if len(order) != 2 || order[0] != "ws-a" || order[1] != "ws-b" {
		t.Errorf("unexpected order: %v", order)
	}
}

func TestBuildGraphDiamond(t *testing.T) {
	root := filepath.Join(testdataDir(t), "diamond")
	result, err := target.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	targets := result.Targets
	g, err := target.BuildGraph(targets)
	if err != nil {
		t.Fatal(err)
	}
	if g.Len() != 4 {
		t.Errorf("expected 4 nodes, got %d", g.Len())
	}
	layers, err := g.Layers()
	if err != nil {
		t.Fatal(err)
	}
	if len(layers) != 3 {
		t.Errorf("expected 3 layers, got %d", len(layers))
	}
}

func TestBuildGraphCycle(t *testing.T) {
	root := filepath.Join(testdataDir(t), "cycle")
	result, err := target.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	targets := result.Targets
	_, err = target.BuildGraph(targets)
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}
	var ce *target.CycleError
	if !errors.As(err, &ce) {
		t.Errorf("expected *CycleError, got %T: %v", err, err)
	}
}

func TestBuildGraphDanglingRef(t *testing.T) {
	root := filepath.Join(testdataDir(t), "dangling")
	result, err := target.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	targets := result.Targets
	_, err = target.BuildGraph(targets)
	if err == nil {
		t.Fatal("expected dangling ref error, got nil")
	}
	var de *target.DanglingRefError
	if !errors.As(err, &de) {
		t.Errorf("expected *DanglingRefError, got %T: %v", err, err)
	}
}

func TestBuildGraphNested(t *testing.T) {
	root := filepath.Join(testdataDir(t), "nested")
	result, err := target.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	targets := result.Targets
	g, err := target.BuildGraph(targets)
	if err != nil {
		t.Fatal(err)
	}
	order, err := g.TopologicalSort()
	if err != nil {
		t.Fatal(err)
	}
	if len(order) != 2 || order[0] != "a/d" || order[1] != "a/b/c" {
		t.Errorf("unexpected order: %v", order)
	}
}

func TestBuildGraphDeepChain(t *testing.T) {
	root := filepath.Join(testdataDir(t), "deep-chain")
	result, err := target.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	targets := result.Targets
	g, err := target.BuildGraph(targets)
	if err != nil {
		t.Fatal(err)
	}
	if g.Len() != 5 {
		t.Errorf("expected 5 nodes, got %d", g.Len())
	}
	order, err := g.TopologicalSort()
	if err != nil {
		t.Fatal(err)
	}
	// Linear chain: a → b → c → d → e
	want := []string{"a", "b", "c", "d", "e"}
	if len(order) != len(want) {
		t.Fatalf("expected %d nodes in order, got %d: %v", len(want), len(order), order)
	}
	for i, id := range want {
		if order[i] != id {
			t.Errorf("order[%d]: expected %q, got %q", i, id, order[i])
		}
	}
	layers, err := g.Layers()
	if err != nil {
		t.Fatal(err)
	}
	if len(layers) != 5 {
		t.Errorf("expected 5 layers, got %d", len(layers))
	}
}

func TestBuildGraphForest(t *testing.T) {
	root := filepath.Join(testdataDir(t), "forest")
	result, err := target.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	targets := result.Targets
	g, err := target.BuildGraph(targets)
	if err != nil {
		t.Fatal(err)
	}
	if g.Len() != 4 {
		t.Errorf("expected 4 nodes, got %d", g.Len())
	}
	roots := g.Roots()
	if len(roots) != 2 {
		t.Errorf("expected 2 roots, got %d: %v", len(roots), roots)
	}
	layers, err := g.Layers()
	if err != nil {
		t.Fatal(err)
	}
	if len(layers) != 2 {
		t.Errorf("expected 2 layers, got %d", len(layers))
	}
}

func TestBuildGraphWideFan(t *testing.T) {
	root := filepath.Join(testdataDir(t), "wide-fan")
	result, err := target.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	targets := result.Targets
	g, err := target.BuildGraph(targets)
	if err != nil {
		t.Fatal(err)
	}
	if g.Len() != 5 {
		t.Errorf("expected 5 nodes, got %d", g.Len())
	}
	roots := g.Roots()
	if len(roots) != 1 || roots[0] != "hub" {
		t.Errorf("expected 1 root (hub), got %v", roots)
	}
	layers, err := g.Layers()
	if err != nil {
		t.Fatal(err)
	}
	if len(layers) != 2 {
		t.Errorf("expected 2 layers, got %d", len(layers))
	}
	// Layer 1 should have 4 spokes
	if len(layers[1]) != 4 {
		t.Errorf("expected 4 nodes in layer 1, got %d: %v", len(layers[1]), layers[1])
	}
}

func TestBuildGraphMultiRoot(t *testing.T) {
	root := filepath.Join(testdataDir(t), "multi-root")
	result, err := target.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	targets := result.Targets
	g, err := target.BuildGraph(targets)
	if err != nil {
		t.Fatal(err)
	}
	if g.Len() != 4 {
		t.Errorf("expected 4 nodes, got %d", g.Len())
	}
	roots := g.Roots()
	if len(roots) != 2 {
		t.Errorf("expected 2 roots, got %d: %v", len(roots), roots)
	}
	layers, err := g.Layers()
	if err != nil {
		t.Fatal(err)
	}
	if len(layers) != 3 {
		t.Errorf("expected 3 layers, got %d", len(layers))
	}
}

func TestBuildGraphEmpty(t *testing.T) {
	g, err := target.BuildGraph(nil)
	if err != nil {
		t.Fatal(err)
	}
	if g.Len() != 0 {
		t.Errorf("expected 0 nodes, got %d", g.Len())
	}
}

func TestBuildGraphSelfDep(t *testing.T) {
	targets := []target.Target{
		{ID: "ws-a", Dir: "/tmp/ws-a", Config: target.TargetConfig{DependsOn: []string{"ws-a"}}},
	}
	_, err := target.BuildGraph(targets)
	if err == nil {
		t.Fatal("expected error for self-dependency, got nil")
	}
}

func TestBuildGraphDuplicateIDs(t *testing.T) {
	targets := []target.Target{
		{ID: "ws-a", Dir: "/tmp/ws-a1", Config: target.TargetConfig{}},
		{ID: "ws-a", Dir: "/tmp/ws-a2", Config: target.TargetConfig{}},
	}
	_, err := target.BuildGraph(targets)
	if err == nil {
		t.Fatal("expected error for duplicate IDs, got nil")
	}
	var de *target.DuplicateIDError
	if !errors.As(err, &de) {
		t.Errorf("expected *DuplicateIDError, got %T: %v", err, err)
	}
}

func TestValidateSimple(t *testing.T) {
	root := filepath.Join(testdataDir(t), "simple")
	result, err := target.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	targets := result.Targets
	if err := target.Validate(targets); err != nil {
		t.Errorf("expected valid graph, got %v", err)
	}
}

func TestValidateCycle(t *testing.T) {
	root := filepath.Join(testdataDir(t), "cycle")
	result, err := target.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	targets := result.Targets
	if err := target.Validate(targets); err == nil {
		t.Fatal("expected validation error for cycle, got nil")
	}
}

func TestBuildGraphSerial(t *testing.T) {
	root := filepath.Join(testdataDir(t), "serial")
	result, err := target.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	targets := result.Targets
	g, err := target.BuildGraph(targets)
	if err != nil {
		t.Fatal(err)
	}
	order, err := g.TopologicalSort()
	if err != nil {
		t.Fatal(err)
	}
	// base must come first, then chain: alpha < beta < gamma
	pos := make(map[string]int, len(order))
	for i, id := range order {
		pos[id] = i
	}
	if pos["base"] >= pos["svc:alpha"] {
		t.Errorf("expected base before svc:alpha, order: %v", order)
	}
	if pos["svc:alpha"] >= pos["svc:beta"] {
		t.Errorf("expected svc:alpha before svc:beta, order: %v", order)
	}
	if pos["svc:beta"] >= pos["svc:gamma"] {
		t.Errorf("expected svc:beta before svc:gamma, order: %v", order)
	}
}

func TestBuildGraphSerialExplicit(t *testing.T) {
	root := filepath.Join(testdataDir(t), "serial-explicit")
	result, err := target.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	targets := result.Targets
	g, err := target.BuildGraph(targets)
	if err != nil {
		t.Fatal(err)
	}
	order, err := g.TopologicalSort()
	if err != nil {
		t.Fatal(err)
	}
	pos := make(map[string]int, len(order))
	for i, id := range order {
		pos[id] = i
	}
	// Explicit: alpha → gamma. Serial chain fills: alpha → beta → gamma.
	if pos["app:alpha"] >= pos["app:beta"] {
		t.Errorf("expected app:alpha before app:beta, order: %v", order)
	}
	if pos["app:beta"] >= pos["app:gamma"] {
		t.Errorf("expected app:beta before app:gamma, order: %v", order)
	}
}

func TestBuildGraphParallelTargets(t *testing.T) {
	root := filepath.Join(testdataDir(t), "parallel-targets")
	result, err := target.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	targets := result.Targets
	g, err := target.BuildGraph(targets)
	if err != nil {
		t.Fatal(err)
	}
	// All three are independent roots
	roots := g.Roots()
	if len(roots) != 3 {
		t.Errorf("expected 3 roots, got %d: %v", len(roots), roots)
	}
	layers, err := g.Layers()
	if err != nil {
		t.Fatal(err)
	}
	if len(layers) != 1 {
		t.Errorf("expected 1 layer, got %d", len(layers))
	}
}
