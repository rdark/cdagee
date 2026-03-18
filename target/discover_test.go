package target_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/rdark/cdagee/target"
)

func testdataDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file path")
	}
	return filepath.Join(filepath.Dir(file), "testdata")
}

func TestDiscoverSimple(t *testing.T) {
	root := filepath.Join(testdataDir(t), "simple")
	result, err := target.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	targets := result.Targets
	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %d: %v", len(targets), ids(targets))
	}
	// Walk order is lexicographic: ws-a before ws-b
	if targets[0].ID != "ws-a" || targets[1].ID != "ws-b" {
		t.Errorf("unexpected order: %v", ids(targets))
	}
	if len(targets[1].Config.DependsOn) != 1 || targets[1].Config.DependsOn[0] != "ws-a" {
		t.Errorf("ws-b depends_on: %v", targets[1].Config.DependsOn)
	}
}

func TestDiscoverDiamond(t *testing.T) {
	root := filepath.Join(testdataDir(t), "diamond")
	result, err := target.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	targets := result.Targets
	if len(targets) != 4 {
		t.Fatalf("expected 4 targets, got %d: %v", len(targets), ids(targets))
	}
}

func TestDiscoverNested(t *testing.T) {
	root := filepath.Join(testdataDir(t), "nested")
	result, err := target.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	targets := result.Targets
	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %d: %v", len(targets), ids(targets))
	}
	// Nested paths use forward slashes
	if targets[0].ID != "a/b/c" || targets[1].ID != "a/d" {
		t.Errorf("unexpected IDs: %v", ids(targets))
	}
}

func TestDiscoverEmpty(t *testing.T) {
	root := filepath.Join(testdataDir(t), "empty")
	result, err := target.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	targets := result.Targets
	if len(targets) != 0 {
		t.Errorf("expected 0 targets, got %d", len(targets))
	}
}

func TestDiscoverSkipsHidden(t *testing.T) {
	// The testdata root has a .hidden dir with cdagee.json — it should be skipped
	root := testdataDir(t)
	result, err := target.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	targets := result.Targets
	for _, tgt := range targets {
		if tgt.ID == ".hidden/ws-h" {
			t.Error("hidden directory target should have been skipped")
		}
	}
}

