// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package schemaprocessor

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func TestMetrics_Rename(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		in              pmetric.Metrics
		out             pmetric.Metrics
		transformations string
		targetVersion   string
	}{
		{
			name: "one_version_downgrade",
			in: func() pmetric.Metrics {
				in, err := golden.ReadMetrics(filepath.Join("testdata", "new-metric.yaml"))
				assert.NoError(t, err, "Failed to read input metrics")
				return in
			}(),
			out: func() pmetric.Metrics {
				out, err := golden.ReadMetrics(filepath.Join("testdata", "old-metric.yaml"))
				assert.NoError(t, err, "Failed to read expected output metrics")
				return out
			}(),
			transformations: `
	1.9.0:
	  all:
		changes:
		  - rename_attributes:
			  attribute_map:
				old.resource.name: new.resource.name
	  metrics:
		changes:
			- rename_attributes:
				  attribute_map:
					old.attr.name: new.attr.name
	    	- rename_metrics:
				  old.sum.metric: new.sum.metric
				  old.gauge.metric: new.gauge.metric
			- rename_metrics:
				  old.histogram.metric: new.histogram.metric
				  old.summary.metric: new.summary.metric
	1.8.0:`,
			targetVersion: "1.8.0",
		},
		{
			name: "one_version_upgrade",
			in: func() pmetric.Metrics {
				in, err := golden.ReadMetrics(filepath.Join("testdata", "old-metric.yaml"))
				assert.NoError(t, err, "Failed to read input metrics")
				return in
			}(),
			out: func() pmetric.Metrics {
				out, err := golden.ReadMetrics(filepath.Join("testdata", "new-metric.yaml"))
				assert.NoError(t, err, "Failed to read expected output metrics")
				return out
			}(),
			transformations: `
	1.9.0:
	  all:
		changes:
		  - rename_attributes:
			  attribute_map:
				old.resource.name: new.resource.name
	  metrics:
		changes:
		  - rename_attributes:
			  attribute_map:
				old.attr.name: new.attr.name
		  - rename_metrics:
				old.sum.metric: new.sum.metric
				old.gauge.metric: new.gauge.metric
		  - rename_metrics:
				old.histogram.metric: new.histogram.metric
				old.summary.metric: new.summary.metric
	1.8.0:`,
			targetVersion: "1.9.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := newTestSchemaProcessor(t, tt.transformations, tt.targetVersion)
			ctx := context.Background()
			out, err := pr.processMetrics(ctx, tt.in)
			if err != nil {
				t.Errorf("Error while processing metrics: %v", err)
			}
			require.NoError(t, pmetrictest.CompareMetrics(tt.out, out, pmetrictest.IgnoreStartTimestamp(),
				pmetrictest.IgnoreTimestamp()))
		})
	}
}

func TestMetrics_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		in              pmetric.Metrics
		errormsg        string
		transformations string
		targetVersion   string
	}{
		{
			name: "target_attribute_already_exists",
			in: func() pmetric.Metrics {
				in, err := golden.ReadMetrics(filepath.Join("testdata", "metric-with-old-attr.yaml"))
				assert.NoError(t, err, "Failed to read input metrics")
				return in
			}(),
			errormsg: "value \"old.attr.name\" already exists",
			transformations: `
	1.9.0:
	  all:
		changes:
		  - rename_attributes:
			  attribute_map:
				old.resource.name: new.resource.name
	  metrics:
		changes:
			- rename_attributes:
				  attribute_map:
					old.attr.name: new.attr.name
	1.8.0:`,
			targetVersion: "1.8.0",
		},
		{
			name: "invalid_schema_translation",
			in: func() pmetric.Metrics {
				in, err := golden.ReadMetrics(filepath.Join("testdata", "metric-with-old-attr.yaml"))
				assert.NoError(t, err, "Failed to read input metrics")
				in.ResourceMetrics().At(0).SetSchemaUrl("invalid_schema_url")
				return in
			}(),
			errormsg: "invalid schema version",
			transformations: `
	1.9.0:
	  all:
		changes:
		  - rename_attributes:
			  attribute_map:
				old.resource.name: new.resource.name
	  metrics:
		changes:
			- rename_attributes:
				  attribute_map:
					old.attr.name: new.attr.name
	1.8.0:`,
			targetVersion: "1.8.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := newTestSchemaProcessor(t, tt.transformations, tt.targetVersion)
			ctx := context.Background()
			_, err := pr.processMetrics(ctx, tt.in)
			require.Error(t, err)
			assert.Equal(t, tt.errormsg, err.Error())
		})
	}
}
