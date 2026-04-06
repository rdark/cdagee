package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"iter"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/rdark/cdagee/target"
)

// execResult holds the outcome of running a command in one target.
type execResult struct {
	Target     string `json:"target"`
	TargetName string `json:"target_name,omitempty"`
	ExitCode   int    `json:"exit_code"`
	Output     string `json:"output"`
	Skipped    bool   `json:"skipped,omitempty"`
	DurationMs int64  `json:"duration_ms,omitempty"`
}

func runExec(args []string) {
	fs := flag.NewFlagSet("exec", flag.ExitOnError)
	cf := addCommonFlags(fs)
	parseTags := addTagsFlag(fs)

	var concurrency int
	fs.IntVar(&concurrency, "concurrency", 0, "max parallel commands (0 = unlimited)")
	var continueOnError bool
	fs.BoolVar(&continueOnError, "continue-on-error", false, "keep running independent branches after a failure")
	var stream bool
	fs.BoolVar(&stream, "stream", false, "stream command output in real-time with [target] prefixes (text mode only)")

	_ = fs.Parse(args) // ExitOnError: exits before returning error
	resolveOutput(cf, fs)

	if stream && cf.format != formatText {
		fatalf("exec: --stream cannot be combined with --json or non-text output formats")
	}

	cmdArgs := fs.Args()
	if len(cmdArgs) == 0 {
		fatalf("exec: no command specified")
	}

	if concurrency < 0 {
		fatalf("exec: --concurrency must be >= 0")
	}

	// Discover -> Filter -> BuildGraph.
	rootDir := cf.absRoot()

	result, err := target.Discover(rootDir)
	if err != nil {
		fatalf("exec: %v", err)
	}

	targets := target.FilterByTags(result.Targets, parseTags())
	if len(targets) == 0 {
		out := struct {
			Results []execResult `json:"results"`
		}{Results: []execResult{}}
		cf.writeOutput(out)
		return
	}

	g, err := target.BuildGraph(targets)
	if err != nil {
		fatalf("exec: %v", err)
	}

	// If any target uses direnv, verify the binary is available.
	needsDirenv := false
	for _, tgt := range targets {
		if tgt.Direnv {
			needsDirenv = true
			break
		}
	}
	if needsDirenv {
		if _, lookErr := exec.LookPath("direnv"); lookErr != nil {
			fatalf("exec: direnv is required but not found on PATH")
		}
	}

	// Concurrency semaphore: nil means unlimited.
	var sem chan struct{}
	if concurrency > 0 {
		sem = make(chan struct{}, concurrency)
	}

	var (
		mu      sync.Mutex
		results []execResult
	)

	ctx := context.Background()

	walkErr := g.Execute(ctx, func(ctx context.Context, id string, tgt target.Target, parentResults iter.Seq2[string, any]) (any, error) {
		// In continue-on-error mode, check if any parent failed or was skipped.
		if continueOnError {
			for _, val := range parentResults {
				if r, ok := val.(*execResult); ok && (r.ExitCode != 0 || r.Skipped) {
					result := &execResult{
						Target:  id,
						Skipped: true,
					}
					mu.Lock()
					results = append(results, *result)
					if cf.format == formatText {
						fmt.Printf("--- %s: SKIPPED (dependency failed) ---\n\n", id)
					}
					mu.Unlock()
					return result, nil
				}
			}
		}

		// Bail out early if the context has been cancelled (e.g. a sibling
		// failed in fail-fast mode) to avoid spawning a doomed process.
		if err := ctx.Err(); err != nil {
			result := &execResult{
				Target:  id,
				Skipped: true,
			}
			mu.Lock()
			results = append(results, *result)
			mu.Unlock()
			return result, err
		}

		// Acquire semaphore.
		if sem != nil {
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				result := &execResult{
					Target:  id,
					Skipped: true,
				}
				mu.Lock()
				results = append(results, *result)
				mu.Unlock()
				return result, ctx.Err()
			}
			defer func() { <-sem }()
		}

		// Run the command, wrapping with direnv exec if enabled.
		start := time.Now()
		runArgs := cmdArgs
		if tgt.Direnv {
			runArgs = append([]string{"direnv", "exec", "."}, cmdArgs...)
		}
		cmd := exec.CommandContext(ctx, runArgs[0], runArgs[1:]...)
		cmd.Dir = tgt.Dir
		env := append(os.Environ(), "CDAGEE_TARGET="+id)
		if name := targetName(id); name != "" {
			env = append(env, "CDAGEE_TARGET_NAME="+name)
		}
		cmd.Env = env

		var buf bytes.Buffer
		var pw *prefixWriter
		if stream {
			pw = newPrefixWriter(id, os.Stdout, &mu)
			w := io.MultiWriter(pw, &buf)
			cmd.Stdout = w
			cmd.Stderr = w
		} else {
			cmd.Stdout = &buf
			cmd.Stderr = &buf
		}

		cmdErr := cmd.Run()
		if pw != nil {
			_ = pw.Flush()
		}
		duration := time.Since(start)

		exitCode := 0
		if cmdErr != nil {
			if exitErr, ok := cmdErr.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				// Command failed to start or other error.
				exitCode = 1
			}
		}

		result := &execResult{
			Target:     id,
			ExitCode:   exitCode,
			Output:     buf.String(),
			DurationMs: duration.Milliseconds(),
		}
		if name := targetName(id); name != "" {
			result.TargetName = name
		}

		mu.Lock()
		results = append(results, *result)
		if cf.format == formatText {
			if exitCode != 0 {
				fmt.Printf("--- %s: FAILED exit %d (%s) ---\n", id, exitCode, fmtDuration(duration))
			} else {
				fmt.Printf("--- %s (%s) ---\n", id, fmtDuration(duration))
			}
			if !stream {
				if output := buf.String(); output != "" {
					fmt.Print(output)
					if !strings.HasSuffix(output, "\n") {
						fmt.Println()
					}
				}
			}
			fmt.Println()
		}
		mu.Unlock()

		if continueOnError {
			return result, nil
		}

		if exitCode != 0 {
			return result, fmt.Errorf("command failed with exit code %d", exitCode)
		}
		return result, nil
	})

	// Structured output: print collected results.
	out := struct {
		Results []execResult `json:"results"`
	}{Results: results}
	cf.writeOutput(out)

	if continueOnError {
		for _, r := range results {
			if r.ExitCode != 0 || r.Skipped {
				os.Exit(2)
			}
		}
		return
	}

	if walkErr != nil {
		os.Exit(1)
	}
}

func targetName(id string) string {
	if i := strings.LastIndex(id, ":"); i >= 0 {
		return id[i+1:]
	}
	return ""
}

func fmtDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}
