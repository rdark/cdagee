package main

import (
	"fmt"
	"os"
)

const usage = `Usage: cdagee <command> [flags]

Commands:
  discover     List targets found under the root directory
  validate     Check target dependency graph for errors
  graph        Output dependency graph in DOT format
  plan-order   Output concurrency-safe execution layers
  exec         Run a command in each target directory

Flags:
  --root              Root directory to scan (default: current directory)
  --json              Output in JSON format (alias for -o json)
  -o, --output FMT    Output format: text, json, go-template=<tmpl>, go-template-file=<path>
  --tags TAGS         Filter targets by tags (comma-separated, OR matching)
                      Supported by: discover, plan-order, exec
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "discover":
		runDiscover(args)
	case "validate":
		runValidate(args)
	case "graph":
		runGraph(args)
	case "plan-order":
		runPlanOrder(args)
	case "exec":
		runExec(args)
	case "help", "--help", "-h":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", cmd)
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}
}
