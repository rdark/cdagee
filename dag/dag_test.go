package dag_test

import (
	"context"
	"errors"
	"iter"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/rdark/cdagee/dag"
)

func mustAddNode[T any](t *testing.T, d *dag.DAG[T], id string, data T) {
	t.Helper()
	if err := d.AddNode(id, data); err != nil {
		t.Fatal(err)
	}
}

func mustAddEdge[T any](t *testing.T, d *dag.DAG[T], parentID, childID string) {
	t.Helper()
	if err := d.AddEdge(parentID, childID); err != nil {
		t.Fatal(err)
	}
}

func build(t *testing.T) *dag.DAG[string] {
	t.Helper()
	d := dag.New[string]()
	for _, id := range []string{"a", "b", "c", "d", "e"} {
		if err := d.AddNode(id, id+"-data"); err != nil {
			t.Fatal(err)
		}
	}
	// a -> b -> d
	//      b -> e
	// a -> c -> e
	for _, edge := range [][2]string{{"a", "b"}, {"a", "c"}, {"b", "d"}, {"b", "e"}, {"c", "e"}} {
		if err := d.AddEdge(edge[0], edge[1]); err != nil {
			t.Fatal(err)
		}
	}
	return d
}

func TestTopologicalSort(t *testing.T) {
	d := build(t)
	order, err := d.TopologicalSort()
	if err != nil {
		t.Fatal(err)
	}
	pos := make(map[string]int, len(order))
	for i, id := range order {
		pos[id] = i
	}
	for _, constraint := range [][2]string{{"a", "b"}, {"a", "c"}, {"b", "d"}, {"b", "e"}, {"c", "e"}} {
		if pos[constraint[0]] >= pos[constraint[1]] {
			t.Errorf("expected %q before %q in %v", constraint[0], constraint[1], order)
		}
	}
}

func TestCanReach(t *testing.T) {
	// build() creates: a→b→d, b→e, a→c→e
	d := build(t)

	tests := []struct {
		from, to string
		want     bool
	}{
		{"a", "e", true},  // a→b→e or a→c→e
		{"a", "d", true},  // a→b→d
		{"b", "e", true},  // b→e
		{"c", "e", true},  // c→e
		{"d", "a", false}, // no reverse path
		{"e", "a", false},
		{"a", "a", true}, // self is trivially reachable
		{"x", "a", false}, // nonexistent node
		{"a", "x", false}, // nonexistent node
	}
	for _, tt := range tests {
		got := d.CanReach(tt.from, tt.to)
		if got != tt.want {
			t.Errorf("CanReach(%q, %q) = %v, want %v", tt.from, tt.to, got, tt.want)
		}
	}
}

func TestCycleDetection(t *testing.T) {
	d := dag.New[string]()
	for _, id := range []string{"x", "y", "z"} {
		if err := d.AddNode(id, id); err != nil {
			t.Fatal(err)
		}
	}
	if err := d.AddEdge("x", "y"); err != nil {
		t.Fatal(err)
	}
	if err := d.AddEdge("y", "z"); err != nil {
		t.Fatal(err)
	}
	if err := d.AddEdge("z", "x"); err == nil {
		t.Fatal("expected cycle error, got nil")
	}
}

func TestSelfLoop(t *testing.T) {
	d := dag.New[string]()
	if err := d.AddNode("a", "a"); err != nil {
		t.Fatal(err)
	}
	if err := d.AddEdge("a", "a"); err == nil {
		t.Fatal("expected self-loop error, got nil")
	}
}

func TestLen(t *testing.T) {
	d := dag.New[string]()
	if d.Len() != 0 {
		t.Fatalf("expected 0, got %d", d.Len())
	}
	if err := d.AddNode("a", "a"); err != nil {
		t.Fatal(err)
	}
	if err := d.AddNode("b", "b"); err != nil {
		t.Fatal(err)
	}
	if d.Len() != 2 {
		t.Fatalf("expected 2, got %d", d.Len())
	}
}

func TestNode(t *testing.T) {
	d := dag.New[string]()
	mustAddNode(t, d, "a", "a-data")
	data, ok := d.Node("a")
	if !ok || data != "a-data" {
		t.Errorf("expected (a-data, true), got (%q, %v)", data, ok)
	}
	_, ok = d.Node("missing")
	if ok {
		t.Error("expected false for missing node")
	}
}

