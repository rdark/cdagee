package main_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var binary string

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "cdagee-test-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmp) //nolint:errcheck // best-effort cleanup

	binary = filepath.Join(tmp, "cdagee")
	cmd := exec.Command("go", "build", "-o", binary, ".")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic("failed to build cdagee: " + err.Error())
	}

	os.Exit(m.Run())
}

// testdataDir returns the absolute path to target/testdata.
func testdataDir(t *testing.T) string {
	t.Helper()
	// The test runs from cmd/cdagee/, so go up two levels to repo root.
	abs, err := filepath.Abs("../../target/testdata")
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

func run(t *testing.T, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command(binary, args...)
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	exitCode = 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("exec error: %v", err)
		}
	}
	return outBuf.String(), errBuf.String(), exitCode
}

// --- no-args / help ---

func TestNoArgs(t *testing.T) {
	_, stderr, code := run(t)
	if code != 1 {
		t.Errorf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr, "Usage:") {
		t.Errorf("expected usage in stderr, got %q", stderr)
	}
}

func TestHelp(t *testing.T) {
	stdout, _, code := run(t, "help")
	if code != 0 {
		t.Errorf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "Usage:") {
		t.Errorf("expected usage in stdout, got %q", stdout)
	}
}

func TestUnknownCommand(t *testing.T) {
	_, stderr, code := run(t, "bogus")
	if code != 1 {
		t.Errorf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr, "unknown command") {
		t.Errorf("expected 'unknown command' in stderr, got %q", stderr)
	}
}

// --- discover ---

func TestDiscoverText(t *testing.T) {
	root := filepath.Join(testdataDir(t), "simple")
	stdout, _, code := run(t, "discover", "--root", root)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	lines := nonEmptyLines(stdout)
	if len(lines) != 2 || lines[0] != "ws-a" || lines[1] != "ws-b" {
		t.Errorf("unexpected output: %v", lines)
	}
}

func TestDiscoverJSON(t *testing.T) {
	root := filepath.Join(testdataDir(t), "diamond")
	stdout, _, code := run(t, "discover", "--root", root, "--json")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	var out struct {
		Targets []struct {
			ID        string   `json:"id"`
			DependsOn []string `json:"depends_on"`
		} `json:"targets"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(out.Targets) != 4 {
		t.Errorf("expected 4 targets, got %d", len(out.Targets))
	}
}

func TestDiscoverEmpty(t *testing.T) {
	root := filepath.Join(testdataDir(t), "empty")
	stdout, _, code := run(t, "discover", "--root", root)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("expected empty output, got %q", stdout)
	}
}

func TestDiscoverBadRoot(t *testing.T) {
	_, _, code := run(t, "discover", "--root", "/nonexistent/path")
	if code == 0 {
		t.Error("expected non-zero exit for bad root")
	}
}

func TestDiscoverUnexpectedArgs(t *testing.T) {
	_, stderr, code := run(t, "discover", "extra-arg")
	if code != 1 {
		t.Errorf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr, "unexpected arguments") {
		t.Errorf("expected 'unexpected arguments' in stderr, got %q", stderr)
	}
}

// --- validate ---

func TestValidateOK(t *testing.T) {
	root := filepath.Join(testdataDir(t), "simple")
	stdout, _, code := run(t, "validate", "--root", root)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if strings.TrimSpace(stdout) != "ok" {
		t.Errorf("expected 'ok', got %q", stdout)
	}
}

func TestValidateCycle(t *testing.T) {
	root := filepath.Join(testdataDir(t), "cycle")
	_, stderr, code := run(t, "validate", "--root", root)
	if code == 0 {
		t.Fatal("expected non-zero exit for cycle")
	}
	if !strings.Contains(stderr, "cycle") {
		t.Errorf("expected 'cycle' in stderr, got %q", stderr)
	}
}

func TestValidateDanglingJSON(t *testing.T) {
	root := filepath.Join(testdataDir(t), "dangling")
	stdout, _, code := run(t, "validate", "--root", root, "--json")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	var out struct {
		Valid  bool     `json:"valid"`
		Errors []string `json:"errors"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if out.Valid {
		t.Error("expected valid=false")
	}
	if len(out.Errors) == 0 {
		t.Error("expected at least one error")
	}
}

func TestValidateUnexpectedArgs(t *testing.T) {
	_, stderr, code := run(t, "validate", "extra-arg")
	if code != 1 {
		t.Errorf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr, "unexpected arguments") {
		t.Errorf("expected 'unexpected arguments' in stderr, got %q", stderr)
	}
}

// --- graph ---

func TestGraphDOT(t *testing.T) {
	root := filepath.Join(testdataDir(t), "diamond")
	stdout, _, code := run(t, "graph", "--root", root)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "digraph targets {") {
		t.Errorf("expected DOT header, got %q", stdout)
	}
	if !strings.Contains(stdout, "rankdir=LR") {
		t.Errorf("expected rankdir=LR in DOT output")
	}
	// Check edges
	if !strings.Contains(stdout, `"root" -> "left"`) {
		t.Errorf("expected root->left edge in DOT output")
	}
}

