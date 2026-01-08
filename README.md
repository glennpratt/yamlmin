# yamlmin

A Go library and CLI for minifying YAML documents by deduplicating common structures using anchors and aliases.

## Features

- **Deduplication**: Automatically identifies duplicate maps, lists, and strings and replaces them with YAML anchors and aliases.
- **Concise Anchor Names**: Uses type-aware anchor names (e.g., `&list1`, `&map1`, `&str1`) for readability.
- **Customizable**: Control minimum occurrence counts, minimum structure sizes, and indentation levels.
- **K8s Style Friendly**: Default output uses a 2-space indent, significantly reducing vertical space.

## Installation

```bash
go get github.com/glennpratt/yamlmin
```

## Usage

### Library

```go
import "github.com/glennpratt/yamlmin"

// Simple usage
minified, err := yamlmin.Marshal(inputStruct)

// Custom options
opts := yamlmin.DefaultOptions()
opts.MinSize = 50
opts.MaxDepth = 100        // Limit recursion depth
opts.MaxWidth = 5000       // Limit children per node
opts.TimeLimit = time.Second // Limit execution time
minified, err = yamlmin.MarshalWithOptions(inputStruct, opts)
```

### CLI

```bash
cat input.yaml | go run ./cmd/yaml-minify > minified.yaml
```

## Benchmarks

The project includes a benchmark suite in `minify_test.go` comparing `yamlmin` against `gopkg.in/yaml.v3` and `sigs.k8s.io/yaml`.

| Metric | Note |
|--------|------|
| **Execution Speed** | `yamlmin` is slower than standard marshaling because it performs a full tree scan and hashing to identify duplicates. |
| **Memory Usage** | Higher than standard encoders as it maintains a hash map of all nodes and their occurrences. |
| **Output Size** | Smaller on most input, can be > 50% smaller on some real-world k8s test data |

The benchmarks are currently designed to run against `testdata/fixture.yaml`. You can run them with:

```bash
go test -v -bench=. -benchmem