func TestRemoveNode(t *testing.T) {
	d := build(t)
	// Remove b (has parent a, children d and e)
	if err := d.RemoveNode("b"); err != nil {
		t.Fatal(err)
	}
	if d.Len() != 4 {
		t.Fatalf("expected 4 nodes, got %d", d.Len())
	}
	if _, ok := d.Node("b"); ok {
		t.Error("node b should be removed")
	}
	// a should no longer have b as a child; d should have no parents
	order, err := d.TopologicalSort()
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(order, "b") {
		t.Errorf("b should not appear in topo order: %v", order)
	}
	// e should still be reachable via c
	if !slices.Contains(order, "e") {
		t.Errorf("e should still be in graph: %v", order)
	}
	// Removing a non-existent node should error
	if err := d.RemoveNode("b"); err == nil {
		t.Error("expected error removing non-existent node")
	}
}

func TestRemoveEdge(t *testing.T) {
	d := build(t)
	// Remove edge b -> e; e should still be reachable via c
	if err := d.RemoveEdge("b", "e"); err != nil {
		t.Fatal(err)
	}
	order, err := d.TopologicalSort()
	if err != nil {
		t.Fatal(err)
	}
	pos := make(map[string]int, len(order))
	for i, id := range order {
		pos[id] = i
	}
	// c -> e should still hold
	if pos["c"] >= pos["e"] {
		t.Errorf("expected c before e in %v", order)
	}
	// Removing non-existent edge should error
	if err := d.RemoveEdge("b", "e"); err == nil {
		t.Error("expected error removing non-existent edge")
	}
	// Removing edge with non-existent node should error
	if err := d.RemoveEdge("z", "a"); err == nil {
		t.Error("expected error for non-existent node")
	}
}

