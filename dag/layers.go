package dag

import "fmt"

// Layers groups nodes by topological depth (concurrency layers).
// Layer 0 contains root nodes (no parents), layer N contains nodes whose
// parents are all in layers 0..N-1. Nodes within a layer can safely execute
// concurrently. Returns an error if the graph contains a cycle.
func (d *DAG[T]) Layers() ([][]string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.layersLocked()
}

// layersLocked implements Layers. Caller must hold at least d.mu.RLock().
func (d *DAG[T]) layersLocked() ([][]string, error) {
	if len(d.nodes) == 0 {
		return nil, nil
	}

	inDegree := make(map[string]int, len(d.nodes))
	depth := make(map[string]int, len(d.nodes))
	for id := range d.nodes {
		inDegree[id] = len(d.parents[id])
	}

	var queue []string
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
			depth[id] = 0
		}
	}

	maxDepth := 0
	sorted := 0
	for head := 0; head < len(queue); head++ {
		cur := queue[head]
		sorted++
		for child := range d.children[cur] {
			newDepth := depth[cur] + 1
			if newDepth > depth[child] {
				depth[child] = newDepth
			}
			inDegree[child]--
			if inDegree[child] == 0 {
				queue = append(queue, child)
				if depth[child] > maxDepth {
					maxDepth = depth[child]
				}
			}
		}
	}

	if sorted != len(d.nodes) {
		return nil, fmt.Errorf("cycle detected: only %d of %d nodes sorted", sorted, len(d.nodes))
	}

	layers := make([][]string, maxDepth+1)
	for id, lvl := range depth {
		layers[lvl] = append(layers[lvl], id)
	}
	return layers, nil
}
