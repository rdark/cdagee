// Package dag provides a generic directed acyclic graph with concurrent execution.
package dag

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"maps"
	"sync"
	"sync/atomic"
)

// DAG is a generic directed acyclic graph.
// T is the type of data stored at each node.
//
// Safe for concurrent use: multiple goroutines may call Execute simultaneously.
// Mutations (AddNode, AddEdge) invalidate the cached execution plan; a plan
// rebuild is triggered on the next Execute or Freeze call.
// Mutating the graph while Execute is running is safe but the in-flight
// execution uses the plan that was current when Execute started.
type DAG[T any] struct {
	mu         sync.RWMutex
	nodes      map[string]T
	children   map[string]map[string]struct{} // node -> set of child IDs
	parents    map[string]map[string]struct{} // node -> set of parent IDs
	generation atomic.Uint64
	plan       atomic.Pointer[execPlan[T]]
}

// execPlan is a pre-computed, immutable execution snapshot.
// Built once per graph generation and reused across Execute calls.
type execPlan[T any] struct {
	generation uint64
	order      []string // topological order
	parents    [][]int  // parents[i] = indices into order of node i's direct parents
	nodes      []T      // data snapshot, indexed by topological position
}

func New[T any]() *DAG[T] {
	return &DAG[T]{
		nodes:    make(map[string]T),
		children: make(map[string]map[string]struct{}),
		parents:  make(map[string]map[string]struct{}),
	}
}

// AddNode adds a node. Returns an error if the ID already exists.
func (d *DAG[T]) AddNode(id string, data T) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, exists := d.nodes[id]; exists {
		return fmt.Errorf("node %q already exists", id)
	}
	d.nodes[id] = data
	d.children[id] = make(map[string]struct{})
	d.parents[id] = make(map[string]struct{})
	d.generation.Add(1)
	return nil
}

// AddEdge adds a directed edge from parent to child.
// Returns an error if either node doesn't exist or if the edge would create a cycle.
func (d *DAG[T]) AddEdge(parentID, childID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.nodes[parentID]; !ok {
		return fmt.Errorf("node %q not found", parentID)
	}
	if _, ok := d.nodes[childID]; !ok {
		return fmt.Errorf("node %q not found", childID)
	}
	if parentID == childID {
		return fmt.Errorf("self-loop on %q", parentID)
	}
	if d.canReach(childID, parentID) {
		return fmt.Errorf("edge %q -> %q would create a cycle", parentID, childID)
	}
	d.children[parentID][childID] = struct{}{}
	d.parents[childID][parentID] = struct{}{}
	d.generation.Add(1)
	return nil
}

// CanReach reports whether from can reach to by following edges.
// A node can always reach itself.
func (d *DAG[T]) CanReach(from, to string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if _, ok := d.nodes[from]; !ok {
		return false
	}
	if _, ok := d.nodes[to]; !ok {
		return false
	}
	return d.canReach(from, to)
}

// canReach returns true if 'from' can reach 'to' via existing edges. Caller must hold mu.
func (d *DAG[T]) canReach(from, to string) bool {
	visited := make(map[string]bool)
	stack := []string{from}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if cur == to {
			return true
		}
		if visited[cur] {
			continue
		}
		visited[cur] = true
		for child := range d.children[cur] {
			stack = append(stack, child)
		}
	}
	return false
}

// Len returns the number of nodes in the graph.
func (d *DAG[T]) Len() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.nodes)
}

// Node returns the data for a node by ID.
// Returns the zero value and false if the node does not exist.
func (d *DAG[T]) Node(id string) (T, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	data, ok := d.nodes[id]
	return data, ok
}

// RemoveNode removes a node and all its edges.
// Returns an error if the node does not exist.
func (d *DAG[T]) RemoveNode(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.nodes[id]; !ok {
		return fmt.Errorf("node %q not found", id)
	}
	for child := range d.children[id] {
		delete(d.parents[child], id)
	}
	for parent := range d.parents[id] {
		delete(d.children[parent], id)
	}
	delete(d.nodes, id)
	delete(d.children, id)
	delete(d.parents, id)
	d.generation.Add(1)
	return nil
}

// RemoveEdge removes a directed edge from parent to child.
// Returns an error if either node does not exist or the edge is not present.
func (d *DAG[T]) RemoveEdge(parentID, childID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.nodes[parentID]; !ok {
		return fmt.Errorf("node %q not found", parentID)
	}
	if _, ok := d.nodes[childID]; !ok {
		return fmt.Errorf("node %q not found", childID)
	}
	if _, ok := d.children[parentID][childID]; !ok {
		return fmt.Errorf("edge %q -> %q not found", parentID, childID)
	}
	delete(d.children[parentID], childID)
	delete(d.parents[childID], parentID)
	d.generation.Add(1)
	return nil
}

