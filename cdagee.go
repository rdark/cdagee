// Package cdagee provides a high-level API for discovering targets,
// building a dependency DAG, and computing execution layers.
//
// For full control over individual steps, use the [target] and [dag]
// sub-packages directly.
package cdagee

import (
	"github.com/rdark/cdagee/dag"
	"github.com/rdark/cdagee/target"
)

// Plan is the result of loading and resolving a target tree.
// It holds the discovered targets, the dependency graph, and
// pre-computed concurrency layers.
type Plan struct {
	// Targets is the (possibly tag-filtered) list of targets.
	Targets []target.Target
	// Settings holds root-level configuration from the root cdagee.json.
	Settings target.Settings
	// Graph is the dependency DAG. Callers can use it to call
	// Execute, Parents, Children, Edges, etc.
	Graph *dag.DAG[target.Target]
	// Layers groups target IDs by topological depth.
	// Layer 0 contains root targets; targets within a layer
	// can safely execute concurrently.
	Layers [][]string

	byID map[string]target.Target
}

// Load discovers targets under rootDir, optionally filters by tags,
// builds the dependency graph, and computes execution layers.
//
// If tags are provided, only targets matching at least one tag are
// included (OR semantics). Dependencies referencing filtered-out
// targets are pruned automatically.
func Load(rootDir string, tags ...string) (*Plan, error) {
	dr, err := target.Discover(rootDir)
	if err != nil {
		return nil, err
	}

	targets := target.FilterByTags(dr.Targets, tags)

	g, err := target.BuildGraph(targets)
	if err != nil {
		return nil, err
	}

	layers, err := g.Layers()
	if err != nil {
		return nil, err
	}

	byID := make(map[string]target.Target, len(targets))
	for _, t := range targets {
		byID[t.ID] = t
	}

	return &Plan{
		Targets:  targets,
		Settings: dr.Settings,
		Graph:    g,
		Layers:   layers,
		byID:     byID,
	}, nil
}

// Target returns the target with the given ID and true,
// or the zero value and false if not found.
func (p *Plan) Target(id string) (target.Target, bool) {
	t, ok := p.byID[id]
	return t, ok
}
