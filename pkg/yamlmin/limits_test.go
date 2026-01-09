package yamlmin_test

import (
	"strings"
	"testing"
	"time"

	"github.com/glennpratt/yamlmin/pkg/yamlmin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestLimits(t *testing.T) {
	tests := []struct {
		name string
		fn   func(interface{}, yamlmin.Options) ([]byte, error)
	}{
		{"MarshalWithOptions", yamlmin.MarshalWithOptions},
		{"K8sMarshalWithOptions", yamlmin.K8sMarshalWithOptions},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Run("LogicalEquality", func(t *testing.T) {
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

				out, err := tt.fn(data, opts)
				require.NoError(t, err)

				var roundtrip interface{}
				require.NoError(t, yaml.Unmarshal(out, &roundtrip))

				expectedBytes, _ := yaml.Marshal(data)
				actualBytes, _ := yaml.Marshal(roundtrip)
				assert.YAMLEq(t, string(expectedBytes), string(actualBytes))
			})

			t.Run("MaxDepth", func(t *testing.T) {
				data := map[string]interface{}{
					"s1": "long_string_shallow",
					"s2": "long_string_shallow",
					"n": map[string]interface{}{
						"d1": "long_string_deep",
						"d2": "long_string_deep",
					},
				}
				opts := yamlmin.DefaultOptions()
				opts.MaxDepth = 1
				opts.MinSize = 5

				out, err := tt.fn(data, opts)
				require.NoError(t, err)
				outputStr := string(out)

				assert.Contains(t, outputStr, "&str")
				assert.Equal(t, 1, strings.Count(outputStr, "&str"))
				assert.Contains(t, outputStr, "long_string_deep")

				var roundtrip interface{}
				require.NoError(t, yaml.Unmarshal(out, &roundtrip))
				expectedBytes, _ := yaml.Marshal(data)
				actualBytes, _ := yaml.Marshal(roundtrip)
				assert.YAMLEq(t, string(expectedBytes), string(actualBytes))
			})

			t.Run("MaxWidth", func(t *testing.T) {
				data := []string{"dup_start", "dup_start", "unique1", "unique2", "dup_end", "dup_end"}

				opts := yamlmin.DefaultOptions()
				opts.MaxWidth = 2
				opts.MinSize = 5

				out, err := tt.fn(data, opts)
				require.NoError(t, err)
				outputStr := string(out)

				assert.Equal(t, 1, strings.Count(outputStr, "&str"))

				var roundtrip interface{}
				require.NoError(t, yaml.Unmarshal(out, &roundtrip))
				expectedBytes, _ := yaml.Marshal(data)
				actualBytes, _ := yaml.Marshal(roundtrip)
				assert.YAMLEq(t, string(expectedBytes), string(actualBytes))
			})

			t.Run("TimeLimit", func(t *testing.T) {
				root := make([]interface{}, 100)
				for i := 0; i < 100; i++ {
					root[i] = map[string]string{"k": "very_very_long_value_to_ensure_dedup"}
				}

				opts := yamlmin.DefaultOptions()
				opts.TimeLimit = 1 * time.Nanosecond

				out, err := tt.fn(root, opts)
				require.NoError(t, err)

				var roundtrip interface{}
				require.NoError(t, yaml.Unmarshal(out, &roundtrip))

				expectedBytes, _ := yaml.Marshal(root)
				actualBytes, _ := yaml.Marshal(roundtrip)
				assert.YAMLEq(t, string(expectedBytes), string(actualBytes))
			})
		})
	}
}
