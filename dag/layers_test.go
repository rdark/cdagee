package dag_test

import (
	"slices"
	"testing"

	"github.com/rdark/cdagee/dag"
)

func TestLayersEmpty(t *testing.T) {
	d := dag.New[string]()
	layers, err := d.Layers()
	if err != nil {
		t.Fatal(err)
	}
	if layers != nil {
		t.Errorf("expected nil layers for empty graph, got %v", layers)
	}
}

func TestLayersSingleNode(t *testing.T) {
	d := dag.New[string]()
	mustAddNode(t, d, "a", "a")
	layers, err := d.Layers()
	if err != nil {
		t.Fatal(err)
	}
	if len(layers) != 1 || len(layers[0]) != 1 || layers[0][0] != "a" {
		t.Errorf("expected [[a]], got %v", layers)
	}
}

func TestLayersLinearChain(t *testing.T) {
	d := dag.New[string]()
	for _, id := range []string{"a", "b", "c", "d"} {
		mustAddNode(t, d, id, id)
	}
	mustAddEdge(t, d, "a", "b")
	mustAddEdge(t, d, "b", "c")
	mustAddEdge(t, d, "c", "d")

	layers, err := d.Layers()
	if err != nil {
		t.Fatal(err)
	}
	if len(layers) != 4 {
		t.Fatalf("expected 4 layers, got %d: %v", len(layers), layers)
	}
	expected := [][]string{{"a"}, {"b"}, {"c"}, {"d"}}
	for i, layer := range layers {
		if len(layer) != 1 || layer[0] != expected[i][0] {
			t.Errorf("layer %d: expected %v, got %v", i, expected[i], layer)
		}
	}
}

func TestLayersDiamond(t *testing.T) {
	// root -> left, right -> leaf
	d := dag.New[string]()
	for _, id := range []string{"root", "left", "right", "leaf"} {
		mustAddNode(t, d, id, id)
	}
	mustAddEdge(t, d, "root", "left")
	mustAddEdge(t, d, "root", "right")
	mustAddEdge(t, d, "left", "leaf")
	mustAddEdge(t, d, "right", "leaf")

	layers, err := d.Layers()
	if err != nil {
		t.Fatal(err)
	}
	if len(layers) != 3 {
		t.Fatalf("expected 3 layers, got %d: %v", len(layers), layers)
	}

	if len(layers[0]) != 1 || layers[0][0] != "root" {
		t.Errorf("layer 0: expected [root], got %v", layers[0])
	}
	slices.Sort(layers[1])
	if len(layers[1]) != 2 || layers[1][0] != "left" || layers[1][1] != "right" {
		t.Errorf("layer 1: expected [left right], got %v", layers[1])
	}
	if len(layers[2]) != 1 || layers[2][0] != "leaf" {
		t.Errorf("layer 2: expected [leaf], got %v", layers[2])
	}
}

func TestLayersWideFanOut(t *testing.T) {
	// root -> {a, b, c, d, e} (all in layer 1)
	d := dag.New[string]()
	mustAddNode(t, d, "root", "root")
	children := []string{"a", "b", "c", "d", "e"}
	for _, id := range children {
		mustAddNode(t, d, id, id)
		mustAddEdge(t, d, "root", id)
	}

	layers, err := d.Layers()
	if err != nil {
		t.Fatal(err)
	}
	if len(layers) != 2 {
		t.Fatalf("expected 2 layers, got %d: %v", len(layers), layers)
	}
	if len(layers[0]) != 1 || layers[0][0] != "root" {
		t.Errorf("layer 0: expected [root], got %v", layers[0])
	}
	slices.Sort(layers[1])
	if len(layers[1]) != 5 {
		t.Errorf("layer 1: expected 5 nodes, got %v", layers[1])
	}
}

func TestLayersBuildGraph(t *testing.T) {
	// Use the shared build() helper:
	// a -> b -> d
	//      b -> e
	// a -> c -> e
	d := build(t)
	layers, err := d.Layers()
	if err != nil {
		t.Fatal(err)
	}
	// Layer 0: a
	// Layer 1: b, c
	// Layer 2: d, e
	if len(layers) != 3 {
		t.Fatalf("expected 3 layers, got %d: %v", len(layers), layers)
	}
	if len(layers[0]) != 1 || layers[0][0] != "a" {
		t.Errorf("layer 0: expected [a], got %v", layers[0])
	}
	slices.Sort(layers[1])
	if len(layers[1]) != 2 || layers[1][0] != "b" || layers[1][1] != "c" {
		t.Errorf("layer 1: expected [b c], got %v", layers[1])
	}
	slices.Sort(layers[2])
	if len(layers[2]) != 2 || layers[2][0] != "d" || layers[2][1] != "e" {
		t.Errorf("layer 2: expected [d e], got %v", layers[2])
	}
}

func TestLayersValidTwoNodes(t *testing.T) {
	d := dag.New[string]()
	mustAddNode(t, d, "x", "x")
	mustAddNode(t, d, "y", "y")
	mustAddEdge(t, d, "x", "y")

	layers, err := d.Layers()
	if err != nil {
		t.Fatal(err)
	}
	if len(layers) != 2 {
		t.Fatalf("expected 2 layers, got %d: %v", len(layers), layers)
	}
}

func TestLayersMultipleRoots(t *testing.T) {
	d := dag.New[string]()
	for _, id := range []string{"r1", "r2", "r3", "child"} {
		mustAddNode(t, d, id, id)
	}
	mustAddEdge(t, d, "r1", "child")
	mustAddEdge(t, d, "r2", "child")
	mustAddEdge(t, d, "r3", "child")

	layers, err := d.Layers()
	if err != nil {
		t.Fatal(err)
	}
	if len(layers) != 2 {
		t.Fatalf("expected 2 layers, got %d: %v", len(layers), layers)
	}
	slices.Sort(layers[0])
	if len(layers[0]) != 3 {
		t.Errorf("layer 0: expected 3 roots, got %v", layers[0])
	}
	if len(layers[1]) != 1 || layers[1][0] != "child" {
		t.Errorf("layer 1: expected [child], got %v", layers[1])
	}
}
