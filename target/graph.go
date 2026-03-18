package target

import (
	"errors"
	"fmt"
	"slices"

	"github.com/rdark/cdagee/dag"
)

// BuildGraph constructs a DAG from the given targets. Each target becomes
// a node; depends_on entries become edges (dependency runs before dependent).
// For targets sharing a directory with Serial=true, chain edges are inserted
// in sorted ID order to ensure they run one at a time.
// All validation errors (duplicate IDs, dangling refs, cycles) are collected
// and returned via errors.Join.
func BuildGraph(targets []Target) (*dag.DAG[Target], error) {
	d := dag.New[Target]()

	var errs []error
	index := make(map[string]struct{}, len(targets))
	added := make(map[int]struct{}, len(targets)) // indices of targets added to the DAG
	for i, tgt := range targets {
		if _, exists := index[tgt.ID]; exists {
			errs = append(errs, &DuplicateIDError{ID: tgt.ID})
			continue
		}
		index[tgt.ID] = struct{}{}
		added[i] = struct{}{}
		if err := d.AddNode(tgt.ID, tgt); err != nil {
			return nil, err
		}
	}

	for i, tgt := range targets {
		if _, ok := added[i]; !ok {
			continue // skip duplicates already reported
		}
		for _, dep := range tgt.Config.DependsOn {
			if _, ok := index[dep]; !ok {
				errs = append(errs, &DanglingRefError{
					Target:    tgt.ID,
					Reference: dep,
				})
				continue
			}
			if err := d.AddEdge(dep, tgt.ID); err != nil {
				errs = append(errs, &CycleError{Err: err})
			}
		}
	}

	// Group serial targets by directory and insert chain edges.
	byDir := map[string][]string{}
	for i, tgt := range targets {
		if _, ok := added[i]; ok && tgt.Serial {
			byDir[tgt.Dir] = append(byDir[tgt.Dir], tgt.ID)
		}
	}
	for _, ids := range byDir {
		if len(ids) < 2 {
			continue
		}
		slices.Sort(ids)
		for j := 0; j < len(ids)-1; j++ {
			a, b := ids[j], ids[j+1]
			if d.CanReach(b, a) || d.CanReach(a, b) {
				continue // already serialized
			}
			if err := d.AddEdge(a, b); err != nil {
				errs = append(errs, fmt.Errorf("serial chain %q -> %q: %w", a, b, err))
			}
		}
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return d, nil
}

// Validate checks that the given targets form a valid DAG with no dangling
// references or cycles.
func Validate(targets []Target) error {
	_, err := BuildGraph(targets)
	return err
}
