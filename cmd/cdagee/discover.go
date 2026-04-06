package main

import (
	"flag"
	"fmt"

	"github.com/rdark/cdagee/target"
)

func runDiscover(args []string) {
	fs := flag.NewFlagSet("discover", flag.ExitOnError)
	cf := addCommonFlags(fs)
	parseTags := addTagsFlag(fs)
	_ = fs.Parse(args) // ExitOnError: exits before returning error
	resolveOutput(cf, fs)
	checkNoArgs(fs)

	rootDir := cf.absRoot()

	result, err := target.Discover(rootDir)
	if err != nil {
		fatalf("discover: %v", err)
	}

	targets := target.FilterByTags(result.Targets, parseTags())

	type tgtJSON struct {
		ID        string   `json:"id"`
		DependsOn []string `json:"depends_on,omitempty"`
		Tags      []string `json:"tags,omitempty"`
		Serial    bool     `json:"serial,omitempty"`
	}
	out := struct {
		Targets []tgtJSON `json:"targets"`
	}{
		Targets: make([]tgtJSON, len(targets)),
	}
	for i, tgt := range targets {
		out.Targets[i] = tgtJSON{
			ID:        tgt.ID,
			DependsOn: tgt.Config.DependsOn,
			Tags:      tgt.Config.Tags,
			Serial:    tgt.Serial,
		}
	}
	if cf.writeOutput(out) {
		return
	}

	for _, tgt := range targets {
		fmt.Println(tgt.ID)
	}
}
