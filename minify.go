// Package yamlmin provides YAML deduplication using anchors and aliases.
//
// This package identifies duplicate structures in YAML documents and replaces
// them with anchors and aliases to reduce file size.
//
// Basic usage:
//
//	import "github.com/glennpratt/yamlmin"
//
//	output, err := yamlmin.Marshal(myStruct)
package yamlmin

import (
	"bytes"
	"errors"
	"fmt"
	"hash/fnv"
	"sort"
	"strconv"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
	k8syaml "sigs.k8s.io/yaml"
)

// Options configures the deduplication behavior.
type Options struct {
	// MinOccurrences is the minimum number of duplicates required to create an anchor.
	// Default: 2
	MinOccurrences int

	// MinSize is the minimum structure size (in chars) to consider for deduplication.
	// Default: 20
	MinSize int

	// Indent is the number of spaces to use for indentation in output.
	// Default: 2
	Indent int

	// MaxDepth is the maximum tree depth to traverse during deduplication.
	// Default: 50
	MaxDepth int

	// MaxWidth is the maximum number of children (map keys or list items) to process in a single node.
	// Default: 10000
	MaxWidth int

	// TimeLimit is the maximum duration to wait for deduplication to complete.
	// Default: 0 (no limit)
	TimeLimit time.Duration
}

// DefaultOptions returns options with default values.
func DefaultOptions() Options {
	return Options{
		MinOccurrences: 2,
		MinSize:        20,
		Indent:         2,
		MaxDepth:       50,
		MaxWidth:       10000,
		TimeLimit:      0,
	}
}

// Marshal parses YAML, deduplicates, and returns minified YAML bytes.
// This matches the signature of gopkg.in/yaml.v3's Marshal and uses default options.
func Marshal(in interface{}) ([]byte, error) {
	return MarshalWithOptions(in, DefaultOptions())
}

// MarshalWithOptions accepts a custom configuration and returns minified YAML.
func MarshalWithOptions(in interface{}, opts Options) ([]byte, error) {
	var root yaml.Node
	if err := root.Encode(in); err != nil {
		return nil, fmt.Errorf("encoding to YAML nodes: %w", err)
	}

	return marshalNode(&root, opts)
}

// K8sMarshal first uses k8s library to marshal respecting JSON tags,
// then deduplicates and returns minified YAML bytes.
// See https://pkg.go.dev/sigs.k8s.io/yaml#Marshal and
// https://pkg.go.dev/sigs.k8s.io/yaml#JSONToYAML
func K8sMarshal(in interface{}) ([]byte, error) {
	opts := DefaultOptions()

	var root yaml.Node
	y, err := k8syaml.Marshal(in)
	if err != nil {
		return nil, fmt.Errorf("k8s marshaling: %w", err)
	}
	if err := yaml.Unmarshal(y, &root); err != nil {
		return nil, fmt.Errorf("parsing k8s YAML: %w", err)
	}

	return marshalNode(&root, opts)
}

// marshalNode performs the minification process and encoding on a yaml.Node.
func marshalNode(root *yaml.Node, opts Options) ([]byte, error) {
	process(root, opts)

	// Use encoder to control indentation
	indent := opts.Indent
	if indent <= 0 {
		indent = 2
	}

	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(indent)
	if err := encoder.Encode(root); err != nil {
		return nil, fmt.Errorf("marshaling YAML: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("closing encoder: %w", err)
	}

	return buf.Bytes(), nil
}

// process deduplicates a YAML node tree in-place.
func process(root *yaml.Node, opts Options) {
	df := newDuplicateFinder(opts)
	// Set deadline if time limit is positive
	if opts.TimeLimit > 0 {
		df.deadline = time.Now().Add(opts.TimeLimit)
	}

	df.scanNode(root, 0)
	df.markDuplicates()

	visited := make(map[uint64]*yaml.Node)
	df.replaceWithAliases(root, visited, 0)

	// Cleanup: remove anchors that have no aliases pointing to them
	df.removeUnusedAnchors()
}

// anchorInfo tracks an anchor node and its reference count.
type anchorInfo struct {
	node     *yaml.Node
	refCount int
}

// hasher pool to reduce allocations
var hasherPool = sync.Pool{
	New: func() interface{} {
		return fnv.New64a()
	},
}

// kv pool for sorting map keys
type kvPair struct {
	key   *yaml.Node
	value *yaml.Node
}

var kvSlicePool = sync.Pool{
	New: func() interface{} {
		s := make([]kvPair, 0, 16)
		return &s
	},
}

// duplicateFinder tracks duplicate YAML structures.
type duplicateFinder struct {
	minOccurrences int
	minSize        int
	maxDepth       int
	maxWidth       int
	deadline       time.Time

	nodesByHash map[uint64][]*yaml.Node
	isDuplicate map[uint64]bool        // tracks which hashes have duplicates
	anchorNodes map[string]*anchorInfo // tracks anchors we create for cleanup
	mapCounter  int
	listCounter int
	strCounter  int
}

// nextAnchorName returns a type-based anchor name like "list1", "map1", "str1", etc.
func (df *duplicateFinder) nextAnchorName(node *yaml.Node) string {
	switch node.Kind {
	case yaml.SequenceNode:
		df.listCounter++
		return "list" + strconv.Itoa(df.listCounter)
	case yaml.MappingNode:
		df.mapCounter++
		return "map" + strconv.Itoa(df.mapCounter)
	case yaml.ScalarNode:
		df.strCounter++
		return "str" + strconv.Itoa(df.strCounter)
	default:
		// Fallback for unexpected types
		df.mapCounter++
		return "anchor" + strconv.Itoa(df.mapCounter)
	}
}

func newDuplicateFinder(opts Options) *duplicateFinder {
	minOccurrences := opts.MinOccurrences
	if minOccurrences <= 0 {
		minOccurrences = 2
	}

	minSize := opts.MinSize
	if minSize <= 0 {
		minSize = 20
	}

	maxDepth := opts.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 50
	}

	maxWidth := opts.MaxWidth
	if maxWidth <= 0 {
		maxWidth = 10000
	}

	return &duplicateFinder{
		minOccurrences: minOccurrences,
		minSize:        minSize,
		maxDepth:       maxDepth,
		maxWidth:       maxWidth,
		nodesByHash:    make(map[uint64][]*yaml.Node),
		isDuplicate:    make(map[uint64]bool),
		anchorNodes:    make(map[string]*anchorInfo),
	}
}

