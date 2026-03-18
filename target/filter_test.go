package target

import (
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

func TestFilterByTagsNil(t *testing.T) {
	tgts := []Target{
		{ID: "a", Config: TargetConfig{Tags: []string{"x"}}},
		{ID: "b", Config: TargetConfig{Tags: []string{"y"}}},
	}
	got := FilterByTags(tgts, nil)
	if len(got) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(got))
	}
}

func TestFilterByTagsEmptySlice(t *testing.T) {
	tgts := []Target{
		{ID: "a", Config: TargetConfig{Tags: []string{"x"}}},
		{ID: "b", Config: TargetConfig{Tags: []string{"y"}}},
	}
	got := FilterByTags(tgts, []string{})
	if len(got) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(got))
	}
}

func TestFilterByTagsSingleMatch(t *testing.T) {
	tgts := []Target{
		{ID: "a", Config: TargetConfig{Tags: []string{"x"}}},
		{ID: "b", Config: TargetConfig{Tags: []string{"y"}}},
		{ID: "c", Config: TargetConfig{Tags: []string{"x", "y"}}},
	}
	got := FilterByTags(tgts, []string{"x"})
	ids := tgtIDs(got)
	if !slices.Equal(ids, []string{"a", "c"}) {
		t.Errorf("expected [a c], got %v", ids)
	}
}

func TestFilterByTagsMultipleOR(t *testing.T) {
	tgts := []Target{
		{ID: "a", Config: TargetConfig{Tags: []string{"x"}}},
		{ID: "b", Config: TargetConfig{Tags: []string{"y"}}},
		{ID: "c", Config: TargetConfig{Tags: []string{"z"}}},
	}
	got := FilterByTags(tgts, []string{"x", "y"})
	ids := tgtIDs(got)
	if !slices.Equal(ids, []string{"a", "b"}) {
		t.Errorf("expected [a b], got %v", ids)
	}
}

func TestFilterByTagsNoMatches(t *testing.T) {
	tgts := []Target{
		{ID: "a", Config: TargetConfig{Tags: []string{"x"}}},
	}
	got := FilterByTags(tgts, []string{"nope"})
	if len(got) != 0 {
		t.Errorf("expected 0 targets, got %d", len(got))
	}
}

func TestFilterByTagsStripsDependsOn(t *testing.T) {
	tgts := []Target{
		{ID: "a", Config: TargetConfig{Tags: []string{"x"}}},
		{ID: "b", Config: TargetConfig{Tags: []string{"x"}, DependsOn: []string{"a", "c"}}},
		{ID: "c", Config: TargetConfig{Tags: []string{"y"}, DependsOn: []string{"a"}}},
	}
	got := FilterByTags(tgts, []string{"x"})
	if len(got) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(got))
	}
	// b should have "c" stripped from depends_on since c is filtered out
	for _, tgt := range got {
		if tgt.ID == "b" {
			if !slices.Equal(tgt.Config.DependsOn, []string{"a"}) {
				t.Errorf("b depends_on: expected [a], got %v", tgt.Config.DependsOn)
			}
		}
	}
}

func TestFilterByTagsDoesNotMutateInput(t *testing.T) {
	tgts := []Target{
		{ID: "a", Config: TargetConfig{Tags: []string{"x"}}},
		{ID: "b", Config: TargetConfig{Tags: []string{"x"}, DependsOn: []string{"a", "c"}}},
		{ID: "c", Config: TargetConfig{Tags: []string{"y"}}},
	}
	origDeps := make([]string, len(tgts[1].Config.DependsOn))
	copy(origDeps, tgts[1].Config.DependsOn)

	_ = FilterByTags(tgts, []string{"x"})

	if !slices.Equal(tgts[1].Config.DependsOn, origDeps) {
		t.Errorf("input mutated: expected %v, got %v", origDeps, tgts[1].Config.DependsOn)
	}
}

func TestFilterByTagsNoTags(t *testing.T) {
	tgts := []Target{
		{ID: "a"},
		{ID: "b", Config: TargetConfig{Tags: []string{"x"}}},
	}
	got := FilterByTags(tgts, []string{"x"})
	ids := tgtIDs(got)
	if !slices.Equal(ids, []string{"b"}) {
		t.Errorf("expected [b], got %v", ids)
	}
}

func filterTestdataDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file path")
	}
	return filepath.Join(filepath.Dir(file), "testdata")
}

func TestFilterByTagsFromFixture(t *testing.T) {
	root := filepath.Join(filterTestdataDir(t), "tagged")
	result, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	targets := result.Targets

	// --tags deploy → infra, app, db (3 targets)
	got := FilterByTags(targets, []string{"deploy"})
	ids := tgtIDs(got)
	if !slices.Equal(ids, []string{"app", "db", "infra"}) {
		t.Errorf("--tags deploy: expected [app db infra], got %v", ids)
	}

	// app depends_on infra (kept), db depends_on infra (kept) — no stripping needed
	for _, tgt := range got {
		if tgt.ID == "app" {
			if !slices.Equal(tgt.Config.DependsOn, []string{"infra"}) {
				t.Errorf("app depends_on: expected [infra], got %v", tgt.Config.DependsOn)
			}
		}
		if tgt.ID == "db" {
			if !slices.Equal(tgt.Config.DependsOn, []string{"infra"}) {
				t.Errorf("db depends_on: expected [infra], got %v", tgt.Config.DependsOn)
			}
		}
	}

	// --tags ci → ci alone; deps on app/db stripped, ci becomes a root
	got = FilterByTags(targets, []string{"ci"})
	ids = tgtIDs(got)
	if !slices.Equal(ids, []string{"ci"}) {
		t.Errorf("--tags ci: expected [ci], got %v", ids)
	}
	if len(got[0].Config.DependsOn) != 0 {
		t.Errorf("ci depends_on: expected empty (stripped), got %v", got[0].Config.DependsOn)
	}

	// --tags nope → empty set
	got = FilterByTags(targets, []string{"nope"})
	if len(got) != 0 {
		t.Errorf("--tags nope: expected 0, got %d", len(got))
	}
}

func TestFilterByTagsMultiFromFixture(t *testing.T) {
	root := filepath.Join(filterTestdataDir(t), "tagged")
	result, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	targets := result.Targets

	// --tags deploy,test → infra, app, db, monitoring (4 targets, OR semantics)
	got := FilterByTags(targets, []string{"deploy", "test"})
	ids := tgtIDs(got)
	if !slices.Equal(ids, []string{"app", "db", "infra", "monitoring"}) {
		t.Errorf("--tags deploy,test: expected [app db infra monitoring], got %v", ids)
	}

	// --tags test → app, monitoring (2 targets)
	// app's dep on infra stripped (infra not tagged "test")
	got = FilterByTags(targets, []string{"test"})
	ids = tgtIDs(got)
	if !slices.Equal(ids, []string{"app", "monitoring"}) {
		t.Errorf("--tags test: expected [app monitoring], got %v", ids)
	}
	for _, tgt := range got {
		if tgt.ID == "app" {
			if len(tgt.Config.DependsOn) != 0 {
				t.Errorf("app depends_on: expected empty (infra stripped), got %v", tgt.Config.DependsOn)
			}
		}
	}
}

func tgtIDs(tgts []Target) []string {
	ids := make([]string, len(tgts))
	for i, tgt := range tgts {
		ids[i] = tgt.ID
	}
	return ids
}