func TestRemoveNodeInvalidatesPlan(t *testing.T) {
	d := dag.New[string]()
	if err := d.AddNode("a", "a"); err != nil {
		t.Fatal(err)
	}
	if err := d.AddNode("b", "b"); err != nil {
		t.Fatal(err)
	}
	if err := d.AddEdge("a", "b"); err != nil {
		t.Fatal(err)
	}
	if err := d.Freeze(); err != nil {
		t.Fatal(err)
	}

	var seen []string
	if err := d.Execute(context.Background(), func(_ context.Context, id string, _ string, _ iter.Seq2[string, any]) (any, error) {
		seen = append(seen, id)
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 2 {
		t.Fatalf("expected 2, got %v", seen)
	}

	if err := d.RemoveNode("b"); err != nil {
		t.Fatal(err)
	}
	seen = nil
	if err := d.Execute(context.Background(), func(_ context.Context, id string, _ string, _ iter.Seq2[string, any]) (any, error) {
		seen = append(seen, id)
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 1 || seen[0] != "a" {
		t.Fatalf("expected [a], got %v", seen)
	}
}

func TestParents(t *testing.T) {
	// build() creates: a→b→d, b→e, a→c→e
	d := build(t)

	// "e" has parents b and c
	got := d.Parents("e")
	slices.Sort(got)
	if want := []string{"b", "c"}; !slices.Equal(got, want) {
		t.Errorf("Parents(e) = %v, want %v", got, want)
	}

	// "a" is a root — no parents
	got = d.Parents("a")
	if len(got) != 0 {
		t.Errorf("Parents(a) = %v, want empty", got)
	}

	// non-existent node returns nil
	if got := d.Parents("z"); got != nil {
		t.Errorf("Parents(z) = %v, want nil", got)
	}
}

func TestChildren(t *testing.T) {
	// build() creates: a→b→d, b→e, a→c→e
	d := build(t)

	// "a" has children b and c
	got := d.Children("a")
	slices.Sort(got)
	if want := []string{"b", "c"}; !slices.Equal(got, want) {
		t.Errorf("Children(a) = %v, want %v", got, want)
	}

	// "b" has children d and e
	got = d.Children("b")
	slices.Sort(got)
	if want := []string{"d", "e"}; !slices.Equal(got, want) {
		t.Errorf("Children(b) = %v, want %v", got, want)
	}

	// "d" is a leaf — no children
	got = d.Children("d")
	if len(got) != 0 {
		t.Errorf("Children(d) = %v, want empty", got)
	}

	// non-existent node returns nil
	if got := d.Children("z"); got != nil {
		t.Errorf("Children(z) = %v, want nil", got)
	}
}

func TestRootsAndLeaves(t *testing.T) {
	d := build(t)
	roots := d.Roots()
	if !slices.Contains(roots, "a") || len(roots) != 1 {
		t.Errorf("expected roots=[a], got %v", roots)
	}
	leaves := d.Leaves()
	for _, expected := range []string{"d", "e"} {
		if !slices.Contains(leaves, expected) {
			t.Errorf("expected %q in leaves %v", expected, leaves)
		}
	}
}

func TestEdges(t *testing.T) {
	d := build(t)
	type edge struct{ from, to string }
	var got []edge
	for from, to := range d.Edges() {
		got = append(got, edge{from, to})
	}
	// build() creates: a→b, a→c, b→d, b→e, c→e = 5 edges
	if len(got) != 5 {
		t.Fatalf("expected 5 edges, got %d: %v", len(got), got)
	}
	want := []edge{
		{"a", "b"}, {"a", "c"}, {"b", "d"}, {"b", "e"}, {"c", "e"},
	}
	slices.SortFunc(got, func(a, b edge) int {
		if c := strings.Compare(a.from, b.from); c != 0 {
			return c
		}
		return strings.Compare(a.to, b.to)
	})
	for i, e := range want {
		if got[i] != e {
			t.Errorf("edge[%d]: expected %v, got %v", i, e, got[i])
		}
	}
}

func TestAll(t *testing.T) {
	d := build(t)
	seen := make(map[string]string)
	for id, data := range d.All() {
		seen[id] = data
	}
	if len(seen) != 5 {
		t.Errorf("expected 5 nodes, got %d: %v", len(seen), seen)
	}
	if seen["a"] != "a-data" {
		t.Errorf("unexpected data for a: %q", seen["a"])
	}
}

func TestExecuteOrder(t *testing.T) {
	d := build(t)
	var mu sync.Mutex
	var execOrder []string

	err := d.Execute(context.Background(), func(ctx context.Context, id string, data string, _ iter.Seq2[string, any]) (any, error) {
		mu.Lock()
		execOrder = append(execOrder, id)
		mu.Unlock()
		return id + "-result", nil
	})
	if err != nil {
		t.Fatal(err)
	}

	pos := make(map[string]int)
	for i, id := range execOrder {
		pos[id] = i
	}
	for _, constraint := range [][2]string{{"a", "b"}, {"a", "c"}, {"b", "d"}, {"b", "e"}, {"c", "e"}} {
		if pos[constraint[0]] >= pos[constraint[1]] {
			t.Errorf("executed %q after %q (order: %v)", constraint[0], constraint[1], execOrder)
		}
	}
}

func TestExecuteParentResults(t *testing.T) {
	d := dag.New[int]()
	for id, val := range map[string]int{"a": 1, "b": 2, "c": 0} {
		if err := d.AddNode(id, val); err != nil {
			t.Fatal(err)
		}
	}
	if err := d.AddEdge("a", "c"); err != nil {
		t.Fatal(err)
	}
	if err := d.AddEdge("b", "c"); err != nil {
		t.Fatal(err)
	}

	var gotA, gotB int
	if err := d.Execute(context.Background(), func(_ context.Context, id string, data int, parentResults iter.Seq2[string, any]) (any, error) {
		if id == "c" {
			var okA, okB bool
			gotA, okA = dag.ParentAs[int](parentResults, "a")
			gotB, okB = dag.ParentAs[int](parentResults, "b")
			if !okA || !okB {
				t.Errorf("ParentAs failed: okA=%v okB=%v", okA, okB)
			}
		}
		return data * 10, nil
	}); err != nil {
		t.Fatal(err)
	}

	if gotA != 10 || gotB != 20 {
		t.Errorf("unexpected parent results for c: a=%v b=%v", gotA, gotB)
	}
}

func TestExecuteParentResultsIteration(t *testing.T) {
	d := dag.New[int]()
	for id, val := range map[string]int{"a": 1, "b": 2, "c": 0} {
		if err := d.AddNode(id, val); err != nil {
			t.Fatal(err)
		}
	}
	if err := d.AddEdge("a", "c"); err != nil {
		t.Fatal(err)
	}
	if err := d.AddEdge("b", "c"); err != nil {
		t.Fatal(err)
	}

	var collected map[string]any
	if err := d.Execute(context.Background(), func(_ context.Context, id string, data int, parentResults iter.Seq2[string, any]) (any, error) {
		if id == "c" {
			collected = make(map[string]any)
			for pid, val := range parentResults {
				collected[pid] = val
			}
		}
		return data * 10, nil
	}); err != nil {
		t.Fatal(err)
	}

	if collected["a"] != 10 || collected["b"] != 20 {
		t.Errorf("unexpected iterated results: %v", collected)
	}
}

func TestParentAsWrongType(t *testing.T) {
	d := dag.New[string]()
	mustAddNode(t, d, "a", "a")
	mustAddNode(t, d, "b", "b")
	mustAddEdge(t, d, "a", "b")

	if err := d.Execute(context.Background(), func(_ context.Context, id string, _ string, parentResults iter.Seq2[string, any]) (any, error) {
		if id == "b" {
			// parent "a" returned string "hello", try to extract as int
			_, ok := dag.ParentAs[int](parentResults, "a")
			if ok {
				t.Error("ParentAs should return false for wrong type")
			}
			// non-existent parent
			_, ok = dag.ParentAs[string](parentResults, "z")
			if ok {
				t.Error("ParentAs should return false for missing parent")
			}
		}
		return "hello", nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestExecuteErrorCancels(t *testing.T) {
	d := dag.New[string]()
	for _, id := range []string{"a", "b", "c"} {
		mustAddNode(t, d, id, id)
	}
	mustAddEdge(t, d, "a", "b")
	mustAddEdge(t, d, "b", "c")

	boom := errors.New("boom")
	var executed []string
	var mu sync.Mutex

	err := d.Execute(context.Background(), func(ctx context.Context, id string, data string, _ iter.Seq2[string, any]) (any, error) {
		mu.Lock()
		executed = append(executed, id)
		mu.Unlock()
		if id == "a" {
			return nil, boom
		}
		return nil, nil
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, boom) {
		t.Errorf("expected boom in error chain, got %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	for _, skipped := range []string{"b", "c"} {
		if slices.Contains(executed, skipped) {
			t.Errorf("%q should have been skipped after a failed (executed: %v)", skipped, executed)
		}
	}
}

func TestExecuteEmptyGraph(t *testing.T) {
	d := dag.New[string]()
	if err := d.Execute(context.Background(), func(_ context.Context, _ string, _ string, _ iter.Seq2[string, any]) (any, error) {
		t.Fatal("WalkFunc called on empty graph")
		return nil, nil
	}); err != nil {
		t.Fatalf("expected nil on empty graph, got %v", err)
	}
}

func TestExecuteSingleNode(t *testing.T) {
	d := dag.New[string]()
	mustAddNode(t, d, "only", "val")
	var called bool
	err := d.Execute(context.Background(), func(_ context.Context, id string, data string, parentResults iter.Seq2[string, any]) (any, error) {
		called = true
		if id != "only" || data != "val" {
			t.Errorf("unexpected args: id=%q data=%q", id, data)
		}
		for pid := range parentResults {
			t.Errorf("unexpected parent: %q", pid)
		}
		return "result", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("WalkFunc was not called")
	}
}

func TestExecuteCancelledContext(t *testing.T) {
	d := dag.New[string]()
	mustAddNode(t, d, "a", "a")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := d.Execute(ctx, func(_ context.Context, _ string, _ string, _ iter.Seq2[string, any]) (any, error) {
		t.Fatal("WalkFunc should not be called with a pre-cancelled context")
		return nil, nil
	})
	if err == nil {
		t.Fatal("expected error for pre-cancelled context, got nil")
	}
}

func TestExecutePanic(t *testing.T) {
	d := dag.New[string]()
	for _, id := range []string{"a", "b"} {
		mustAddNode(t, d, id, id)
	}
	mustAddEdge(t, d, "a", "b")

	err := d.Execute(context.Background(), func(_ context.Context, id string, _ string, _ iter.Seq2[string, any]) (any, error) {
		if id == "a" {
			panic("something went wrong")
		}
		return nil, nil
	})
	if err == nil {
		t.Fatal("expected error from panic, got nil")
	}
	t.Logf("panic error: %v", err)
}

func TestExecutePanicError(t *testing.T) {
	d := dag.New[string]()
	mustAddNode(t, d, "a", "a")
	underlying := errors.New("underlying")
	err := d.Execute(context.Background(), func(_ context.Context, _ string, _ string, _ iter.Seq2[string, any]) (any, error) {
		panic(underlying)
	})
	if !errors.Is(err, underlying) {
		t.Errorf("expected underlying error in chain, got %v", err)
	}
}

func TestConcurrentExecute(t *testing.T) {
	d := dag.New[string]()
	for _, id := range []string{"a", "b", "c", "d"} {
		mustAddNode(t, d, id, id)
	}
	mustAddEdge(t, d, "a", "b")
	mustAddEdge(t, d, "a", "c")
	mustAddEdge(t, d, "b", "d")
	mustAddEdge(t, d, "c", "d")
	if err := d.Freeze(); err != nil {
		t.Fatal(err)
	}

	const concurrency = 20
	errs := make(chan error, concurrency)
	var wg sync.WaitGroup
	for range concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- d.Execute(context.Background(), func(_ context.Context, _ string, _ string, _ iter.Seq2[string, any]) (any, error) {
				return nil, nil
			})
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("concurrent Execute returned error: %v", err)
		}
	}
}

func TestExecutePlanInvalidation(t *testing.T) {
	d := dag.New[string]()
	mustAddNode(t, d, "a", "a")
	if err := d.Freeze(); err != nil {
		t.Fatal(err)
	}

	var seen []string
	if err := d.Execute(context.Background(), func(_ context.Context, id string, _ string, _ iter.Seq2[string, any]) (any, error) {
		seen = append(seen, id)
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 1 || seen[0] != "a" {
		t.Fatalf("first execute: expected [a], got %v", seen)
	}

	mustAddNode(t, d, "b", "b")
	mustAddEdge(t, d, "a", "b")

	seen = nil
	if err := d.Execute(context.Background(), func(_ context.Context, id string, _ string, _ iter.Seq2[string, any]) (any, error) {
		seen = append(seen, id)
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 2 {
		t.Fatalf("second execute: expected 2 nodes, got %v", seen)
	}
}

// TestExecuteMultiError verifies that independent branch failures are both reported.
func TestExecuteMultiError(t *testing.T) {
	d := dag.New[string]()
	for _, id := range []string{"root", "left", "right", "leaf"} {
		mustAddNode(t, d, id, id)
	}
	mustAddEdge(t, d, "root", "left")
	mustAddEdge(t, d, "root", "right")
	mustAddEdge(t, d, "left", "leaf")
	mustAddEdge(t, d, "right", "leaf")

	errLeft := errors.New("left-broke")
	errRight := errors.New("right-broke")

	err := d.Execute(context.Background(), func(ctx context.Context, id string, _ string, _ iter.Seq2[string, any]) (any, error) {
		switch id {
		case "left":
			return nil, errLeft
		case "right":
			return nil, errRight
		}
		return nil, nil
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, errLeft) {
		t.Errorf("expected errLeft in %v", err)
	}
	if !errors.Is(err, errRight) {
		t.Errorf("expected errRight in %v", err)
	}
}

func TestExecuteTyped(t *testing.T) {
	d := dag.New[int]()
	for id, val := range map[string]int{"a": 1, "b": 2, "c": 0} {
		mustAddNode(t, d, id, val)
	}
	mustAddEdge(t, d, "a", "c")
	mustAddEdge(t, d, "b", "c")

	var cResult int
	err := dag.ExecuteTyped[int, int](d, context.Background(), func(_ context.Context, id string, data int, parentResults iter.Seq2[string, int]) (int, error) {
		sum := data
		for _, val := range parentResults {
			sum += val
		}
		if id == "c" {
			cResult = sum
		}
		return sum, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// a=1, b=2, c=0+(1)+(2)=3
	if cResult != 3 {
		t.Errorf("expected c=3, got %d", cResult)
	}
}

func TestExecuteTypedError(t *testing.T) {
	d := dag.New[string]()
	mustAddNode(t, d, "a", "a")
	mustAddNode(t, d, "b", "b")
	mustAddEdge(t, d, "a", "b")

	boom := errors.New("boom")
	err := dag.ExecuteTyped[string, string](d, context.Background(), func(_ context.Context, id string, _ string, _ iter.Seq2[string, string]) (string, error) {
		if id == "a" {
			return "", boom
		}
		return "ok", nil
	})
	if !errors.Is(err, boom) {
		t.Errorf("expected boom in error chain, got %v", err)
	}
}
