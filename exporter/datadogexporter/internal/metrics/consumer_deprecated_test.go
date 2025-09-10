// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"testing"

	"github.com/DataDog/datadog-agent/comp/otelcol/otlp/testutil"
	"github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/attributes"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/otel/semconv/v1.6.1"
	"go.uber.org/zap"
)

func addTestMetric(_ *testing.T, rm pmetric.ResourceMetrics) {
	met := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	met.SetEmptyGauge()
	met.SetName("test.metric")
	met.Gauge().DataPoints().AppendEmpty().SetDoubleValue(1.0)
}

func TestZorkianRunningMetrics(t *testing.T) {
	ms := pmetric.NewMetrics()
	rms := ms.ResourceMetrics()

	rm := rms.AppendEmpty()
	resAttrs := rm.Resource().Attributes()
	resAttrs.PutStr(attributes.AttributeDatadogHostname, "resource-hostname-1")
	addTestMetric(t, rm)

	rm = rms.AppendEmpty()
	resAttrs = rm.Resource().Attributes()
	resAttrs.PutStr(attributes.AttributeDatadogHostname, "resource-hostname-1")
	addTestMetric(t, rm)

	rm = rms.AppendEmpty()
	resAttrs = rm.Resource().Attributes()
	resAttrs.PutStr(attributes.AttributeDatadogHostname, "resource-hostname-2")
	addTestMetric(t, rm)

	rm = rms.AppendEmpty()
	addTestMetric(t, rm)

	logger, _ := zap.NewProduction()
	tr := newTranslator(t, logger)

	ctx := t.Context()
	consumer := NewZorkianConsumer()
	_, err := tr.MapMetrics(ctx, ms, consumer, nil)
	assert.NoError(t, err)

	var runningHostnames []string
	for _, metric := range consumer.runningMetrics(0, component.BuildInfo{}) {
		if metric.Host != nil {
			runningHostnames = append(runningHostnames, *metric.Host)
		}
	}

	assert.ElementsMatch(t,
		runningHostnames,
		[]string{"fallbackHostname", "resource-hostname-1", "resource-hostname-2"},
	)
}

func TestZorkianTagsMetrics(t *testing.T) {
	ms := pmetric.NewMetrics()
	rms := ms.ResourceMetrics()

	rm := rms.AppendEmpty()
	baseAttrs := testutil.NewAttributeMap(map[string]string{
		string(conventions.CloudProviderKey):      conventions.CloudProviderAWS.Value.AsString(),
		string(conventions.CloudPlatformKey):      conventions.CloudPlatformAWSECS.Value.AsString(),
		string(conventions.AWSECSTaskFamilyKey):   "example-task-family",
		string(conventions.AWSECSTaskRevisionKey): "example-task-revision",
		string(conventions.AWSECSLaunchtypeKey):   conventions.AWSECSLaunchtypeFargate.Value.AsString(),
	})
	baseAttrs.CopyTo(rm.Resource().Attributes())
	rm.Resource().Attributes().PutStr(string(conventions.AWSECSTaskARNKey), "task-arn-1")
	addTestMetric(t, rm)

	rm = rms.AppendEmpty()
	baseAttrs.CopyTo(rm.Resource().Attributes())
	rm.Resource().Attributes().PutStr(string(conventions.AWSECSTaskARNKey), "task-arn-2")
	addTestMetric(t, rm)

	rm = rms.AppendEmpty()
	baseAttrs.CopyTo(rm.Resource().Attributes())
	rm.Resource().Attributes().PutStr(string(conventions.AWSECSTaskARNKey), "task-arn-3")
	addTestMetric(t, rm)

	logger, _ := zap.NewProduction()
	tr := newTranslator(t, logger)

	ctx := t.Context()
	consumer := NewZorkianConsumer()
	_, err := tr.MapMetrics(ctx, ms, consumer, nil)
	assert.NoError(t, err)

	runningMetrics := consumer.runningMetrics(0, component.BuildInfo{})
	var runningTags []string
	var runningHostnames []string
	for _, metric := range runningMetrics {
		runningTags = append(runningTags, metric.Tags...)
		if metric.Host != nil {
			runningHostnames = append(runningHostnames, *metric.Host)
		}
	}

	assert.ElementsMatch(t, runningHostnames, []string{"", "", ""})
	assert.Len(t, runningMetrics, 3)
	assert.ElementsMatch(t, runningTags, []string{"task_arn:task-arn-1", "task_arn:task-arn-2", "task_arn:task-arn-3"})
}
