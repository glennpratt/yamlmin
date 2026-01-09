package yamlmin_test

import (
	"os"
	"testing"

	"github.com/glennpratt/yamlmin/pkg/yamlmin"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	k8syaml "sigs.k8s.io/yaml"
)

func BenchmarkYAMLv3Marshal(b *testing.B) {
	input, err := os.ReadFile("testdata/fixture.yaml")
	require.NoError(b, err)
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

func BenchmarkK8sYAMLMarshal(b *testing.B) {
	input, err := os.ReadFile("testdata/fixture.yaml")
	require.NoError(b, err)
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

func BenchmarkMarshal(b *testing.B) {
	input, err := os.ReadFile("testdata/fixture.yaml")
	require.NoError(b, err)
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

func BenchmarkK8sMarshal(b *testing.B) {
	input, err := os.ReadFile("testdata/fixture.yaml")
	require.NoError(b, err)
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

func BenchmarkOutputSize(b *testing.B) {
	testData, err := os.ReadFile("testdata/fixture.yaml")
	require.NoError(b, err)

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