// Edges returns an iterator over all (parentID, childID) edge pairs.
// A snapshot is taken at call time; concurrent mutations are not reflected.
func (d *DAG[T]) Edges() iter.Seq2[string, string] {
	d.mu.RLock()
	type edge struct{ from, to string }
	var edges []edge
	for parent, children := range d.children {
		for child := range children {
			edges = append(edges, edge{parent, child})
		}
	}
	d.mu.RUnlock()
	return func(yield func(string, string) bool) {
		for _, e := range edges {
			if !yield(e.from, e.to) {
				return
			}
		}
	}
}

// All returns an iterator over all (id, data) pairs.
// A snapshot is taken at call time; concurrent mutations are not reflected.
func (d *DAG[T]) All() iter.Seq2[string, T] {
	d.mu.RLock()
	snapshot := maps.Clone(d.nodes)
	d.mu.RUnlock()
	return maps.All(snapshot)
}

// Parents returns the IDs of direct parents (dependencies) of the given node.
// Returns nil if the node does not exist.
func (d *DAG[T]) Parents(id string) []string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	ps, ok := d.parents[id]
	if !ok {
		return nil
	}
	out := make([]string, 0, len(ps))
	for p := range ps {
		out = append(out, p)
	}
	return out
}

// Children returns the IDs of direct children (dependents) of the given node.
// Returns nil if the node does not exist.
func (d *DAG[T]) Children(id string) []string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	cs, ok := d.children[id]
	if !ok {
		return nil
	}
	out := make([]string, 0, len(cs))
	for c := range cs {
		out = append(out, c)
	}
	return out
}

// Roots returns nodes with no parents (entry points).
func (d *DAG[T]) Roots() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var roots []string
	for id := range d.nodes {
		if len(d.parents[id]) == 0 {
			roots = append(roots, id)
		}
	}
	return roots
}

// Leaves returns nodes with no children (exit points).
func (d *DAG[T]) Leaves() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var leaves []string
	for id := range d.nodes {
		if len(d.children[id]) == 0 {
			leaves = append(leaves, id)
		}
	}
	return leaves
}

// TopologicalSort returns all nodes in topological order (Kahn's algorithm).
// Nodes earlier in the slice have no dependency on nodes later in the slice.
func (d *DAG[T]) TopologicalSort() ([]string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.topoSortLocked()
}

// topoSortLocked runs Kahn's algorithm. Caller must hold at least d.mu.RLock().
func (d *DAG[T]) topoSortLocked() ([]string, error) {
	inDegree := make(map[string]int, len(d.nodes))
	for id := range d.nodes {
		inDegree[id] = len(d.parents[id])
	}

	var queue []string
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	order := make([]string, 0, len(d.nodes))
	for head := 0; head < len(queue); head++ {
		cur := queue[head]
		order = append(order, cur)
		for child := range d.children[cur] {
			inDegree[child]--
			if inDegree[child] == 0 {
				queue = append(queue, child)
			}
		}
	}

	if len(order) != len(d.nodes) {
		return nil, fmt.Errorf("cycle detected: only %d of %d nodes sorted", len(order), len(d.nodes))
	}
	return order, nil
}

// Freeze pre-builds and caches the execution plan, validating the graph for cycles.
// Calling it after the graph is fully constructed avoids paying the build cost on
// the first Execute call and provides fail-fast cycle detection.
func (d *DAG[T]) Freeze() error {
	_, err := d.loadOrBuildPlan()
	return err
}

// loadOrBuildPlan returns the cached plan if the graph generation is current,
// otherwise builds and caches a new one. The entire build runs under a single
// RLock to guarantee the topo sort and parent-index snapshot are consistent.
//
// Concurrent callers holding RLock may both build a plan; the last Store wins.
// All plans for the same generation are equivalent, so no result is lost.
func (d *DAG[T]) loadOrBuildPlan() (*execPlan[T], error) {
	// Fast path: no lock needed.
	gen := d.generation.Load()
	if p := d.plan.Load(); p != nil && p.generation == gen {
		return p, nil
	}

	// Slow path: hold a single RLock for the entire build so the topo sort and
	// parent-index snapshot are from the same graph state.
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Re-read generation under the lock and double-check the cache; another
	// goroutine may have built the plan while we waited.
	gen = d.generation.Load()
	if p := d.plan.Load(); p != nil && p.generation == gen {
		return p, nil
	}

	order, err := d.topoSortLocked()
	if err != nil {
		return nil, err
	}

	pos := make(map[string]int, len(order))
	for i, id := range order {
		pos[id] = i
	}

	parentIdx := make([][]int, len(order))
	nodes := make([]T, len(order))
	for i, id := range order {
		nodes[i] = d.nodes[id]
		for parentID := range d.parents[id] {
			parentIdx[i] = append(parentIdx[i], pos[parentID])
		}
	}

	p := &execPlan[T]{
		generation: gen,
		order:      order,
		parents:    parentIdx,
		nodes:      nodes,
	}
	d.plan.Store(p)
	return p, nil
}