// isDeadlineExceeded checks if the time limit has been reached.
func (df *duplicateFinder) isDeadlineExceeded() bool {
	if !df.deadline.IsZero() && time.Now().After(df.deadline) {
		return true
	}
	return false
}

func (df *duplicateFinder) hashNode(node *yaml.Node, depth int) (uint64, error) {
	h := hasherPool.Get().(interface {
		Write([]byte) (int, error)
		Sum64() uint64
		Reset()
	})
	defer func() {
		h.Reset()
		hasherPool.Put(h)
	}()

	if err := df.writeNodeToHash(h, node, depth); err != nil {
		return 0, err
	}
	return h.Sum64(), nil
}

var errLimitHit = errors.New("limit hit")

func (df *duplicateFinder) writeNodeToHash(h interface{ Write([]byte) (int, error) }, node *yaml.Node, depth int) error {
	if depth > df.maxDepth {
		return errLimitHit
	}
	if df.isDeadlineExceeded() {
		return errLimitHit
	}

	if node == nil {
		if _, err := h.Write([]byte("null")); err != nil {
			return err
		}
		return nil
	}

	if _, err := h.Write([]byte{byte(node.Kind)}); err != nil {
		return err
	}

	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			if err := df.writeNodeToHash(h, child, depth+1); err != nil {
				return err
			}
		}
	case yaml.MappingNode:
		// Cannot partially hash a map, must process all or fail to safeguard correctness
		if len(node.Content)/2 > df.maxWidth {
			return errLimitHit
		}

		// Get pooled slice
		pairsPtr := kvSlicePool.Get().(*[]kvPair)
		pairs := (*pairsPtr)[:0]

		for i := 0; i < len(node.Content); i += 2 {
			pairs = append(pairs, kvPair{node.Content[i], node.Content[i+1]})
		}
		sort.Slice(pairs, func(i, j int) bool {
			return pairs[i].key.Value < pairs[j].key.Value
		})

		for _, p := range pairs {
			if _, err := h.Write([]byte(p.key.Value)); err != nil {
				return err
			}
			if err := df.writeNodeToHash(h, p.value, depth+1); err != nil {
				*pairsPtr = pairs[:0]
				kvSlicePool.Put(pairsPtr)
				return err
			}
		}

		// Return slice to pool
		*pairsPtr = pairs[:0]
		kvSlicePool.Put(pairsPtr)
	case yaml.SequenceNode:
		if len(node.Content) > df.maxWidth {
			return errLimitHit
		}
		for _, child := range node.Content {
			if err := df.writeNodeToHash(h, child, depth+1); err != nil {
				return err
			}
		}
	case yaml.ScalarNode:
		if _, err := h.Write([]byte(node.Value)); err != nil {
			return err
		}
	case yaml.AliasNode:
		if node.Alias != nil {
			if err := df.writeNodeToHash(h, node.Alias, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}

func (df *duplicateFinder) estimateSize(node *yaml.Node, depth int) int {
	if depth > df.maxDepth {
		return 0 // Stop counting at max depth
	}
	if node == nil {
		return 0
	}

	size := len(node.Value)
	switch node.Kind {
	case yaml.MappingNode:
		for i := 0; i < len(node.Content); i++ {
			if i/2 >= df.maxWidth {
				break
			}
			size += df.estimateSize(node.Content[i], depth+1)
		}
	case yaml.SequenceNode:
		for i, child := range node.Content {
			if i >= df.maxWidth {
				break
			}
			size += df.estimateSize(child, depth+1)
		}
	default:
		for _, child := range node.Content {
			size += df.estimateSize(child, depth+1)
		}
	}
	return size
}

func (df *duplicateFinder) shouldAnchor(node *yaml.Node, depth int) bool {
	if node.Kind == yaml.ScalarNode {
		// Only deduplicate strings for now, and only if they meet size requirements
		if node.Tag != "!!str" {
			return false
		}
	} else if node.Kind != yaml.MappingNode && node.Kind != yaml.SequenceNode {
		return false
	}
	return df.estimateSize(node, depth) >= df.minSize
}

func (df *duplicateFinder) scanNode(node *yaml.Node, depth int) {
	if depth > df.maxDepth || df.isDeadlineExceeded() {
		return
	}
	if node == nil {
		return
	}

	if df.shouldAnchor(node, depth) {
		// If hashing fails (due to limits), we just skip this node as a duplicate candidate
		if hash, err := df.hashNode(node, depth); err == nil {
			df.nodesByHash[hash] = append(df.nodesByHash[hash], node)
		}
	}

	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			df.scanNode(child, depth+1)
		}
	case yaml.MappingNode:
		for i := 1; i < len(node.Content); i += 2 {
			if i/2 >= df.maxWidth {
				break
			}
			df.scanNode(node.Content[i], depth+1)
		}
	case yaml.SequenceNode:
		for i, child := range node.Content {
			if i >= df.maxWidth {
				break
			}
			df.scanNode(child, depth+1)
		}
	}
}

