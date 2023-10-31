// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resourceprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/attraction"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
)

var (
	cfg = &Config{
		AttributesActions: []attraction.ActionKeyValue{
			{Key: "cloud.availability_zone", Value: "zone-1", Action: attraction.UPSERT},
			{Key: "k8s.cluster.name", FromAttribute: "k8s-cluster", Action: attraction.INSERT},
			{Key: "redundant-attribute", Action: attraction.DELETE},
		},
	}
)

func TestResourceProcessorAttributesUpsert(t *testing.T) {
	tests := []struct {
		name             string
		config           *Config
		sourceAttributes map[string]string
		wantAttributes   map[string]string
	}{
		{
			name:             "config_with_attributes_applied_on_nil_resource",
			config:           cfg,
			sourceAttributes: nil,
			wantAttributes: map[string]string{
				"cloud.availability_zone": "zone-1",
			},
		},
		{
			name:             "config_with_attributes_applied_on_empty_resource",
			config:           cfg,
			sourceAttributes: map[string]string{},
			wantAttributes: map[string]string{
				"cloud.availability_zone": "zone-1",
			},
		},
		{
			name:   "config_attributes_applied_on_existing_resource_attributes",
			config: cfg,
			sourceAttributes: map[string]string{
				"cloud.availability_zone": "to-be-replaced",
				"k8s-cluster":             "test-cluster",
				"redundant-attribute":     "to-be-removed",
			},
			wantAttributes: map[string]string{
				"cloud.availability_zone": "zone-1",
				"k8s-cluster":             "test-cluster",
				"k8s.cluster.name":        "test-cluster",
			},
		},
		{
			name: "config_attributes_replacement",
			config: &Config{
				AttributesActions: []attraction.ActionKeyValue{
					{Key: "k8s.cluster.name", FromAttribute: "k8s-cluster", Action: attraction.INSERT},
					{Key: "k8s-cluster", Action: attraction.DELETE},
				},
			},
			sourceAttributes: map[string]string{
				"k8s-cluster": "test-cluster",
			},
			wantAttributes: map[string]string{
				"k8s.cluster.name": "test-cluster",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test trace consumer
			ttn := new(consumertest.TracesSink)

			factory := NewFactory()
			rtp, err := factory.CreateTracesProcessor(context.Background(), processortest.NewNopCreateSettings(), tt.config, ttn)
			require.NoError(t, err)
			assert.True(t, rtp.Capabilities().MutatesData)

			sourceTraceData := generateTraceData(tt.sourceAttributes)
			wantTraceData := generateTraceData(tt.wantAttributes)
			err = rtp.ConsumeTraces(context.Background(), sourceTraceData)
			require.NoError(t, err)
			traces := ttn.AllTraces()
			require.Len(t, traces, 1)
			assert.NoError(t, ptracetest.CompareTraces(wantTraceData, traces[0]))

			// Test metrics consumer
			tmn := new(consumertest.MetricsSink)
			rmp, err := factory.CreateMetricsProcessor(context.Background(), processortest.NewNopCreateSettings(), tt.config, tmn)
			require.NoError(t, err)
			assert.True(t, rtp.Capabilities().MutatesData)

			sourceMetricData := generateMetricData(tt.sourceAttributes)
			wantMetricData := generateMetricData(tt.wantAttributes)
			err = rmp.ConsumeMetrics(context.Background(), sourceMetricData)
			require.NoError(t, err)
			metrics := tmn.AllMetrics()
			require.Len(t, metrics, 1)
			assert.NoError(t, pmetrictest.CompareMetrics(wantMetricData, metrics[0]))

			// Test logs consumer
			tln := new(consumertest.LogsSink)
			rlp, err := factory.CreateLogsProcessor(context.Background(), processortest.NewNopCreateSettings(), tt.config, tln)
			require.NoError(t, err)
			assert.True(t, rtp.Capabilities().MutatesData)

			sourceLogData := generateLogData(tt.sourceAttributes)
			wantLogData := generateLogData(tt.wantAttributes)
			err = rlp.ConsumeLogs(context.Background(), sourceLogData)
			require.NoError(t, err)
			logs := tln.AllLogs()
			require.Len(t, logs, 1)
			assert.NoError(t, plogtest.CompareLogs(wantLogData, logs[0]))
		})
	}
}

func generateTraceData(attributes map[string]string) ptrace.Traces {
	td := testdata.GenerateTracesOneSpanNoResource()
	if attributes == nil {
		return td
	}
	resource := td.ResourceSpans().At(0).Resource()
	for k, v := range attributes {
		resource.Attributes().PutStr(k, v)
	}
	return td
}

func generateMetricData(attributes map[string]string) pmetric.Metrics {
	md := testdata.GenerateMetricsOneMetricNoResource()
	if attributes == nil {
		return md
	}
	resource := md.ResourceMetrics().At(0).Resource()
	for k, v := range attributes {
		resource.Attributes().PutStr(k, v)
	}
	return md
}

func generateLogData(attributes map[string]string) plog.Logs {
	ld := testdata.GenerateLogsOneLogRecordNoResource()
	if attributes == nil {
		return ld
	}
	resource := ld.ResourceLogs().At(0).Resource()
	for k, v := range attributes {
		resource.Attributes().PutStr(k, v)
	}
	return ld
}
