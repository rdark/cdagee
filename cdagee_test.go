package cdagee_test

import (
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/rdark/cdagee"
)

// testdataDir returns the absolute path to target/testdata/<name>.
func testdataDir(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "target", "testdata", name)
}

func TestLoadSimple(t *testing.T) {
	p, err := cdagee.Load(testdataDir("simple"))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(p.Targets))
	}
	if len(p.Layers) != 2 {
		t.Fatalf("expected 2 layers, got %d", len(p.Layers))
	}
	if p.Graph.Len() != 2 {
		t.Fatalf("expected graph with 2 nodes, got %d", p.Graph.Len())
	}
}

func TestLoadWithTags(t *testing.T) {
	// tagged testdata has: infra(deploy), app(deploy,test), db(deploy),
	// ci(ci), monitoring(test)
	p, err := cdagee.Load(testdataDir("tagged"), "deploy")
	if err != nil {
		t.Fatal(err)
	}

	var ids []string
	for _, tgt := range p.Targets {
		ids = append(ids, tgt.ID)
	}
	slices.Sort(ids)

	want := []string{"app", "db", "infra"}
	if !slices.Equal(ids, want) {
		t.Errorf("expected targets %v, got %v", want, ids)
	}
}

func TestLoadTargetLookup(t *testing.T) {
	p, err := cdagee.Load(testdataDir("simple"))
	if err != nil {
		t.Fatal(err)
	}

	tgt, ok := p.Target("ws-a")
	if !ok {
		t.Fatal("expected to find target ws-a")
	}
	if tgt.ID != "ws-a" {
		t.Errorf("expected ID ws-a, got %q", tgt.ID)
	}

	_, ok = p.Target("nonexistent")
	if ok {
		t.Error("expected false for nonexistent target")
	}
}

func TestLoadEmpty(t *testing.T) {
	p, err := cdagee.Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Targets) != 0 {
		t.Errorf("expected 0 targets, got %d", len(p.Targets))
	}
	if len(p.Layers) != 0 {
		t.Errorf("expected 0 layers, got %d", len(p.Layers))
	}
}

func TestLoadInvalidRoot(t *testing.T) {
	_, err := cdagee.Load("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Fatal("expected error for invalid root, got nil")
	}
}
