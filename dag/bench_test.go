package dag_test

import (
	"context"
	"iter"
	"strconv"
	"testing"

	"github.com/rdark/cdagee/dag"
)

// --- topology builders ---

// chainDAG builds a fully sequential graph: 0 → 1 → … → n-1.
// No parallelism is possible; useful for measuring per-node scheduler overhead.
func chainDAG(tb testing.TB, n int) *dag.DAG[int] {
	tb.Helper()
	d := dag.New[int]()
	for i := range n {
		if err := d.AddNode(strconv.Itoa(i), i); err != nil {
			tb.Fatal(err)
		}
	}
	for i := range n - 1 {
		if err := d.AddEdge(strconv.Itoa(i), strconv.Itoa(i+1)); err != nil {
			tb.Fatal(err)
		}
	}
	return d
}

// wideDAG builds a maximally parallel graph: root → {0…n-1} → leaf.
// All n middle nodes can execute concurrently.
func wideDAG(tb testing.TB, n int) *dag.DAG[int] {
	tb.Helper()
	d := dag.New[int]()
	if err := d.AddNode("root", 0); err != nil {
		tb.Fatal(err)
	}
	if err := d.AddNode("leaf", 0); err != nil {
		tb.Fatal(err)
	}
	for i := range n {
		id := strconv.Itoa(i)
		if err := d.AddNode(id, i); err != nil {
			tb.Fatal(err)
		}
		if err := d.AddEdge("root", id); err != nil {
			tb.Fatal(err)
		}
		if err := d.AddEdge(id, "leaf"); err != nil {
			tb.Fatal(err)
		}
	}
	return d
}

// treeDAG builds a complete binary tree of the given depth (2^depth - 1 nodes).
// Parallelism is layered: each level can run concurrently within itself.
func treeDAG(tb testing.TB, depth int) *dag.DAG[int] {
	tb.Helper()
	d := dag.New[int]()
	total := (1 << depth) - 1
	for i := range total {
		if err := d.AddNode(strconv.Itoa(i), i); err != nil {
			tb.Fatal(err)
		}
	}
	for i := range total {
		if left := 2*i + 1; left < total {
			if err := d.AddEdge(strconv.Itoa(i), strconv.Itoa(left)); err != nil {
				tb.Fatal(err)
			}
		}
		if right := 2*i + 2; right < total {
			if err := d.AddEdge(strconv.Itoa(i), strconv.Itoa(right)); err != nil {
				tb.Fatal(err)
			}
		}
	}
	return d
}

var noopWalk = func(_ context.Context, _ string, _ int, _ iter.Seq2[string, any]) (any, error) {
	return nil, nil
}

// --- benchmarks ---

// BenchmarkBuild measures the full cost of constructing a DAG (AddNode + AddEdge),
// including cycle detection on each edge addition.
func BenchmarkBuild(b *testing.B) {
	for _, n := range []int{10, 100, 1000} {
		b.Run("chain/"+strconv.Itoa(n), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				chainDAG(b, n)
			}
		})
		b.Run("wide/"+strconv.Itoa(n), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				wideDAG(b, n)
			}
		})
	}
}

// BenchmarkTopologicalSort measures Kahn's algorithm on pre-built graphs.
func BenchmarkTopologicalSort(b *testing.B) {
	for _, n := range []int{10, 100, 1000} {
		chain := chainDAG(b, n)
		b.Run("chain/"+strconv.Itoa(n), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				if _, err := chain.TopologicalSort(); err != nil {
					b.Fatal(err)
				}
			}
		})

		wide := wideDAG(b, n)
		b.Run("wide/"+strconv.Itoa(n), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				if _, err := wide.TopologicalSort(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkExecute measures the concurrent execution engine with a no-op WalkFunc,
// isolating scheduler and synchronisation overhead from actual work.
// The "cold" sub-benchmarks invalidate the plan cache on each iteration by
// rebuilding the DAG, showing total cost including plan construction.
// The "warm" sub-benchmarks call Freeze() once before the loop, showing the
// steady-state cost when the graph is stable across many Execute calls.
func BenchmarkExecute(b *testing.B) {
	ctx := context.Background()

	for _, n := range []int{10, 100, 1000, 10000} {
		b.Run("cold/chain/"+strconv.Itoa(n), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				if err := chainDAG(b, n).Execute(ctx, noopWalk); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run("cold/wide/"+strconv.Itoa(n), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				if err := wideDAG(b, n).Execute(ctx, noopWalk); err != nil {
					b.Fatal(err)
				}
			}
		})

		chain := chainDAG(b, n)
		if err := chain.Freeze(); err != nil {
			b.Fatal(err)
		}
		b.Run("warm/chain/"+strconv.Itoa(n), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				if err := chain.Execute(ctx, noopWalk); err != nil {
					b.Fatal(err)
				}
			}
		})

		wide := wideDAG(b, n)
		if err := wide.Freeze(); err != nil {
			b.Fatal(err)
		}
		b.Run("warm/wide/"+strconv.Itoa(n), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				if err := wide.Execute(ctx, noopWalk); err != nil {
					b.Fatal(err)
				}
			}
		})
	}

	for _, depth := range []int{4, 8, 14} { // 15, 255, and 16383 nodes
		tree := treeDAG(b, depth)
		if err := tree.Freeze(); err != nil {
			b.Fatal(err)
		}
		b.Run("warm/tree/depth"+strconv.Itoa(depth), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				if err := tree.Execute(ctx, noopWalk); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
