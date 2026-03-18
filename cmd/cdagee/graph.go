package main

import (
	"flag"
	"fmt"
	"slices"
	"strings"

	"github.com/rdark/cdagee/target"
)

func runGraph(args []string) {
	fs := flag.NewFlagSet("graph", flag.ExitOnError)
	cf := addCommonFlags(fs)
	_ = fs.Parse(args) // ExitOnError: exits before returning error
	resolveOutput(cf, fs)
	checkNoArgs(fs)

	if cf.format != formatText {
		fatalf("graph: structured output is not supported; output is DOT format")
	}

	result, err := target.Discover(cf.root)
	if err != nil {
		fatalf("graph: %v", err)
	}
	targets := result.Targets

	g, err := target.BuildGraph(targets)
	if err != nil {
		fatalf("graph: %v", err)
	}

	fmt.Println("digraph targets {")
	fmt.Println("  rankdir=LR;")

	// Sort for deterministic output
	sorted := slices.Clone(targets)
	slices.SortFunc(sorted, func(a, b target.Target) int {
		return strings.Compare(a.ID, b.ID)
	})

	for _, tgt := range sorted {
		attrs := ""
		if len(tgt.Config.Tags) > 0 {
			attrs = fmt.Sprintf(" [tooltip=%q]", strings.Join(tgt.Config.Tags, ", "))
		}
		fmt.Printf("  %q%s;\n", tgt.ID, attrs)
	}

	// Render edges from the DAG (includes synthetic serial chain edges).
	type edge struct{ from, to string }
	var edges []edge
	for from, to := range g.Edges() {
		edges = append(edges, edge{from, to})
	}
	slices.SortFunc(edges, func(a, b edge) int {
		if c := strings.Compare(a.from, b.from); c != 0 {
			return c
		}
		return strings.Compare(a.to, b.to)
	})
	for _, e := range edges {
		fmt.Printf("  %q -> %q;\n", e.from, e.to)
	}

	fmt.Println("}")
}
