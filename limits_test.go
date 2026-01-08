package yamlmin_test

import (
	"strings"
	"testing"
	"time"

	"github.com/glennpratt/yamlmin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestLogicalEquality(t *testing.T) {
	data := map[string]interface{}{
		"a": "long_string_1",
		"b": "long_string_1",
		"c": map[string]interface{}{
			"d": "long_string_2",
			"e": "long_string_2",
			"f": []string{"long_string_3", "long_string_3"},
		},
	}

	opts := yamlmin.DefaultOptions()
	opts.MinSize = 5

	out, err := yamlmin.MarshalWithOptions(data, opts)
	require.NoError(t, err)

	var roundtrip interface{}
	require.NoError(t, yaml.Unmarshal(out, &roundtrip))

	expectedBytes, _ := yaml.Marshal(data)
	actualBytes, _ := yaml.Marshal(roundtrip)
	assert.YAMLEq(t, string(expectedBytes), string(actualBytes))
}

func TestMaxDepthDeduplication(t *testing.T) {
	data := map[string]interface{}{
		"s1": "long_string_shallow",
		"s2": "long_string_shallow",
		"n": map[string]interface{}{
			"d1": "long_string_deep",
			"d2": "long_string_deep",
		},
	}

	// Depth 0: Root MappingNode
	// Depth 1: Values s1, s2, n
	// Depth 2: Values d1, d2

	opts := yamlmin.DefaultOptions()
	opts.MaxDepth = 1 // Should allow s1, s2 but not d1, d2
	opts.MinSize = 5

	out, err := yamlmin.MarshalWithOptions(data, opts)
	require.NoError(t, err)
	outputStr := string(out)

	// Verify s1/s2 are deduplicated
	assert.Contains(t, outputStr, "&str", "Should have shallow deduplication")

	// Count anchors. Should be 1.
	anchorCount := strings.Count(outputStr, "&str")
	assert.Equal(t, 1, anchorCount, "Should ONLY have 1 anchor (shallow)")

	// Verify deep strings are present but not anchored
	assert.Contains(t, outputStr, "long_string_deep")

	// Logical equality check
	var roundtrip interface{}
	require.NoError(t, yaml.Unmarshal(out, &roundtrip))
	expectedBytes, _ := yaml.Marshal(data)
	actualBytes, _ := yaml.Marshal(roundtrip)
	assert.YAMLEq(t, string(expectedBytes), string(actualBytes))
}

func TestMaxWidthDeduplication(t *testing.T) {
	data := []string{
		"dup_start",
		"dup_start",
		"unique1",
		"unique2",
		"dup_end",
		"dup_end",
	}

	opts := yamlmin.DefaultOptions()
	opts.MaxWidth = 2
	opts.MinSize = 5

	out, err := yamlmin.MarshalWithOptions(data, opts)
	require.NoError(t, err)

	outputStr := string(out)

	anchorCount := strings.Count(outputStr, "&str")
	assert.Equal(t, 1, anchorCount, "Should only have 1 anchor (start)")

	// Logical equality check
	var roundtrip interface{}
	require.NoError(t, yaml.Unmarshal(out, &roundtrip))
	expectedBytes, _ := yaml.Marshal(data)
	actualBytes, _ := yaml.Marshal(roundtrip)
	assert.YAMLEq(t, string(expectedBytes), string(actualBytes))
}

func TestTimeLimitGraceful(t *testing.T) {
	root := make([]interface{}, 100)
	for i := 0; i < 100; i++ {
		root[i] = map[string]string{
			"k": "very_very_long_value_to_ensure_dedup",
		}
	}

	opts := yamlmin.DefaultOptions()
	opts.TimeLimit = 1 * time.Nanosecond

	out, err := yamlmin.MarshalWithOptions(root, opts)
	require.NoError(t, err)

	var roundtrip interface{}
	require.NoError(t, yaml.Unmarshal(out, &roundtrip))

	// Logical equality check
	expectedBytes, _ := yaml.Marshal(root)
	actualBytes, _ := yaml.Marshal(roundtrip)
	assert.YAMLEq(t, string(expectedBytes), string(actualBytes))
}