func TestGraphJSONNotSupported(t *testing.T) {
	root := filepath.Join(testdataDir(t), "simple")
	_, stderr, code := run(t, "graph", "--root", root, "--json")
	if code == 0 {
		t.Fatal("expected non-zero exit for --json with graph")
	}
	if !strings.Contains(stderr, "not supported") {
		t.Errorf("expected 'not supported' in stderr, got %q", stderr)
	}
}

func TestGraphCycleError(t *testing.T) {
	root := filepath.Join(testdataDir(t), "cycle")
	_, _, code := run(t, "graph", "--root", root)
	if code == 0 {
		t.Fatal("expected non-zero exit for cycle")
	}
}

func TestGraphSerial(t *testing.T) {
	root := filepath.Join(testdataDir(t), "serial")
	stdout, _, code := run(t, "graph", "--root", root)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	// Serial chain edges are synthetic (not in Config.DependsOn).
	// The graph output should include them.
	if !strings.Contains(stdout, `"svc:alpha" -> "svc:beta"`) {
		t.Errorf("expected serial chain edge svc:alpha -> svc:beta in DOT output:\n%s", stdout)
	}
	if !strings.Contains(stdout, `"svc:beta" -> "svc:gamma"`) {
		t.Errorf("expected serial chain edge svc:beta -> svc:gamma in DOT output:\n%s", stdout)
	}
}

