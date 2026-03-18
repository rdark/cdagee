# cdagee

Like GNU parallel, but with DAGs — run commands across directories with
dependency ordering and safe concurrency.

**c**oncurrent **d**irected **a**cyclic **g**raph **e**xecution **e**ngine

cdagee discovers targets (directories containing a `cdagee.json` marker file),
builds a dependency DAG, and executes commands across them with correct ordering
and safe concurrency. It supports tag-based filtering, multi-target directories,
serialization control, and flexible output formatting including Go templates for
CI/CD integration.

## Install

```bash
go install github.com/rdark/cdagee/cmd/cdagee@latest
```

Or build from source:

```bash
make build    # produces ./cdagee
```

## Quick start

Given a directory layout:

```
infra/
  network/cdagee.json          # {"tags": ["deploy"]}
  compute/cdagee.json          # {"depends_on": ["network"], "tags": ["deploy"]}
  monitoring/cdagee.json       # {"depends_on": ["compute"], "tags": ["observe"]}
```

```bash
# List all targets
cdagee discover --root infra

# Validate the dependency graph
cdagee validate --root infra

# See execution layers (what can run in parallel)
cdagee plan-order --root infra

# Run terraform plan across all targets, respecting dependencies
cdagee exec --root infra -- terraform plan

# Only deploy-tagged targets, max 2 at a time
cdagee exec --root infra --tags deploy --concurrency 2 -- terraform apply -auto-approve
```

## Configuration

Each target directory contains a `cdagee.json` file. Hidden directories
(starting with `.`) are skipped during discovery.

### Root-level settings

A `cdagee.json` in the root directory itself is not a target — it configures
discovery-wide behaviour:

```json
{
  "direnv": true
}
```

- **`direnv`** — when `true`, all commands are wrapped with `direnv exec .`
  (default: `false`). Can be overridden per-directory (see below).

### Single-target directory

```json
{
  "depends_on": ["network"],
  "tags": ["deploy"]
}
```

All fields are optional. An empty `{}` is valid.

- **`depends_on`** — target IDs that must complete before this one
- **`tags`** — labels for filtering with `--tags`
- **`direnv`** — override the root-level `direnv` setting for this directory

The target ID is the directory's relative path from root, using forward slashes
(e.g. `services/api`).

### Multi-target directory

A single directory can define multiple targets, useful when the same codebase
needs to be applied to different environments (e.g. staging and production
Terraform workspaces):

```json
{
  "depends_on": ["infra"],
  "tags": ["deploy"],
  "serial": true,
  "targets": {
    "staging": {
      "tags": ["staging"]
    },
    "prod": {
      "depends_on": [":staging"],
      "tags": ["prod"]
    }
  }
}
```

Each key in `targets` becomes a separate node in the DAG with ID
`dirpath:name` (e.g. `myapp:staging`, `myapp:prod`).

Directory-level `depends_on` and `tags` are inherited by all targets and merged
with per-target values. In the example above, `myapp:prod` resolves to
`depends_on: ["infra", "myapp:staging"]` and `tags: ["deploy", "prod"]`.

**Intra-directory references** — prefix a dependency with `:` to reference a
sibling target. `:staging` expands to `myapp:staging` when the directory ID is
`myapp`.

**Serial execution** — set `"serial": true` to insert chain edges between
targets in sorted order so they execute one at a time. The default is parallel
execution within the directory.

Target names must be non-empty and cannot contain `:` or `/`.

## Commands

All commands accept `--root DIR` (default `.`).

### discover

List all targets.

```bash
cdagee discover --root infra
cdagee discover --root infra --tags deploy --json
```

### validate

Check the dependency graph for cycles, dangling references, and duplicate IDs.
Exits 0 on success, 1 on error.

```bash
cdagee validate --root infra
```

### graph

Output the dependency graph in DOT format (Graphviz). Includes synthetic serial
chain edges.

```bash
cdagee graph --root infra | dot -Tpng -o deps.png
```

### plan-order

Show concurrency-safe execution layers. Targets in the same layer can run in
parallel; layers must execute in order.

```bash
cdagee plan-order --root infra --json
```

