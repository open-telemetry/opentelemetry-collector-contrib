// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	yaml "gopkg.in/yaml.v2"
)

type FakeBuilder struct {
	OperatorID   string   `json:"id" yaml:"id"`
	OperatorType string   `json:"type" yaml:"type"`
	Array        []string `json:"array" yaml:"array"`
}

func (f *FakeBuilder) Build(_ component.TelemetrySettings) (Operator, error) {
	return nil, nil
}
func (f *FakeBuilder) ID() string     { return "operator" }
func (f *FakeBuilder) Type() string   { return "operator" }
func (f *FakeBuilder) SetID(_ string) {}

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
		require.ErrorContains(t, err, "invalid")
	})

	t.Run("MissingType", func(t *testing.T) {
		raw := `{"id":"stdout"}`
		var cfg Config
		err := json.Unmarshal([]byte(raw), &cfg)
		require.ErrorContains(t, err, "missing required field")
	})

	t.Run("UnknownType", func(t *testing.T) {
		raw := `{"id":"stdout","type":"nonexist"}`
		var cfg Config
		err := json.Unmarshal([]byte(raw), &cfg)
		require.ErrorContains(t, err, "unsupported type")
	})

	t.Run("TypeSpecificUnmarshal", func(t *testing.T) {
		raw := `{"id":"operator","type":"operator","array":"non-array-value"}`
		Register("operator", func() Builder { return &FakeBuilder{} })
		var cfg Config
		err := json.Unmarshal([]byte(raw), &cfg)
		require.ErrorContains(t, err, "cannot unmarshal string into")
	})
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
		require.ErrorContains(t, err, "failed ")
	})

	t.Run("MissingType", func(t *testing.T) {
		raw := "id: operator\n"
		var cfg Config
		err := yaml.Unmarshal([]byte(raw), &cfg)
		require.ErrorContains(t, err, "missing required field")
	})

	t.Run("NonStringType", func(t *testing.T) {
		raw := "id: operator\ntype: 123"
		var cfg Config
		err := yaml.Unmarshal([]byte(raw), &cfg)
		require.ErrorContains(t, err, "non-string type")
	})

	t.Run("UnknownType", func(t *testing.T) {
		raw := "id: operator\ntype: unknown\n"
		var cfg Config
		err := yaml.Unmarshal([]byte(raw), &cfg)
		require.ErrorContains(t, err, "unsupported type")
	})

	t.Run("TypeSpecificUnmarshal", func(t *testing.T) {
		raw := "id: operator\ntype: operator\narray: nonarray"
		Register("operator", func() Builder { return &FakeBuilder{} })
		var cfg Config
		err := yaml.Unmarshal([]byte(raw), &cfg)
		require.ErrorContains(t, err, "cannot unmarshal !!str")
	})
}