// WalkFunc is called for each node during Execute.
// parentResults yields (parentID, value) pairs for each direct parent.
// The iterator reads directly from the executor's internal results slice;
// there is no mutable state to corrupt if a reference is retained, but values
// are only meaningful during the current Execute call.
type WalkFunc[T any] func(ctx context.Context, id string, data T, parentResults iter.Seq2[string, any]) (any, error)

// ParentAs extracts a specific parent's result from parentResults with a type
// assertion. Returns the zero value and false if the parent ID is not found or
// the value is not of type R.
func ParentAs[R any](parentResults iter.Seq2[string, any], id string) (R, bool) {
	for pid, val := range parentResults {
		if pid == id {
			r, ok := val.(R)
			return r, ok
		}
	}
	var zero R
	return zero, false
}

type nodeResult struct {
	val     any
	err     error
	skipped bool // fn was never called; not a root-cause error
}

// invokeFn calls fn and recovers any panic, converting it to a returned error.
func invokeFn[T any](fn WalkFunc[T], ctx context.Context, id string, data T, parentResults iter.Seq2[string, any]) (v any, err error) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(error); ok {
				err = fmt.Errorf("panic: %w", e)
			} else {
				err = fmt.Errorf("panic: %v", r)
			}
		}
	}()
	return fn(ctx, id, data, parentResults)
}

// ExecuteTyped is a type-safe variant of Execute for homogeneous pipelines
// where all nodes return the same result type R. It wraps Execute internally,
// providing compile-time type safety for both the return value and parent
// result access.
//
// Because Go methods cannot introduce type parameters beyond those on the
// receiver, this is a package-level function rather than a method on DAG.
func ExecuteTyped[T, R any](d *DAG[T], ctx context.Context, fn func(ctx context.Context, id string, data T, parentResults iter.Seq2[string, R]) (R, error)) error {
	return d.Execute(ctx, func(ctx context.Context, id string, data T, parentResults iter.Seq2[string, any]) (any, error) {
		typed := func(yield func(string, R) bool) {
			for pid, val := range parentResults {
				if !yield(pid, val.(R)) {
					return
				}
			}
		}
		return fn(ctx, id, data, typed)
	})
}

// Execute walks the DAG concurrently in dependency order.
// Each node's WalkFunc is called only after all its parents have completed.
// Parent results are passed downstream as an iterator over (parentID, value) pairs.
// If any node fails or panics, the context is cancelled and all downstream nodes are skipped.
// All root-cause errors (not cascade skips) are returned via errors.Join.
//
// Lock-free result passing: each goroutine writes its nodeResult then closes its done
// channel (defers are LIFO, so close fires after the write). The Go memory model
// guarantees that a channel close happens-before any receive that observes it, so
// child goroutines may read a parent's result after unblocking from <-done[pi]
// without additional synchronisation. wg.Wait() provides the same guarantee for the
// final error sweep.
func (d *DAG[T]) Execute(ctx context.Context, fn WalkFunc[T]) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	plan, err := d.loadOrBuildPlan()
	if err != nil {
		return err
	}

	n := len(plan.order)

	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	// Flat slices indexed by topological position — one allocation each,
	// vs the previous map + n pointer allocations.
	done := make([]chan struct{}, n)
	results := make([]nodeResult, n)
	for i := range n {
		done[i] = make(chan struct{})
	}

	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			defer close(done[i]) // LIFO: fires after results[i] is written below

			for _, pi := range plan.parents[i] {
				select {
				case <-done[pi]:
					// happens-after: safe to read results[pi] without a lock.
					// Propagate both failures and skips so downstream nodes don't execute.
					if results[pi].err != nil || results[pi].skipped {
						results[i].skipped = true
						return
					}
				case <-ctx.Done():
					results[i].skipped = true
					return
				}
			}

			// Build an iterator that reads parent results directly from the
			// results slice. Zero allocations — no map, no pool.
			parentResults := func(yield func(string, any) bool) {
				for _, pi := range plan.parents[i] {
					if !yield(plan.order[pi], results[pi].val) {
						return
					}
				}
			}

			val, err := invokeFn(fn, ctx, plan.order[i], plan.nodes[i], parentResults)

			results[i].val = val // write before close(done[i]) via defer
			results[i].err = err
			if err != nil {
				cancel(fmt.Errorf("node %q: %w", plan.order[i], err))
			}
		})
	}

	wg.Wait() // happens-after all writes to results; no lock needed below

	var errs []error
	for i, id := range plan.order {
		if r := results[i]; !r.skipped && r.err != nil {
			errs = append(errs, fmt.Errorf("node %q: %w", id, r.err))
		}
	}
	return errors.Join(errs...)
}