### exec

Run a command in each target directory, respecting dependency order.

```bash
cdagee exec --root infra -- terraform plan
cdagee exec --root infra --concurrency 4 -- make build
cdagee exec --root infra --continue-on-error -- ./deploy.sh
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--tags TAGS` | Filter targets by comma-separated tags (OR matching) |
| `--concurrency N` | Max parallel commands (0 = unlimited) |
| `--continue-on-error` | Don't cancel sibling branches on failure (exit 2) |
| `--stream` | Stream command output in real-time with `[target]` prefixes (text mode only) |

**Environment variables** set for each command:

| Variable | Description |
|----------|-------------|
| `CDAGEE_TARGET` | Full target ID (e.g. `myapp:staging`) |
| `CDAGEE_TARGET_NAME` | Sub-target name (e.g. `staging`); only set for multi-target IDs |

**Exit codes:** 0 = all passed, 1 = failure (fail-fast), 2 = failure
(continue-on-error).

## Output formats

All commands except `graph` support `-o, --output FMT` (or `--json` as
shorthand for `-o json`):

| Format | Description |
|--------|-------------|
| `text` | Human-readable (default) |
| `json` | Pretty-printed JSON |
| `go-template=<tmpl>` | Go [text/template](https://pkg.go.dev/text/template) |
| `go-template-file=<path>` | Template loaded from a file |

**Template functions:** `toJSON`, `toPrettyJSON`, `join` (strings.Join).

**Template data structures** — the template receives the same object as JSON
output:

- `discover`: `{{.Targets}}` — each has `.ID`, `.DependsOn`, `.Tags`, `.Serial`
- `plan-order`: `{{.Layers}}` — each has `.Depth`, `.Targets` (string slice)
- `exec`: `{{.Results}}` — each has `.Target`, `.TargetName`, `.ExitCode`, `.Output`, `.DurationMs`, `.Skipped`

## CI/CD integration

### GitHub Actions dynamic matrix

Use `go-template` output to generate a
[dynamic matrix](https://docs.github.com/en/actions/writing-workflows/choosing-what-your-workflow-does/running-variations-of-jobs-in-a-workflow#example-adding-configurations)
for GitHub Actions:

```yaml
jobs:
  discover:
    runs-on: ubuntu-latest
    outputs:
      matrix: ${{ steps.matrix.outputs.matrix }}
    steps:
      - uses: actions/checkout@v4
      # Install cdagee (or use a pre-built binary)
      - run: go install github.com/rdark/cdagee/cmd/cdagee@latest
      - id: matrix
        run: |
          MATRIX=$(cdagee discover --root infra --tags deploy \
            -o 'go-template={"include":{{toJSON .Targets}}}')
          echo "matrix=$MATRIX" >> "$GITHUB_OUTPUT"

  deploy:
    needs: discover
    runs-on: ubuntu-latest
    strategy:
      matrix: ${{ fromJson(needs.discover.outputs.matrix) }}
    steps:
      - uses: actions/checkout@v4
      - run: echo "Deploying ${{ matrix.id }}"
```

This produces a matrix entry per target with `id`, `depends_on`, and `tags`
fields available as expressions.

#### Layer-aware matrix (sequential stages)

If you need to respect dependency ordering across jobs, generate a matrix per
layer:

```yaml
jobs:
  plan:
    runs-on: ubuntu-latest
    outputs:
      layers: ${{ steps.layers.outputs.layers }}
    steps:
      - uses: actions/checkout@v4
      - run: go install github.com/rdark/cdagee/cmd/cdagee@latest
      - id: layers
        run: |
          LAYERS=$(cdagee plan-order --root infra --tags deploy -o json)
          echo "layers=$LAYERS" >> "$GITHUB_OUTPUT"
```

Then use separate jobs per layer with `needs:` chains, or iterate layers in a
reusable workflow.

#### Filtering by tags for separate pipelines

```yaml
# Deploy pipeline — only deploy-tagged targets
- run: cdagee exec --root infra --tags deploy -- ./deploy.sh

# Test pipeline — only test-tagged targets
- run: cdagee exec --root infra --tags test -- make test
```

### GitLab CI dynamic child pipelines

Generate a [child pipeline](https://docs.gitlab.com/ci/pipelines/downstream_pipelines/)
configuration from discovered targets:

```yaml
generate-pipeline:
  stage: prepare
  script:
    - |
      cdagee discover --root infra --tags deploy \
        -o 'go-template-file=.gitlab/target-pipeline.tmpl' \
        > child-pipeline.yml
  artifacts:
    paths:
      - child-pipeline.yml

trigger-targets:
  stage: deploy
  trigger:
    include:
      - artifact: child-pipeline.yml
        job: generate-pipeline
```

With a template file `.gitlab/target-pipeline.tmpl`:

```
{{- range .Targets}}
{{.ID}}:
  stage: deploy
  script:
    - cd {{.ID}} && ./deploy.sh
{{end -}}
```

### Terraform workspaces

Multi-target directories map naturally to Terraform workspaces:

```json
{
  "depends_on": ["network"],
  "serial": true,
  "targets": {
    "dev": { "tags": ["dev"] },
    "staging": { "depends_on": [":dev"], "tags": ["staging"] },
    "prod": { "depends_on": [":staging"], "tags": ["prod"] }
  }
}
```

```bash
cdagee exec --root infra --tags prod -- \
  sh -c 'terraform workspace select "$CDAGEE_TARGET_NAME" && terraform apply'
```

The `CDAGEE_TARGET_NAME` environment variable provides the sub-target name
(e.g. `staging`, `prod`) for workspace selection.

### Makefile / script orchestration

Run a common Makefile target across all services:

```bash
# Build all services respecting dependency order
cdagee exec --root services -- make build

# Test everything, don't stop on first failure
cdagee exec --root services --continue-on-error -- make test

# Lint only services tagged "go"
cdagee exec --root services --tags go -- golangci-lint run
```

## Generating dependency documentation

```bash
# SVG dependency graph
cdagee graph --root infra | dot -Tsvg -o deps.svg

# Simple target list for shell scripts
cdagee discover --root infra --tags deploy -o 'go-template={{range .Targets}}{{.ID}}{{"\n"}}{{end}}'

# CSV of targets with their tags
cdagee discover --root infra -o 'go-template={{range .Targets}}{{.ID}},{{join .Tags ","}}{{"\n"}}{{end}}'
```

## Library usage

cdagee's packages are importable as a Go library. There are three levels of
abstraction depending on how much control you need.

### Quick start (facade)

The top-level `cdagee` package provides a single `Load` call that discovers
targets, builds the graph, and computes execution layers:

```go
import "github.com/rdark/cdagee"

plan, err := cdagee.Load("infra")                  // all targets
plan, err := cdagee.Load("infra", "deploy", "test") // tag-filtered

// Execution layers — targets within a layer can run concurrently
for i, layer := range plan.Layers {
    fmt.Printf("layer %d: %v\n", i, layer)
}

// Look up a specific target by ID
tgt, ok := plan.Target("network")

// Access the underlying DAG for queries or execution
parents := plan.Graph.Parents("compute")
children := plan.Graph.Children("network")
```

### Lower-level packages

For full control over each step, use `target` and `dag` directly:

```go
import (
    "github.com/rdark/cdagee/dag"
    "github.com/rdark/cdagee/target"
)

dr, _ := target.Discover("infra")
filtered := target.FilterByTags(dr.Targets, []string{"deploy"})
g, _ := target.BuildGraph(filtered)

g.Execute(ctx, func(ctx context.Context, id string, tgt target.Target,
    parentResults iter.Seq2[string, any]) (any, error) {
    // run your logic per target
    return result, nil
})
```

### Standalone DAG

The `dag` package is a generic, dependency-free DAG that can be used
independently of the target system:

```go
import "github.com/rdark/cdagee/dag"

d := dag.New[string]()
d.AddNode("a", "data-a")
d.AddNode("b", "data-b")
d.AddEdge("a", "b")

layers, _ := d.Layers()     // [[a] [b]]
parents := d.Parents("b")   // [a]
children := d.Children("a") // [b]
```
