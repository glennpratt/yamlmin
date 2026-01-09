package yamlmin_test

import (
	"os"
	"testing"

	"github.com/glennpratt/yamlmin/pkg/yamlmin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"gotest.tools/v3/golden"
)

func TestMarshal(t *testing.T) {
	input, err := os.ReadFile("testdata/fixture.yaml")
	require.NoError(t, err)

	type K8sStruct struct {
		Name string `json:"name"`
		Spec struct {
			Replicas int `json:"replicas"`
			Template struct {
				ContainerName string `json:"containerName"`
			} `json:"template"`
		} `json:"spec"`
	}

	structData := K8sStruct{Name: "deploy1"}
	structData.Spec.Replicas = 3
	structData.Spec.Template.ContainerName = "nginx"

	tests := []struct {
		name   string
		fn     func(interface{}) ([]byte, error)
		data   interface{}
		verify func(t *testing.T, output []byte)
	}{
		{
			name: "Marshal",
			fn:   yamlmin.Marshal,
			data: func() interface{} {
				var data interface{}
				require.NoError(t, yaml.Unmarshal(input, &data))
				return data
			}(),
			verify: func(t *testing.T, output []byte) {
				golden.Assert(t, string(output), "golden.yaml")
			},
		},
		{
			name: "MarshalWithOptions",
			fn: func(in interface{}) ([]byte, error) {
				return yamlmin.MarshalWithOptions(in, yamlmin.DefaultOptions())
			},
			data: func() interface{} {
				var data interface{}
				require.NoError(t, yaml.Unmarshal(input, &data))
				return data
			}(),
			verify: func(t *testing.T, output []byte) {
				assert.Regexp(t, `\*(map|list|str)`, string(output))
			},
		},
		{
			name: "K8sMarshal",
			fn:   yamlmin.K8sMarshal,
			data: map[string]interface{}{
				"item1": structData,
				"item2": structData,
			},
			verify: func(t *testing.T, output []byte) {
				assert.Contains(t, string(output), "containerName")
				assert.NotContains(t, string(output), "ContainerName")
				assert.Regexp(t, `\*map`, string(output))
			},
		},
		{
			name: "K8sMarshalWithOptions",
			fn: func(in interface{}) ([]byte, error) {
				return yamlmin.K8sMarshalWithOptions(in, yamlmin.DefaultOptions())
			},
			data: map[string]interface{}{
				"item1": structData,
				"item2": structData,
			},
			verify: func(t *testing.T, output []byte) {
				assert.Contains(t, string(output), "containerName")
				assert.Regexp(t, `\*map`, string(output))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := tt.fn(tt.data)
			require.NoError(t, err)
			tt.verify(t, output)
		})
	}
}

func TestMarshalReduction(t *testing.T) {
	input := []byte(`
data:
  block1:
    key1: value1
    nested: { deep: data }
  block2:
    key1: value1
    nested: { deep: data }
`)

	var data interface{}
	require.NoError(t, yaml.Unmarshal(input, &data))

	output, err := yamlmin.Marshal(data)
	require.NoError(t, err)

	assert.Less(t, len(output), len(input))
}