func TestGraphEdges(t *testing.T) {
	root := t.TempDir()
	writeCdageeJSON(t, root, "networking", `{}`)
	writeCdageeJSON(t, root, "app", `{"depends_on":["networking"]}`)
	writeCdageeJSON(t, root, "database", `{"depends_on":["networking"]}`)

	stdout, _, code := run(t, "graph", "--root", root)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	if !strings.Contains(stdout, `"networking" -> "app"`) {
		t.Errorf("expected networking -> app edge, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, `"networking" -> "database"`) {
		t.Errorf("expected networking -> database edge, got:\n%s", stdout)
	}
}

func TestGraphUnexpectedArgs(t *testing.T) {
	_, stderr, code := run(t, "graph", "extra-arg")
	if code != 1 {
		t.Errorf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr, "unexpected arguments") {
		t.Errorf("expected 'unexpected arguments' in stderr, got %q", stderr)
	}
}

// --- plan-order ---

func TestPlanOrderText(t *testing.T) {
	root := filepath.Join(testdataDir(t), "diamond")
	stdout, _, code := run(t, "plan-order", "--root", root)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	lines := nonEmptyLines(stdout)
	if len(lines) != 3 {
		t.Fatalf("expected 3 layers, got %d: %v", len(lines), lines)
	}
	if !strings.Contains(lines[0], "root") {
		t.Errorf("expected root in layer 0: %s", lines[0])
	}
	if !strings.Contains(lines[1], "left") || !strings.Contains(lines[1], "right") {
		t.Errorf("expected left,right in layer 1: %s", lines[1])
	}
	if !strings.Contains(lines[2], "leaf") {
		t.Errorf("expected leaf in layer 2: %s", lines[2])
	}
}

func TestPlanOrderJSON(t *testing.T) {
	root := filepath.Join(testdataDir(t), "diamond")
	stdout, _, code := run(t, "plan-order", "--root", root, "--json")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	var out struct {
		Layers []struct {
			Depth   int      `json:"depth"`
			Targets []string `json:"targets"`
		} `json:"layers"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(out.Layers) != 3 {
		t.Fatalf("expected 3 layers, got %d", len(out.Layers))
	}
	if out.Layers[0].Depth != 0 {
		t.Errorf("expected depth 0 for first layer, got %d", out.Layers[0].Depth)
	}
}

func TestPlanOrderUnexpectedArgs(t *testing.T) {
	_, stderr, code := run(t, "plan-order", "extra-arg")
	if code != 1 {
		t.Errorf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr, "unexpected arguments") {
		t.Errorf("expected 'unexpected arguments' in stderr, got %q", stderr)
	}
}

// --- exec ---

func TestExecBasic(t *testing.T) {
	root := filepath.Join(testdataDir(t), "simple")
	stdout, _, code := run(t, "exec", "--root", root, "--", "echo", "hello")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "ws-a") {
		t.Errorf("expected ws-a in output, got %q", stdout)
	}
	if !strings.Contains(stdout, "ws-b") {
		t.Errorf("expected ws-b in output, got %q", stdout)
	}
	if !strings.Contains(stdout, "hello") {
		t.Errorf("expected 'hello' in output, got %q", stdout)
	}
}

func TestExecJSON(t *testing.T) {
	root := filepath.Join(testdataDir(t), "diamond")
	stdout, _, code := run(t, "exec", "--root", root, "--json", "--", "echo", "ok")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	var out struct {
		Results []struct {
			Target     string `json:"target"`
			ExitCode   int    `json:"exit_code"`
			Output     string `json:"output"`
			DurationMs int64  `json:"duration_ms"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(out.Results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(out.Results))
	}
	for _, r := range out.Results {
		if r.ExitCode != 0 {
			t.Errorf("target %s: expected exit 0, got %d", r.Target, r.ExitCode)
		}
		if !strings.Contains(r.Output, "ok") {
			t.Errorf("target %s: expected 'ok' in output, got %q", r.Target, r.Output)
		}
	}
}

func TestExecFailFast(t *testing.T) {
	// Two independent targets; ws-fail always exits 1.
	// Execution order is nondeterministic (both are roots), but either way
	// the process exits 1: ws-fail's error triggers fail-fast.
	root := t.TempDir()
	writeCdageeJSON(t, root, "ws-pass", `{"tags":["a"]}`)
	writeCdageeJSON(t, root, "ws-fail", `{"tags":["a"]}`)

	_, _, code := run(t, "exec", "--root", root, "--concurrency", "1", "--", "sh", "-c",
		`if [ "$(basename "$PWD")" = "ws-fail" ]; then exit 1; fi; echo ok`)
	if code != 1 {
		t.Errorf("expected exit 1, got %d", code)
	}
}

func TestExecContinueOnError(t *testing.T) {
	// forest fixture: tree1-root, tree1-leaf (depends tree1-root),
	// tree2-root, tree2-leaf (depends tree2-root).
	// Fail tree1-root → tree1-leaf should be skipped, tree2-* should complete.
	root := filepath.Join(testdataDir(t), "forest")

	stdout, _, code := run(t, "exec", "--root", root, "--json", "--continue-on-error", "--",
		"sh", "-c", `if [ "$(basename "$PWD")" = "tree1-root" ]; then exit 1; fi; echo ok`)
	if code != 2 {
		t.Errorf("expected exit 2, got %d", code)
	}

	var out struct {
		Results []struct {
			Target   string `json:"target"`
			ExitCode int    `json:"exit_code"`
			Skipped  bool   `json:"skipped"`
			Output   string `json:"output"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}

	resultMap := make(map[string]struct {
		ExitCode int
		Skipped  bool
		Output   string
	})
	for _, r := range out.Results {
		resultMap[r.Target] = struct {
			ExitCode int
			Skipped  bool
			Output   string
		}{r.ExitCode, r.Skipped, r.Output}
	}

	if r, ok := resultMap["tree1-root"]; !ok || r.ExitCode == 0 {
		t.Errorf("tree1-root: expected failure, got %+v", r)
	}
	if r, ok := resultMap["tree1-leaf"]; !ok || !r.Skipped {
		t.Errorf("tree1-leaf: expected skipped, got %+v", r)
	}
	if r, ok := resultMap["tree2-root"]; !ok || r.ExitCode != 0 {
		t.Errorf("tree2-root: expected success, got %+v", r)
	}
	if r, ok := resultMap["tree2-leaf"]; !ok || r.ExitCode != 0 {
		t.Errorf("tree2-leaf: expected success, got %+v", r)
	}
}

func TestExecTags(t *testing.T) {
	// tagged fixture: infra(deploy), app(deploy,test), db(deploy),
	// monitoring(test), ci(ci).
	// --tags deploy → infra, app, db (3 targets).
	root := filepath.Join(testdataDir(t), "tagged")

	stdout, _, code := run(t, "exec", "--root", root, "--json", "--tags", "deploy", "--", "echo", "hi")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	var out struct {
		Results []struct {
			Target string `json:"target"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}

	if len(out.Results) != 3 {
		t.Fatalf("expected 3 results (infra, app, db), got %d: %+v", len(out.Results), out.Results)
	}

	names := make(map[string]bool)
	for _, r := range out.Results {
		names[r.Target] = true
	}
	if !names["infra"] || !names["app"] || !names["db"] {
		t.Errorf("expected infra, app, and db; got %v", names)
	}
}

func TestExecConcurrency(t *testing.T) {
	// With concurrency=1, targets should run sequentially.
	root := filepath.Join(testdataDir(t), "diamond")
	stdout, _, code := run(t, "exec", "--root", root, "--concurrency", "1", "--json", "--", "echo", "serial")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	var out struct {
		Results []struct {
			Target   string `json:"target"`
			ExitCode int    `json:"exit_code"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(out.Results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(out.Results))
	}
	for _, r := range out.Results {
		if r.ExitCode != 0 {
			t.Errorf("target %s: expected exit 0, got %d", r.Target, r.ExitCode)
		}
	}
}

func TestExecNoCommand(t *testing.T) {
	root := filepath.Join(testdataDir(t), "simple")
	_, stderr, code := run(t, "exec", "--root", root)
	if code != 1 {
		t.Errorf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr, "no command") {
		t.Errorf("expected 'no command' in stderr, got %q", stderr)
	}
}

func TestExecCycleError(t *testing.T) {
	root := filepath.Join(testdataDir(t), "cycle")
	_, stderr, code := run(t, "exec", "--root", root, "--", "echo", "fail")
	if code != 1 {
		t.Errorf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr, "cycle") {
		t.Errorf("expected 'cycle' in stderr, got %q", stderr)
	}
}

func TestExecEmptyAfterTagFilter(t *testing.T) {
	root := t.TempDir()
	writeCdageeJSON(t, root, "ws-a", `{"tags":["x"]}`)

	stdout, _, code := run(t, "exec", "--root", root, "--json", "--tags", "nope", "--", "echo", "hi")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	var out struct {
		Results []struct{} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(out.Results) != 0 {
		t.Errorf("expected 0 results, got %d", len(out.Results))
	}
}

func TestExecContinueOnErrorAllPass(t *testing.T) {
	root := filepath.Join(testdataDir(t), "simple")
	_, _, code := run(t, "exec", "--root", root, "--continue-on-error", "--", "echo", "ok")
	if code != 0 {
		t.Errorf("expected exit 0, got %d", code)
	}
}

func TestExecWithoutDashDash(t *testing.T) {
	// Go's flag package stops at the first non-flag argument, so the --
	// separator is optional when the command doesn't start with a dash.
	root := filepath.Join(testdataDir(t), "simple")
	stdout, _, code := run(t, "exec", "--root", root, "echo", "hello")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "hello") {
		t.Errorf("expected 'hello' in output, got %q", stdout)
	}
}

func TestExecNegativeConcurrency(t *testing.T) {
	root := filepath.Join(testdataDir(t), "simple")
	_, stderr, code := run(t, "exec", "--root", root, "--concurrency", "-1", "--", "echo", "hi")
	if code != 1 {
		t.Errorf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr, "concurrency") {
		t.Errorf("expected 'concurrency' in stderr, got %q", stderr)
	}
}

// --- exec: new fixture-based tests ---

func TestExecDeepChainOrder(t *testing.T) {
	root := filepath.Join(testdataDir(t), "deep-chain")
	stdout, _, code := run(t, "exec", "--root", root, "--json", "--concurrency", "1", "--", "echo", "ok")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	var out struct {
		Results []struct {
			Target   string `json:"target"`
			ExitCode int    `json:"exit_code"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(out.Results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(out.Results))
	}
	// With concurrency=1, results reflect completion order which is deterministic: a,b,c,d,e
	want := []string{"a", "b", "c", "d", "e"}
	for i, r := range out.Results {
		if r.Target != want[i] {
			t.Errorf("result[%d]: expected %q, got %q", i, want[i], r.Target)
		}
		if r.ExitCode != 0 {
			t.Errorf("result[%d] (%s): expected exit 0, got %d", i, r.Target, r.ExitCode)
		}
	}
}

func TestExecDeepChainContinueOnError(t *testing.T) {
	root := filepath.Join(testdataDir(t), "deep-chain")
	// Fail at "c" → d and e should be skipped.
	stdout, _, code := run(t, "exec", "--root", root, "--json", "--continue-on-error", "--",
		"sh", "-c", `if [ "$(basename "$PWD")" = "c" ]; then exit 1; fi; echo ok`)
	if code != 2 {
		t.Errorf("expected exit 2, got %d", code)
	}
	var out struct {
		Results []struct {
			Target   string `json:"target"`
			ExitCode int    `json:"exit_code"`
			Skipped  bool   `json:"skipped"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}

	resultMap := make(map[string]struct {
		ExitCode int
		Skipped  bool
	})
	for _, r := range out.Results {
		resultMap[r.Target] = struct {
			ExitCode int
			Skipped  bool
		}{r.ExitCode, r.Skipped}
	}

	if r := resultMap["a"]; r.ExitCode != 0 {
		t.Errorf("a: expected success, got exit %d", r.ExitCode)
	}
	if r := resultMap["b"]; r.ExitCode != 0 {
		t.Errorf("b: expected success, got exit %d", r.ExitCode)
	}
	if r := resultMap["c"]; r.ExitCode == 0 {
		t.Errorf("c: expected failure, got exit 0")
	}
	if r := resultMap["d"]; !r.Skipped {
		t.Errorf("d: expected skipped, got %+v", r)
	}
	if r := resultMap["e"]; !r.Skipped {
		t.Errorf("e: expected skipped, got %+v", r)
	}
}

func TestExecWideFanConcurrency(t *testing.T) {
	root := filepath.Join(testdataDir(t), "wide-fan")
	stdout, _, code := run(t, "exec", "--root", root, "--json", "--concurrency", "1", "--", "echo", "ok")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	var out struct {
		Results []struct {
			Target   string `json:"target"`
			ExitCode int    `json:"exit_code"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(out.Results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(out.Results))
	}
	// hub must be first (it's the only root)
	if out.Results[0].Target != "hub" {
		t.Errorf("expected hub first, got %q", out.Results[0].Target)
	}
	for _, r := range out.Results {
		if r.ExitCode != 0 {
			t.Errorf("target %s: expected exit 0, got %d", r.Target, r.ExitCode)
		}
	}
}

func TestExecMultiRootJSON(t *testing.T) {
	root := filepath.Join(testdataDir(t), "multi-root")
	stdout, _, code := run(t, "exec", "--root", root, "--json", "--", "echo", "ok")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	var out struct {
		Results []struct {
			Target   string `json:"target"`
			ExitCode int    `json:"exit_code"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(out.Results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(out.Results))
	}
	names := make(map[string]bool)
	for _, r := range out.Results {
		names[r.Target] = true
		if r.ExitCode != 0 {
			t.Errorf("target %s: expected exit 0, got %d", r.Target, r.ExitCode)
		}
	}
	for _, want := range []string{"alpha", "beta", "merge", "final"} {
		if !names[want] {
			t.Errorf("expected target %q in results", want)
		}
	}
}

func TestExecTaggedFilterMulti(t *testing.T) {
	// --tags test on tagged fixture → app, monitoring
	root := filepath.Join(testdataDir(t), "tagged")
	stdout, _, code := run(t, "exec", "--root", root, "--json", "--tags", "test", "--", "echo", "ok")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	var out struct {
		Results []struct {
			Target string `json:"target"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(out.Results) != 2 {
		t.Fatalf("expected 2 results, got %d: %+v", len(out.Results), out.Results)
	}
	names := make(map[string]bool)
	for _, r := range out.Results {
		names[r.Target] = true
	}
	if !names["app"] || !names["monitoring"] {
		t.Errorf("expected app and monitoring; got %v", names)
	}
}

// --- output flag ---

func TestDiscoverOutputJSON(t *testing.T) {
	root := filepath.Join(testdataDir(t), "diamond")
	stdoutJSON, _, code1 := run(t, "discover", "--root", root, "--json")
	if code1 != 0 {
		t.Fatalf("--json: expected exit 0, got %d", code1)
	}
	stdoutO, _, code2 := run(t, "discover", "--root", root, "-o", "json")
	if code2 != 0 {
		t.Fatalf("-o json: expected exit 0, got %d", code2)
	}
	if stdoutJSON != stdoutO {
		t.Errorf("--json and -o json produced different output:\n--json:  %s\n-o json: %s", stdoutJSON, stdoutO)
	}
}

func TestDiscoverOutputShorthand(t *testing.T) {
	root := filepath.Join(testdataDir(t), "simple")
	stdout, _, code := run(t, "discover", "--root", root, "-o", "json")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	var out struct {
		Targets []struct {
			ID string `json:"id"`
		} `json:"targets"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(out.Targets) != 2 {
		t.Errorf("expected 2 targets, got %d", len(out.Targets))
	}
}

func TestOutputAndJSONConflict(t *testing.T) {
	root := filepath.Join(testdataDir(t), "simple")
	_, stderr, code := run(t, "discover", "--root", root, "--json", "--output", "json")
	if code != 1 {
		t.Errorf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr, "cannot be used together") {
		t.Errorf("expected 'cannot be used together' in stderr, got %q", stderr)
	}
}

func TestOutputUnknownFormat(t *testing.T) {
	root := filepath.Join(testdataDir(t), "simple")
	_, stderr, code := run(t, "discover", "--root", root, "--output", "yaml")
	if code != 1 {
		t.Errorf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr, "unknown output format") {
		t.Errorf("expected 'unknown output format' in stderr, got %q", stderr)
	}
}

func TestGraphOutputNotSupported(t *testing.T) {
	root := filepath.Join(testdataDir(t), "simple")
	_, stderr, code := run(t, "graph", "--root", root, "-o", "json")
	if code == 0 {
		t.Fatal("expected non-zero exit for -o json with graph")
	}
	if !strings.Contains(stderr, "not supported") {
		t.Errorf("expected 'not supported' in stderr, got %q", stderr)
	}
}

// --- go templates ---

func TestDiscoverGoTemplate(t *testing.T) {
	root := filepath.Join(testdataDir(t), "simple")
	stdout, _, code := run(t, "discover", "--root", root, "-o", "go-template={{range .Targets}}{{.ID}} {{end}}")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	trimmed := strings.TrimSpace(stdout)
	if trimmed != "ws-a ws-b" {
		t.Errorf("expected 'ws-a ws-b', got %q", trimmed)
	}
}

func TestDiscoverGoTemplateToJSON(t *testing.T) {
	root := filepath.Join(testdataDir(t), "simple")
	stdout, _, code := run(t, "discover", "--root", root, "-o", `go-template={"include":{{toJSON .Targets}}}`)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	var out struct {
		Include []struct {
			ID string `json:"id"`
		} `json:"include"`
	}
	trimmed := strings.TrimSpace(stdout)
	if err := json.Unmarshal([]byte(trimmed), &out); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, trimmed)
	}
	if len(out.Include) != 2 {
		t.Errorf("expected 2 items in include, got %d", len(out.Include))
	}
}

func TestDiscoverGoTemplateFile(t *testing.T) {
	root := filepath.Join(testdataDir(t), "simple")
	tmplFile := filepath.Join(t.TempDir(), "test.tmpl")
	if err := os.WriteFile(tmplFile, []byte(`{{range .Targets}}{{.ID}} {{end}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, _, code := run(t, "discover", "--root", root, "-o", "go-template-file="+tmplFile)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	trimmed := strings.TrimSpace(stdout)
	if trimmed != "ws-a ws-b" {
		t.Errorf("expected 'ws-a ws-b', got %q", trimmed)
	}
}

func TestGoTemplateInvalidSyntax(t *testing.T) {
	root := filepath.Join(testdataDir(t), "simple")
	_, stderr, code := run(t, "discover", "--root", root, "-o", "go-template={{.Foo")
	if code != 1 {
		t.Errorf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr, "invalid template") {
		t.Errorf("expected 'invalid template' in stderr, got %q", stderr)
	}
}

func TestExecGoTemplate(t *testing.T) {
	root := filepath.Join(testdataDir(t), "simple")
	stdout, _, code := run(t, "exec", "--root", root, "-o", "go-template={{range .Results}}{{.Target}}:{{.ExitCode}} {{end}}", "--", "echo", "ok")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	trimmed := strings.TrimSpace(stdout)
	if !strings.Contains(trimmed, "ws-a:0") || !strings.Contains(trimmed, "ws-b:0") {
		t.Errorf("expected ws-a:0 and ws-b:0 in output, got %q", trimmed)
	}
}

func TestPlanOrderGoTemplate(t *testing.T) {
	root := filepath.Join(testdataDir(t), "diamond")
	stdout, _, code := run(t, "plan-order", "--root", root, "-o", "go-template={{range .Layers}}[{{range .Targets}}{{.}} {{end}}]{{end}}")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	trimmed := strings.TrimSpace(stdout)
	if !strings.Contains(trimmed, "root") {
		t.Errorf("expected 'root' in output, got %q", trimmed)
	}
	if !strings.Contains(trimmed, "leaf") {
		t.Errorf("expected 'leaf' in output, got %q", trimmed)
	}
}

func TestTemplateFuncJoin(t *testing.T) {
	root := filepath.Join(testdataDir(t), "diamond")
	stdout, _, code := run(t, "plan-order", "--root", root, "-o", `go-template={{range .Layers}}{{join .Targets ","}} {{end}}`)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	trimmed := strings.TrimSpace(stdout)
	// Layer 1 has "left,right" (sorted)
	if !strings.Contains(trimmed, "left,right") {
		t.Errorf("expected 'left,right' in output, got %q", trimmed)
	}
}

// --- tags on discover and plan-order ---

func TestDiscoverTags(t *testing.T) {
	root := filepath.Join(testdataDir(t), "tagged")
	stdout, _, code := run(t, "discover", "--root", root, "--tags", "deploy", "-o", "json")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	var out struct {
		Targets []struct {
			ID string `json:"id"`
		} `json:"targets"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(out.Targets) != 3 {
		t.Fatalf("expected 3 targets (infra, app, db), got %d: %+v", len(out.Targets), out.Targets)
	}
	names := make(map[string]bool)
	for _, tgt := range out.Targets {
		names[tgt.ID] = true
	}
	if !names["infra"] || !names["app"] || !names["db"] {
		t.Errorf("expected infra, app, db; got %v", names)
	}
}

func TestDiscoverTagsText(t *testing.T) {
	root := filepath.Join(testdataDir(t), "tagged")
	stdout, _, code := run(t, "discover", "--root", root, "--tags", "ci")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	lines := nonEmptyLines(stdout)
	if len(lines) != 1 || lines[0] != "ci" {
		t.Errorf("expected 1 line 'ci', got %v", lines)
	}
}

func TestDiscoverTagsNoMatch(t *testing.T) {
	root := filepath.Join(testdataDir(t), "tagged")
	stdout, _, code := run(t, "discover", "--root", root, "--tags", "nope")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("expected empty output, got %q", stdout)
	}
}

func TestPlanOrderTags(t *testing.T) {
	root := filepath.Join(testdataDir(t), "tagged")
	stdout, _, code := run(t, "plan-order", "--root", root, "--tags", "deploy", "-o", "json")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	var out struct {
		Layers []struct {
			Depth   int      `json:"depth"`
			Targets []string `json:"targets"`
		} `json:"layers"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(out.Layers) == 0 {
		t.Fatal("expected at least one layer")
	}
	// Collect all target names across layers
	all := make(map[string]bool)
	for _, layer := range out.Layers {
		for _, tgt := range layer.Targets {
			all[tgt] = true
		}
	}
	if !all["infra"] || !all["app"] || !all["db"] {
		t.Errorf("expected infra, app, db in layers; got %v", all)
	}
	if all["monitoring"] || all["ci"] {
		t.Errorf("unexpected non-deploy targets in layers: %v", all)
	}
}

func TestPlanOrderTagsEmpty(t *testing.T) {
	root := filepath.Join(testdataDir(t), "tagged")
	stdout, _, code := run(t, "plan-order", "--root", root, "--tags", "nope", "-o", "json")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	var out struct {
		Layers []struct{} `json:"layers"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(out.Layers) != 0 {
		t.Errorf("expected 0 layers, got %d", len(out.Layers))
	}
}

// --- multi-target ---

func TestDiscoverMultiTarget(t *testing.T) {
	root := filepath.Join(testdataDir(t), "multi-target")
	stdout, _, code := run(t, "discover", "--root", root, "--json")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	var out struct {
		Targets []struct {
			ID string `json:"id"`
		} `json:"targets"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(out.Targets) != 3 {
		t.Fatalf("expected 3 targets, got %d: %+v", len(out.Targets), out.Targets)
	}
}

func TestDiscoverMultiTargetTags(t *testing.T) {
	root := filepath.Join(testdataDir(t), "multi-target")
	stdout, _, code := run(t, "discover", "--root", root, "--tags", "staging", "--json")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	var out struct {
		Targets []struct {
			ID string `json:"id"`
		} `json:"targets"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(out.Targets) != 1 {
		t.Fatalf("expected 1 target, got %d: %+v", len(out.Targets), out.Targets)
	}
	if out.Targets[0].ID != "myapp:staging" {
		t.Errorf("expected myapp:staging, got %q", out.Targets[0].ID)
	}
}

func TestPlanOrderSerial(t *testing.T) {
	root := filepath.Join(testdataDir(t), "serial")
	stdout, _, code := run(t, "plan-order", "--root", root, "--json")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	var out struct {
		Layers []struct {
			Depth   int      `json:"depth"`
			Targets []string `json:"targets"`
		} `json:"layers"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	// base in first layer, then alpha, beta, gamma serialized
	if len(out.Layers) < 2 {
		t.Fatalf("expected at least 2 layers, got %d", len(out.Layers))
	}
	// Collect order: base must come before svc:alpha < svc:beta < svc:gamma
	pos := make(map[string]int)
	for _, layer := range out.Layers {
		for _, tgt := range layer.Targets {
			pos[tgt] = layer.Depth
		}
	}
	if pos["base"] >= pos["svc:alpha"] {
		t.Errorf("expected base before svc:alpha in layers")
	}
	if pos["svc:alpha"] >= pos["svc:beta"] {
		t.Errorf("expected svc:alpha before svc:beta in layers")
	}
	if pos["svc:beta"] >= pos["svc:gamma"] {
		t.Errorf("expected svc:beta before svc:gamma in layers")
	}
}

func TestExecSimpleTargetNoTargetName(t *testing.T) {
	root := filepath.Join(testdataDir(t), "simple")
	stdout, _, code := run(t, "exec", "--root", root, "--json", "--", "sh", "-c", "env | grep UPMAN_ || true")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	var out struct {
		Results []struct {
			Target     string `json:"target"`
			TargetName string `json:"target_name"`
			Output     string `json:"output"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	for _, r := range out.Results {
		if r.TargetName != "" {
			t.Errorf("target %s: expected empty target_name, got %q", r.Target, r.TargetName)
		}
		if strings.Contains(r.Output, "CDAGEE_TARGET_NAME=") {
			t.Errorf("target %s: CDAGEE_TARGET_NAME should not be set, output: %q", r.Target, r.Output)
		}
	}
}

func TestExecMultiTargetEnvVars(t *testing.T) {
	root := filepath.Join(testdataDir(t), "multi-target")
	stdout, _, code := run(t, "exec", "--root", root, "--tags", "staging", "--json", "--", "sh", "-c", "echo $CDAGEE_TARGET $CDAGEE_TARGET_NAME")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	var out struct {
		Results []struct {
			Target     string `json:"target"`
			TargetName string `json:"target_name"`
			Output     string `json:"output"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(out.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(out.Results))
	}
	r := out.Results[0]
	if r.Target != "myapp:staging" {
		t.Errorf("expected target myapp:staging, got %q", r.Target)
	}
	if r.TargetName != "staging" {
		t.Errorf("expected target_name staging, got %q", r.TargetName)
	}
	if !strings.Contains(r.Output, "myapp:staging staging") {
		t.Errorf("expected env vars in output, got %q", r.Output)
	}
}

// --- direnv ---

func TestExecDirenvWrapping(t *testing.T) {
	// Create a fixture with root direnv=true. Instead of requiring real direnv,
	// use a script that proves the command was wrapped.
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "cdagee.json"), []byte(`{"direnv": true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	writeCdageeJSON(t, root, "ws-a", `{}`)

	// Create a fake "direnv" script that prints a marker and execs the rest.
	binDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	fakeDirenv := filepath.Join(binDir, "direnv")
	if err := os.WriteFile(fakeDirenv, []byte("#!/bin/sh\necho DIRENV_WRAP\nshift 2\nexec \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Run with the fake direnv on PATH.
	cmd := exec.Command(binary, "exec", "--root", root, "--json", "--", "echo", "hello")
	cmd.Env = append(os.Environ(), "PATH="+binDir+":"+os.Getenv("PATH"))
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("exec failed: %v", err)
	}

	var result struct {
		Results []struct {
			Target string `json:"target"`
			Output string `json:"output"`
		} `json:"results"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(result.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result.Results))
	}
	if !strings.Contains(result.Results[0].Output, "DIRENV_WRAP") {
		t.Errorf("expected DIRENV_WRAP in output, got %q", result.Results[0].Output)
	}
	if !strings.Contains(result.Results[0].Output, "hello") {
		t.Errorf("expected 'hello' in output, got %q", result.Results[0].Output)
	}
}

func TestExecDirenvNotOnPath(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "cdagee.json"), []byte(`{"direnv": true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	writeCdageeJSON(t, root, "ws-a", `{}`)

	// Run with an empty PATH so direnv cannot be found.
	cmd := exec.Command(binary, "exec", "--root", root, "--", "echo", "hello")
	cmd.Env = []string{"PATH=/nonexistent"}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected non-zero exit when direnv not on PATH")
	}
	if !strings.Contains(stderr.String(), "direnv is required") {
		t.Errorf("expected 'direnv is required' in stderr, got %q", stderr.String())
	}
}

func TestExecDirenvDisabledNoCheck(t *testing.T) {
	// When no direnv is configured, should not check for direnv on PATH.
	root := t.TempDir()
	writeCdageeJSON(t, root, "ws-a", `{}`)

	stdout, _, code := run(t, "exec", "--root", root, "--json", "--", "echo", "ok")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "ok") {
		t.Errorf("expected 'ok' in output, got %q", stdout)
	}
}

func TestExecDirenvPerWorkspaceOverride(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "cdagee.json"), []byte(`{"direnv": true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	// ws-a inherits direnv=true, ws-b overrides to false
	writeCdageeJSON(t, root, "ws-a", `{}`)
	writeCdageeJSON(t, root, "ws-b", `{"direnv": false}`)

	// Create a fake direnv that prints a marker.
	binDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	fakeDirenv := filepath.Join(binDir, "direnv")
	if err := os.WriteFile(fakeDirenv, []byte("#!/bin/sh\necho DIRENV_WRAP\nshift 2\nexec \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(binary, "exec", "--root", root, "--json", "--concurrency", "1", "--", "echo", "hello")
	cmd.Env = append(os.Environ(), "PATH="+binDir+":"+os.Getenv("PATH"))
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("exec failed: %v", err)
	}

	var result struct {
		Results []struct {
			Target string `json:"target"`
			Output string `json:"output"`
		} `json:"results"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}

	resultMap := make(map[string]string)
	for _, r := range result.Results {
		resultMap[r.Target] = r.Output
	}

	// ws-a should be wrapped with direnv
	if !strings.Contains(resultMap["ws-a"], "DIRENV_WRAP") {
		t.Errorf("ws-a: expected DIRENV_WRAP in output, got %q", resultMap["ws-a"])
	}
	// ws-b should NOT be wrapped with direnv
	if strings.Contains(resultMap["ws-b"], "DIRENV_WRAP") {
		t.Errorf("ws-b: expected no DIRENV_WRAP in output, got %q", resultMap["ws-b"])
	}
}

// --- exec --stream ---

func TestExecStreamBasic(t *testing.T) {
	root := filepath.Join(testdataDir(t), "simple")
	stdout, _, code := run(t, "exec", "--root", root, "--stream", "--", "echo", "hello")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	// Both targets should appear with [target] prefixes.
	if !strings.Contains(stdout, "[ws-a] hello") {
		t.Errorf("expected [ws-a] hello in output, got %q", stdout)
	}
	if !strings.Contains(stdout, "[ws-b] hello") {
		t.Errorf("expected [ws-b] hello in output, got %q", stdout)
	}
}

func TestExecStreamSummary(t *testing.T) {
	// Use concurrency=1 for deterministic ordering.
	root := filepath.Join(testdataDir(t), "simple")
	stdout, _, code := run(t, "exec", "--root", root, "--stream", "--concurrency", "1", "--", "echo", "hello")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	// Summary banner should appear after streamed output.
	if !strings.Contains(stdout, "--- ws-a") {
		t.Errorf("expected summary banner for ws-a, got %q", stdout)
	}
	if !strings.Contains(stdout, "--- ws-b") {
		t.Errorf("expected summary banner for ws-b, got %q", stdout)
	}
	// The buffered output should NOT be re-printed after the banner.
	// With streaming, "hello" only appears in [target] prefixed lines.
	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		if line == "hello" {
			t.Errorf("found bare 'hello' line (non-prefixed) — output should not be re-printed in stream mode")
		}
	}
}

func TestExecStreamWithJSON(t *testing.T) {
	root := filepath.Join(testdataDir(t), "simple")
	_, stderr, code := run(t, "exec", "--root", root, "--stream", "--json", "--", "echo", "hello")
	if code != 1 {
		t.Errorf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr, "cannot be combined") {
		t.Errorf("expected 'cannot be combined' in stderr, got %q", stderr)
	}
}

func TestExecStreamFailedTarget(t *testing.T) {
	root := t.TempDir()
	writeCdageeJSON(t, root, "ws-pass", `{}`)
	writeCdageeJSON(t, root, "ws-fail", `{}`)

	stdout, _, code := run(t, "exec", "--root", root, "--stream", "--concurrency", "1", "--", "sh", "-c",
		`if [ "$(basename "$PWD")" = "ws-fail" ]; then echo "bad"; exit 1; fi; echo ok`)
	if code != 1 {
		t.Errorf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stdout, "FAILED") {
		t.Errorf("expected FAILED banner in output, got %q", stdout)
	}
}

func TestExecStreamContinueOnError(t *testing.T) {
	root := filepath.Join(testdataDir(t), "forest")
	stdout, _, code := run(t, "exec", "--root", root, "--stream", "--continue-on-error", "--",
		"sh", "-c", `if [ "$(basename "$PWD")" = "tree1-root" ]; then exit 1; fi; echo ok`)
	if code != 2 {
		t.Errorf("expected exit 2, got %d", code)
	}
	if !strings.Contains(stdout, "SKIPPED") {
		t.Errorf("expected SKIPPED message in output, got %q", stdout)
	}
}

// --- helpers ---

func writeCdageeJSON(t *testing.T, root, name, content string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cdagee.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func nonEmptyLines(s string) []string {
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