func TestDiscoverSkipsRootUpmanJSON(t *testing.T) {
	root := t.TempDir()
	// Place cdagee.json in root — should be ignored
	if err := os.WriteFile(filepath.Join(root, "cdagee.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Place cdagee.json in a subdirectory — should be discovered
	sub := filepath.Join(root, "child")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "cdagee.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := target.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	targets := result.Targets
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d: %v", len(targets), ids(targets))
	}
	if targets[0].ID != "child" {
		t.Errorf("expected ID 'child', got %q", targets[0].ID)
	}
}

func TestDiscoverDeepChain(t *testing.T) {
	root := filepath.Join(testdataDir(t), "deep-chain")
	result, err := target.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	targets := result.Targets
	got := ids(targets)
	want := []string{"a", "b", "c", "d", "e"}
	if len(got) != len(want) {
		t.Fatalf("expected %d targets, got %d: %v", len(want), len(got), got)
	}
	for i, id := range want {
		if got[i] != id {
			t.Errorf("target[%d]: expected %q, got %q", i, id, got[i])
		}
	}
}

func TestDiscoverForest(t *testing.T) {
	root := filepath.Join(testdataDir(t), "forest")
	result, err := target.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	targets := result.Targets
	got := ids(targets)
	want := []string{"tree1-leaf", "tree1-root", "tree2-leaf", "tree2-root"}
	if len(got) != len(want) {
		t.Fatalf("expected %d targets, got %d: %v", len(want), len(got), got)
	}
	for i, id := range want {
		if got[i] != id {
			t.Errorf("target[%d]: expected %q, got %q", i, id, got[i])
		}
	}
}

func TestDiscoverWideFan(t *testing.T) {
	root := filepath.Join(testdataDir(t), "wide-fan")
	result, err := target.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	targets := result.Targets
	got := ids(targets)
	want := []string{"hub", "spoke-a", "spoke-b", "spoke-c", "spoke-d"}
	if len(got) != len(want) {
		t.Fatalf("expected %d targets, got %d: %v", len(want), len(got), got)
	}
	for i, id := range want {
		if got[i] != id {
			t.Errorf("target[%d]: expected %q, got %q", i, id, got[i])
		}
	}
}

func TestDiscoverMultiRoot(t *testing.T) {
	root := filepath.Join(testdataDir(t), "multi-root")
	result, err := target.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	targets := result.Targets
	got := ids(targets)
	want := []string{"alpha", "beta", "final", "merge"}
	if len(got) != len(want) {
		t.Fatalf("expected %d targets, got %d: %v", len(want), len(got), got)
	}
	for i, id := range want {
		if got[i] != id {
			t.Errorf("target[%d]: expected %q, got %q", i, id, got[i])
		}
	}
}

func TestDiscoverTagged(t *testing.T) {
	root := filepath.Join(testdataDir(t), "tagged")
	result, err := target.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	targets := result.Targets
	got := ids(targets)
	want := []string{"app", "ci", "db", "infra", "monitoring"}
	if len(got) != len(want) {
		t.Fatalf("expected %d targets, got %d: %v", len(want), len(got), got)
	}
	for i, id := range want {
		if got[i] != id {
			t.Errorf("target[%d]: expected %q, got %q", i, id, got[i])
		}
	}
	// Verify tags are populated
	tagsByID := make(map[string][]string)
	for _, tgt := range targets {
		tagsByID[tgt.ID] = tgt.Config.Tags
	}
	if len(tagsByID["infra"]) != 1 || tagsByID["infra"][0] != "deploy" {
		t.Errorf("infra tags: expected [deploy], got %v", tagsByID["infra"])
	}
	if len(tagsByID["app"]) != 2 {
		t.Errorf("app tags: expected 2 tags, got %v", tagsByID["app"])
	}
	if len(tagsByID["monitoring"]) != 1 || tagsByID["monitoring"][0] != "test" {
		t.Errorf("monitoring tags: expected [test], got %v", tagsByID["monitoring"])
	}
}

func ids(targets []target.Target) []string {
	out := make([]string, len(targets))
	for i, tgt := range targets {
		out[i] = tgt.ID
	}
	return out
}

func TestDiscoverMultiTarget(t *testing.T) {
	root := filepath.Join(testdataDir(t), "multi-target")
	result, err := target.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	targets := result.Targets
	got := ids(targets)
	want := []string{"infra", "myapp:prod", "myapp:staging"}
	if len(got) != len(want) {
		t.Fatalf("expected %d targets, got %d: %v", len(want), len(got), got)
	}
	for i, id := range want {
		if got[i] != id {
			t.Errorf("target[%d]: expected %q, got %q", i, id, got[i])
		}
	}

	// Verify inheritance
	byID := make(map[string]target.Target)
	for _, tgt := range targets {
		byID[tgt.ID] = tgt
	}

	staging := byID["myapp:staging"]
	if !slicesEqual(staging.Config.DependsOn, []string{"infra"}) {
		t.Errorf("staging depends_on: expected [infra], got %v", staging.Config.DependsOn)
	}
	if !slicesEqual(staging.Config.Tags, []string{"deploy", "staging"}) {
		t.Errorf("staging tags: expected [deploy staging], got %v", staging.Config.Tags)
	}

	prod := byID["myapp:prod"]
	if !slicesEqual(prod.Config.DependsOn, []string{"infra", "myapp:staging"}) {
		t.Errorf("prod depends_on: expected [infra myapp:staging], got %v", prod.Config.DependsOn)
	}
	if !slicesEqual(prod.Config.Tags, []string{"deploy", "prod"}) {
		t.Errorf("prod tags: expected [deploy prod], got %v", prod.Config.Tags)
	}
}

func TestDiscoverSerial(t *testing.T) {
	root := filepath.Join(testdataDir(t), "serial")
	result, err := target.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	targets := result.Targets
	got := ids(targets)
	want := []string{"base", "svc:alpha", "svc:beta", "svc:gamma"}
	if len(got) != len(want) {
		t.Fatalf("expected %d targets, got %d: %v", len(want), len(got), got)
	}
	for i, id := range want {
		if got[i] != id {
			t.Errorf("target[%d]: expected %q, got %q", i, id, got[i])
		}
	}
	// svc:* targets should have Serial=true (default)
	for _, tgt := range targets {
		if tgt.ID == "base" {
			continue
		}
		if !tgt.Serial {
			t.Errorf("%s: expected Serial=true", tgt.ID)
		}
	}
}

func TestDiscoverParallelTargets(t *testing.T) {
	root := filepath.Join(testdataDir(t), "parallel-targets")
	result, err := target.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	targets := result.Targets
	got := ids(targets)
	want := []string{"svc:one", "svc:three", "svc:two"}
	if len(got) != len(want) {
		t.Fatalf("expected %d targets, got %d: %v", len(want), len(got), got)
	}
	for _, tgt := range targets {
		if tgt.Serial {
			t.Errorf("%s: expected Serial=false", tgt.ID)
		}
	}
}

func TestDiscoverColonExpansion(t *testing.T) {
	root := filepath.Join(testdataDir(t), "multi-target")
	result, err := target.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	targets := result.Targets
	byID := make(map[string]target.Target)
	for _, tgt := range targets {
		byID[tgt.ID] = tgt
	}
	// prod has depends_on: [":staging"] which should expand to "myapp:staging"
	prod := byID["myapp:prod"]
	found := false
	for _, dep := range prod.Config.DependsOn {
		if dep == "myapp:staging" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected myapp:staging in prod depends_on, got %v", prod.Config.DependsOn)
	}
}

func TestDiscoverMultiTargetDuplicateDeps(t *testing.T) {
	root := t.TempDir()
	// Create an "infra" target
	infra := filepath.Join(root, "infra")
	if err := os.MkdirAll(infra, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(infra, "cdagee.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	// Create a multi-target dir where both dir-level and target-level depend on "infra"
	app := filepath.Join(root, "app")
	if err := os.MkdirAll(app, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "cdagee.json"), []byte(`{
		"depends_on": ["infra"],
		"targets": {
			"blue": {"depends_on": ["infra"]},
			"green": {}
		}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := target.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	targets := result.Targets
	byID := make(map[string]target.Target)
	for _, tgt := range targets {
		byID[tgt.ID] = tgt
	}

	blue := byID["app:blue"]
	// "infra" should appear exactly once despite being in both dir and target deps
	count := 0
	for _, dep := range blue.Config.DependsOn {
		if dep == "infra" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 'infra' once in app:blue depends_on, got %d times: %v", count, blue.Config.DependsOn)
	}
}

// --- Direnv resolution tests ---

func TestDiscoverDirenvFromRootSettings(t *testing.T) {
	root := t.TempDir()
	// Root settings with direnv=true
	if err := os.WriteFile(filepath.Join(root, "cdagee.json"), []byte(`{"direnv": true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	// A target without direnv override
	sub := filepath.Join(root, "ws-a")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "cdagee.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := target.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if result.Settings.Direnv == nil || !*result.Settings.Direnv {
		t.Errorf("expected root settings direnv=true, got %v", result.Settings.Direnv)
	}
	if len(result.Targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(result.Targets))
	}
	if !result.Targets[0].Direnv {
		t.Error("expected target to inherit direnv=true from root settings")
	}
}

func TestDiscoverDirenvPerWorkspaceOverride(t *testing.T) {
	root := t.TempDir()
	// Root settings with direnv=true
	if err := os.WriteFile(filepath.Join(root, "cdagee.json"), []byte(`{"direnv": true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	// ws-a: no override (inherits true)
	wsA := filepath.Join(root, "ws-a")
	if err := os.MkdirAll(wsA, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wsA, "cdagee.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	// ws-b: explicitly disabled
	wsB := filepath.Join(root, "ws-b")
	if err := os.MkdirAll(wsB, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wsB, "cdagee.json"), []byte(`{"direnv": false}`), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := target.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	targets := result.Targets
	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(targets))
	}
	byID := make(map[string]target.Target)
	for _, tgt := range targets {
		byID[tgt.ID] = tgt
	}
	if !byID["ws-a"].Direnv {
		t.Error("ws-a: expected direnv=true (inherited from root)")
	}
	if byID["ws-b"].Direnv {
		t.Error("ws-b: expected direnv=false (per-workspace override)")
	}
}

func TestDiscoverDirenvDefaultFalse(t *testing.T) {
	root := t.TempDir()
	// No root settings
	sub := filepath.Join(root, "ws-a")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "cdagee.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := target.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(result.Targets))
	}
	if result.Targets[0].Direnv {
		t.Error("expected direnv=false when no root settings")
	}
}

func TestDiscoverRootSettingsBadJSON(t *testing.T) {
	root := t.TempDir()
	// Invalid root settings
	if err := os.WriteFile(filepath.Join(root, "cdagee.json"), []byte(`{bad`), 0o644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "ws-a")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "cdagee.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := target.Discover(root)
	if err == nil {
		t.Fatal("expected error for bad root settings JSON, got nil")
	}
}

func TestDiscoverDirenvWithMultiTarget(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "cdagee.json"), []byte(`{"direnv": true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := filepath.Join(root, "svc")
	if err := os.MkdirAll(svc, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(svc, "cdagee.json"), []byte(`{"targets":{"one":{},"two":{}}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := target.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, tgt := range result.Targets {
		if !tgt.Direnv {
			t.Errorf("%s: expected direnv=true (inherited from root)", tgt.ID)
		}
	}
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
