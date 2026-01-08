package yamlmin_test

import (
	"os"
	"regexp"
	"testing"

	"github.com/glennpratt/yamlmin"
	"gopkg.in/yaml.v3"
	k8syaml "sigs.k8s.io/yaml"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gotest.tools/v3/golden"
)

// TestMarshal uses golden files to verify minification.
// To update golden file: go test . -update
func TestMarshal(t *testing.T) {
	// Read the fixture file
	input, err := os.ReadFile("testdata/fixture.yaml")
	require.NoError(t, err, "failed to read testdata/fixture.yaml")

	opts := yamlmin.DefaultOptions()
	var data interface{}
	require.NoError(t, yaml.Unmarshal(input, &data))

	output, err := yamlmin.MarshalWithOptions(data, opts)
	require.NoError(t, err, "MarshalWithOptions failed")

	// Count aliases (deduplicated references) via regex
	aliasRe := regexp.MustCompile(`\*(map|list|str|num|bool|null)\d+`)
	aliases := aliasRe.FindAllString(string(output), -1)
	assert.NotEmpty(t, aliases, "expected to find aliases (deduplicated structures) in output")

	// Assert against golden file
	golden.Assert(t, string(output), "golden.yaml")

	// Log stats
	t.Logf("Input size: %d bytes", len(input))
	t.Logf("Output size: %d bytes", len(output))
	t.Logf("Reduction: %.1f%%", 100.0*(1.0-float64(len(output))/float64(len(input))))
	t.Logf("Duplicates deduplicated: %d", len(aliases))
}

func TestMarshalReduction(t *testing.T) {
	// Create test data with known duplicates
	input := []byte(`
data:
  block1:
    key1: value1
    key2: value2
    nested:
      deep: data
  block2:
    key1: value1
    key2: value2
    nested:
      deep: data
  block3:
    key1: value1
    key2: value2
    nested:
      deep: data
`)

	data := map[string]interface{}{}
	require.NoError(t, yaml.Unmarshal(input, &data))

	output, err := yamlmin.Marshal(data)
	require.NoError(t, err)

	if len(output) >= len(input) {
		t.Errorf("Expected output to be smaller than input. Input: %d, Output: %d",
			len(input), len(output))
	}

	t.Logf("Reduction: %d bytes -> %d bytes (%.1f%%)",
		len(input), len(output), 100.0*(1.0-float64(len(output))/float64(len(input))))
}

func TestK8sMarshal(t *testing.T) {
	type K8sStruct struct {
		Name string `json:"name"`
		Spec struct {
			Replicas int `json:"replicas"`
			Template struct {
				ContainerName string `json:"containerName"`
			} `json:"template"`
		} `json:"spec"`
	}

	// Create duplicate data
	struct1 := K8sStruct{Name: "deploy1"}
	struct1.Spec.Replicas = 3
	struct1.Spec.Template.ContainerName = "nginx"

	struct2 := K8sStruct{Name: "deploy2"}
	struct2.Spec.Replicas = 3
	struct2.Spec.Template.ContainerName = "nginx"

	input := map[string]interface{}{
		"item1": struct1,
		"item2": struct2,
	}

	// Marshal with standard Marshal (will use field names or yaml tags if present)
	outStd, err := yamlmin.Marshal(input)
	require.NoError(t, err)
	// Marshal with K8sMarshal (will use json tags)
	outK8s, err := yamlmin.K8sMarshal(input)
	require.NoError(t, err)

	// Verify outK8s contains json tags
	assert.Contains(t, string(outK8s), "containerName")
	assert.NotContains(t, string(outK8s), "ContainerName")

	// Verify outStd contains field names (since no yaml tags)
	assert.Contains(t, string(outStd), "containername") // yaml.v3 lowercases by default if no tag? Let's check.
	// Actually based on my previous test:
	// yaml.v3 with only json: myfield: value (lowercase of MyField)

	// Verify deduplication happened by checking for an anchor and an alias
	assert.Regexp(t, `&map\d+`, string(outK8s))
	assert.Regexp(t, `\*map\d+`, string(outK8s))
}

// BenchmarkYAMLv3Marshal benchmarks standard yaml.v3 Marshal
func BenchmarkYAMLv3Marshal(b *testing.B) {
	input, err := os.ReadFile("testdata/fixture.yaml")
	require.NoError(b, err, "failed to read testdata/fixture.yaml")
	var data interface{}
	require.NoError(b, yaml.Unmarshal(input, &data))

	b.ResetTimer()
	b.ReportAllocs()

	var output []byte
	for b.Loop() {
		var err error
		output, err = yaml.Marshal(data)
		if err != nil {
			b.Fatal(err)
		}
	}
	b.SetBytes(int64(len(output)))
}

// BenchmarkK8sYAMLMarshal benchmarks sigs.k8s.io/yaml Marshal
func BenchmarkK8sYAMLMarshal(b *testing.B) {
	input, err := os.ReadFile("testdata/fixture.yaml")
	require.NoError(b, err, "failed to read testdata/fixture.yaml")
	var data map[string]interface{}
	require.NoError(b, k8syaml.Unmarshal(input, &data))

	b.ResetTimer()
	b.ReportAllocs()

	var output []byte
	for b.Loop() {
		var err error
		output, err = k8syaml.Marshal(data)
		if err != nil {
			b.Fatal(err)
		}
	}
	b.SetBytes(int64(len(output)))
}

// BenchmarkMarshal benchmarks yamlmin.Marshal
func BenchmarkMarshal(b *testing.B) {
	input, err := os.ReadFile("testdata/fixture.yaml")
	require.NoError(b, err, "failed to read testdata/fixture.yaml")
	var data interface{}
	require.NoError(b, yaml.Unmarshal(input, &data))

	b.ResetTimer()
	b.ReportAllocs()

	var output []byte
	for b.Loop() {
		var err error
		output, err = yamlmin.Marshal(data)
		if err != nil {
			b.Fatal(err)
		}
	}
	b.SetBytes(int64(len(output)))
}

// BenchmarkK8sMarshal benchmarks yamlmin.K8sMarshal
func BenchmarkK8sMarshal(b *testing.B) {
	input, err := os.ReadFile("testdata/fixture.yaml")
	require.NoError(b, err, "failed to read testdata/fixture.yaml")
	var data interface{}
	require.NoError(b, k8syaml.Unmarshal(input, &data))

	b.ResetTimer()
	b.ReportAllocs()

	var output []byte
	for b.Loop() {
		var err error
		output, err = yamlmin.K8sMarshal(data)
		if err != nil {
			b.Fatal(err)
		}
	}
	b.SetBytes(int64(len(output)))
}

// BenchmarkOutputSize compares output sizes (not speed)
func BenchmarkOutputSize(b *testing.B) {
	testData, err := os.ReadFile("testdata/fixture.yaml")
	require.NoError(b, err, "failed to read test data: %v", err)

	var data map[string]interface{}
	require.NoError(b, k8syaml.Unmarshal(testData, &data))

	yamlv3Out, _ := yaml.Marshal(data)
	k8sOut, _ := k8syaml.Marshal(data)
	minifiedOut, _ := yamlmin.Marshal(data)

	b.ReportMetric(float64(len(testData)), "input_bytes")
	b.ReportMetric(float64(len(yamlv3Out)), "yamlv3_bytes")
	b.ReportMetric(float64(len(k8sOut)), "k8s_bytes")
	b.ReportMetric(float64(len(minifiedOut)), "minified_bytes")
	b.ReportMetric(100.0*(1.0-float64(len(minifiedOut))/float64(len(yamlv3Out))), "reduction_%")
}
