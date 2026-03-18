package main

import (
	"flag"
	"fmt"
	"slices"
	"strings"

	"github.com/rdark/cdagee/target"
)

func runPlanOrder(args []string) {
	fs := flag.NewFlagSet("plan-order", flag.ExitOnError)
	cf := addCommonFlags(fs)
	parseTags := addTagsFlag(fs)
	_ = fs.Parse(args) // ExitOnError: exits before returning error
	resolveOutput(cf, fs)
	checkNoArgs(fs)

	result, err := target.Discover(cf.root)
	if err != nil {
		fatalf("plan-order: %v", err)
	}

	targets := target.FilterByTags(result.Targets, parseTags())

	type layerJSON struct {
		Depth   int      `json:"depth"`
		Targets []string `json:"targets"`
	}

	if len(targets) == 0 {
		out := struct {
			Layers []layerJSON `json:"layers"`
		}{Layers: []layerJSON{}}
		if cf.writeOutput(out) {
			return
		}
		return
	}

	g, err := target.BuildGraph(targets)
	if err != nil {
		fatalf("plan-order: %v", err)
	}

	layers, err := g.Layers()
	if err != nil {
		fatalf("plan-order: %v", err)
	}

	// Sort each layer alphabetically for deterministic output
	for _, layer := range layers {
		slices.Sort(layer)
	}

	out := struct {
		Layers []layerJSON `json:"layers"`
	}{
		Layers: make([]layerJSON, len(layers)),
	}
	for i, layer := range layers {
		out.Layers[i] = layerJSON{
			Depth:   i,
			Targets: layer,
		}
	}
	if cf.writeOutput(out) {
		return
	}

	for i, layer := range layers {
		fmt.Printf("Layer %d (concurrent): %s\n", i, strings.Join(layer, ", "))
	}
}
