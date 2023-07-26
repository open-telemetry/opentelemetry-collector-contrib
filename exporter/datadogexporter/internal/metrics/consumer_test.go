// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"
	"testing"

	"github.com/DataDog/datadog-agent/pkg/trace/pb"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes/source"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/testutil"
)

type testProvider string

func (t testProvider) Source(context.Context) (source.Source, error) {
	return source.Source{Kind: source.HostnameKind, Identifier: string(t)}, nil
}

func newTranslator(t *testing.T, logger *zap.Logger) *metrics.Translator {
	tr, err := metrics.NewTranslator(logger,
		metrics.WithHistogramMode(metrics.HistogramModeDistributions),
		metrics.WithNumberMode(metrics.NumberModeCumulativeToDelta),
		metrics.WithFallbackSourceProvider(testProvider("fallbackHostname")),
	)
	require.NoError(t, err)
	return tr
}

func TestRunningMetrics(t *testing.T) {
	ms := pmetric.NewMetrics()
	rms := ms.ResourceMetrics()

	rm := rms.AppendEmpty()
	resAttrs := rm.Resource().Attributes()
	resAttrs.PutStr(attributes.AttributeDatadogHostname, "resource-hostname-1")

	rm = rms.AppendEmpty()
	resAttrs = rm.Resource().Attributes()
	resAttrs.PutStr(attributes.AttributeDatadogHostname, "resource-hostname-1")

	rm = rms.AppendEmpty()
	resAttrs = rm.Resource().Attributes()
	resAttrs.PutStr(attributes.AttributeDatadogHostname, "resource-hostname-2")

	rms.AppendEmpty()

	logger, _ := zap.NewProduction()
	tr := newTranslator(t, logger)

	ctx := context.Background()
	consumer := NewConsumer()
	metadata, err := tr.MapMetrics(ctx, ms, consumer)
	assert.NoError(t, err)

	var runningHostnames []string
	for _, metric := range consumer.runningMetrics(0, component.BuildInfo{}, metadata) {
		for _, res := range metric.Resources {
			runningHostnames = append(runningHostnames, *res.Name)
		}
	}

	assert.ElementsMatch(t,
		runningHostnames,
		[]string{"fallbackHostname", "resource-hostname-1", "resource-hostname-2"},
	)
}

func TestTagsMetrics(t *testing.T) {
	ms := pmetric.NewMetrics()
	rms := ms.ResourceMetrics()

	rm := rms.AppendEmpty()
	baseAttrs := testutil.NewAttributeMap(map[string]string{
		conventions.AttributeCloudProvider:      conventions.AttributeCloudProviderAWS,
		conventions.AttributeCloudPlatform:      conventions.AttributeCloudPlatformAWSECS,
		conventions.AttributeAWSECSTaskFamily:   "example-task-family",
		conventions.AttributeAWSECSTaskRevision: "example-task-revision",
		conventions.AttributeAWSECSLaunchtype:   conventions.AttributeAWSECSLaunchtypeFargate,
	})
	baseAttrs.CopyTo(rm.Resource().Attributes())
	rm.Resource().Attributes().PutStr(conventions.AttributeAWSECSTaskARN, "task-arn-1")

	rm = rms.AppendEmpty()
	baseAttrs.CopyTo(rm.Resource().Attributes())
	rm.Resource().Attributes().PutStr(conventions.AttributeAWSECSTaskARN, "task-arn-2")

	rm = rms.AppendEmpty()
	baseAttrs.CopyTo(rm.Resource().Attributes())
	rm.Resource().Attributes().PutStr(conventions.AttributeAWSECSTaskARN, "task-arn-3")

	logger, _ := zap.NewProduction()
	tr := newTranslator(t, logger)

	ctx := context.Background()
	consumer := NewConsumer()
	metadata, err := tr.MapMetrics(ctx, ms, consumer)
	assert.NoError(t, err)

	runningMetrics := consumer.runningMetrics(0, component.BuildInfo{}, metadata)
	var runningTags []string
	var runningHostnames []string
	for _, metric := range runningMetrics {
		runningTags = append(runningTags, metric.Tags...)
		for _, res := range metric.Resources {
			runningHostnames = append(runningHostnames, *res.Name)
		}
	}

	assert.ElementsMatch(t, runningHostnames, []string{"", "", ""})
	assert.Len(t, runningMetrics, 3)
	assert.ElementsMatch(t, runningTags, []string{"task_arn:task-arn-1", "task_arn:task-arn-2", "task_arn:task-arn-3"})
}

func TestConsumeAPMStats(t *testing.T) {
	var md metrics.Metadata
	c := NewConsumer()
	for _, sp := range testutil.StatsPayloads {
		c.ConsumeAPMStats(sp)
	}
	require.Len(t, c.as, len(testutil.StatsPayloads))
	require.ElementsMatch(t, c.as, testutil.StatsPayloads)
	_, _, out := c.All(0, component.BuildInfo{}, []string{}, md)
	require.ElementsMatch(t, out, testutil.StatsPayloads)
	_, _, out = c.All(0, component.BuildInfo{}, []string{"extra:key"}, md)
	var copies []pb.ClientStatsPayload
	for _, sp := range testutil.StatsPayloads {
		sp.Tags = append(sp.Tags, "extra:key")
		copies = append(copies, sp)
	}
	require.ElementsMatch(t, out, copies)
}
