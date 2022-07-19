// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package operator

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	yaml "gopkg.in/yaml.v2"
)

type FakeBuilder struct {
	OperatorID   string   `json:"id" yaml:"id"`
	OperatorType string   `json:"type" yaml:"type"`
	Array        []string `json:"array" yaml:"array"`
}

func (f *FakeBuilder) Build(_ *zap.SugaredLogger) (Operator, error) {
	return nil, nil
}
func (f *FakeBuilder) ID() string     { return "operator" }
func (f *FakeBuilder) Type() string   { return "operator" }
func (f *FakeBuilder) SetID(s string) {}

func TestUnmarshalJSONErrors(t *testing.T) {
	t.Cleanup(func() {
		DefaultRegistry = NewRegistry()
	})

	t.Run("ValidJSON", func(t *testing.T) {
		Register("fake_operator", func() Builder { return &FakeBuilder{} })
		raw := `{"type":"fake_operator"}`
		cfg := &Config{}
		err := cfg.UnmarshalJSON([]byte(raw))
		require.NoError(t, err)
		require.IsType(t, &FakeBuilder{}, cfg.Builder)
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		raw := `{}}`
		cfg := &Config{}
		err := cfg.UnmarshalJSON([]byte(raw))
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid")
	})

	t.Run("MissingType", func(t *testing.T) {
		raw := `{"id":"stdout"}`
		var cfg Config
		err := json.Unmarshal([]byte(raw), &cfg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "missing required field")
	})

	t.Run("UnknownType", func(t *testing.T) {
		raw := `{"id":"stdout","type":"nonexist"}`
		var cfg Config
		err := json.Unmarshal([]byte(raw), &cfg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported type")
	})

	t.Run("TypeSpecificUnmarshal", func(t *testing.T) {
		raw := `{"id":"operator","type":"operator","array":"non-array-value"}`
		Register("operator", func() Builder { return &FakeBuilder{} })
		var cfg Config
		err := json.Unmarshal([]byte(raw), &cfg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot unmarshal string into")
	})
}

func TestMarshalJSON(t *testing.T) {
	cfg := Config{
		Builder: &FakeBuilder{
			OperatorID:   "operator",
			OperatorType: "operator",
			Array:        []string{"test"},
		},
	}
	out, err := json.Marshal(cfg)
	require.NoError(t, err)
	expected := `{"id":"operator","type":"operator","array":["test"]}`
	require.Equal(t, expected, string(out))
}

func TestUnmarshalYAMLErrors(t *testing.T) {
	t.Run("ValidYAML", func(t *testing.T) {
		Register("fake_operator", func() Builder { return &FakeBuilder{} })
		raw := `type: fake_operator`
		var cfg Config
		err := yaml.Unmarshal([]byte(raw), &cfg)
		require.NoError(t, err)
		require.IsType(t, &FakeBuilder{}, cfg.Builder)
	})

	t.Run("InvalidYAML", func(t *testing.T) {
		raw := `-- - \n||\\`
		var cfg Config
		err := yaml.Unmarshal([]byte(raw), &cfg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed ")
	})

	t.Run("MissingType", func(t *testing.T) {
		raw := "id: operator\n"
		var cfg Config
		err := yaml.Unmarshal([]byte(raw), &cfg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "missing required field")
	})

	t.Run("NonStringType", func(t *testing.T) {
		raw := "id: operator\ntype: 123"
		var cfg Config
		err := yaml.Unmarshal([]byte(raw), &cfg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "non-string type")
	})

	t.Run("UnknownType", func(t *testing.T) {
		raw := "id: operator\ntype: unknown\n"
		var cfg Config
		err := yaml.Unmarshal([]byte(raw), &cfg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported type")
	})

	t.Run("TypeSpecificUnmarshal", func(t *testing.T) {
		raw := "id: operator\ntype: operator\narray: nonarray"
		Register("operator", func() Builder { return &FakeBuilder{} })
		var cfg Config
		err := yaml.Unmarshal([]byte(raw), &cfg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot unmarshal !!str")
	})
}

func TestMarshalYAML(t *testing.T) {
	cfg := Config{
		Builder: &FakeBuilder{
			OperatorID:   "operator",
			OperatorType: "operator",
			Array:        []string{"test"},
		},
	}
	out, err := yaml.Marshal(cfg)
	require.NoError(t, err)
	expected := "id: operator\ntype: operator\narray:\n- test\n"
	require.Equal(t, expected, string(out))
}