func (df *duplicateFinder) markDuplicates() {
	for hash, nodes := range df.nodesByHash {
		if len(nodes) >= df.minOccurrences {
			df.isDuplicate[hash] = true
		}
	}
}

func (df *duplicateFinder) replaceWithAliases(node *yaml.Node, visited map[uint64]*yaml.Node, depth int) {
	if depth > df.maxDepth || df.isDeadlineExceeded() {
		return
	}
	if node == nil {
		return
	}

	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			df.replaceWithAliases(child, visited, depth+1)
		}
	case yaml.MappingNode:
		for i := 1; i < len(node.Content); i += 2 {
			if i/2 >= df.maxWidth {
				break
			}
			value := node.Content[i]

			if df.shouldAnchor(value, depth) {
				// If hash fails, we can't safely replace, so skip
				if hash, err := df.hashNode(value, depth); err == nil {
					if firstNode, exists := visited[hash]; exists && firstNode.Anchor != "" {
						if value != firstNode {
							aliasNode := &yaml.Node{
								Kind:  yaml.AliasNode,
								Value: firstNode.Anchor,
								Alias: firstNode,
							}
							node.Content[i] = aliasNode
							df.anchorNodes[firstNode.Anchor].refCount++
							continue
						}
					} else if !exists {
						// Only create anchor if this hash has duplicates
						if df.isDuplicate[hash] {
							value.Anchor = df.nextAnchorName(value)
							df.anchorNodes[value.Anchor] = &anchorInfo{node: value, refCount: 0}
							visited[hash] = value
						}
					}
				}
			}

			df.replaceWithAliases(value, visited, depth+1)
		}
	case yaml.SequenceNode:
		for i, child := range node.Content {
			if i >= df.maxWidth {
				break
			}
			if df.shouldAnchor(child, depth) {
				if hash, err := df.hashNode(child, depth); err == nil {
					if firstNode, exists := visited[hash]; exists && firstNode.Anchor != "" {
						if child != firstNode {
							aliasNode := &yaml.Node{
								Kind:  yaml.AliasNode,
								Value: firstNode.Anchor,
								Alias: firstNode,
							}
							node.Content[i] = aliasNode
							df.anchorNodes[firstNode.Anchor].refCount++
							continue
						}
					} else if !exists {
						if df.isDuplicate[hash] {
							child.Anchor = df.nextAnchorName(child)
							df.anchorNodes[child.Anchor] = &anchorInfo{node: child, refCount: 0}
							visited[hash] = child
						}
					}
				}
			}

			df.replaceWithAliases(child, visited, depth+1)
		}
	}
}

// removeUnusedAnchors clears anchors that have no aliases pointing to them.
// Uses O(m) map iteration instead of O(n) tree traversal.
func (df *duplicateFinder) removeUnusedAnchors() {
	for _, info := range df.anchorNodes {
		if info.refCount == 0 {
			info.node.Anchor = ""
		}
	}
}
