// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricsgenerationprocessor

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricsgenerationprocessor/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id           component.ID
		expected     component.Config
		errorMessage string
	}{
		{
			id: component.NewIDWithName(metadata.Type, ""),
			expected: &Config{
				Rules: []Rule{
					{
						Name:      "new_metric",
						Unit:      "percent",
						Type:      "calculate",
						Metric1:   "metric1",
						Metric2:   "metric2",
						Operation: "percent",
					},
					{
						Name:      "new_metric",
						Unit:      "unit",
						Type:      "scale",
						Metric1:   "metric1",
						ScaleBy:   1000,
						Operation: "multiply",
					},
				},
			},
		},
		{
			id:           component.NewIDWithName(metadata.Type, "missing_new_metric"),
			errorMessage: fmt.Sprintf("missing required field %q", nameFieldName),
		},
		{
			id:           component.NewIDWithName(metadata.Type, "missing_type"),
			errorMessage: fmt.Sprintf("missing required field %q", typeFieldName),
		},
		{
			id:           component.NewIDWithName(metadata.Type, "invalid_generation_type"),
			errorMessage: fmt.Sprintf("%q must be in %q", typeFieldName, generationTypeKeys()),
		},
		{
			id:           component.NewIDWithName(metadata.Type, "missing_operand1"),
			errorMessage: fmt.Sprintf("missing required field %q", metric1FieldName),
		},
		{
			id:           component.NewIDWithName(metadata.Type, "missing_operand2"),
			errorMessage: fmt.Sprintf("missing required field %q for generation type %q", metric2FieldName, calculate),
		},
		{
			id:           component.NewIDWithName(metadata.Type, "missing_scale_by"),
			errorMessage: fmt.Sprintf("field %q required to be greater than 0 for generation type %q", scaleByFieldName, scale),
		},
		{
			id:           component.NewIDWithName(metadata.Type, "invalid_operation"),
			errorMessage: fmt.Sprintf("%q must be in %q", operationFieldName, operationTypeKeys()),
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
			require.NoError(t, err)

			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()
			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			if tt.expected == nil {
				assert.EqualError(t, component.ValidateConfig(cfg), tt.errorMessage)
				return
			}
			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
