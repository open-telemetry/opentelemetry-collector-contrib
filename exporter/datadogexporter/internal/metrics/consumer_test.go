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

package metrics

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/model/attributes"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/model/source"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/model/translator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/testutils"
)

type testProvider string

func (t testProvider) Source(context.Context) (source.Source, error) {
	return source.Source{Kind: source.HostnameKind, Identifier: string(t)}, nil
}

func newTranslator(t *testing.T, logger *zap.Logger) *translator.Translator {
	tr, err := translator.New(logger,
		translator.WithHistogramMode(translator.HistogramModeDistributions),
		translator.WithNumberMode(translator.NumberModeCumulativeToDelta),
		translator.WithFallbackSourceProvider(testProvider("fallbackHostname")),
	)
	require.NoError(t, err)
	return tr
}

func TestRunningMetrics(t *testing.T) {
	ms := pmetric.NewMetrics()
	rms := ms.ResourceMetrics()

	rm := rms.AppendEmpty()
	resAttrs := rm.Resource().Attributes()
	resAttrs.Insert(attributes.AttributeDatadogHostname, pcommon.NewValueString("resource-hostname-1"))

	rm = rms.AppendEmpty()
	resAttrs = rm.Resource().Attributes()
	resAttrs.Insert(attributes.AttributeDatadogHostname, pcommon.NewValueString("resource-hostname-1"))

	rm = rms.AppendEmpty()
	resAttrs = rm.Resource().Attributes()
	resAttrs.Insert(attributes.AttributeDatadogHostname, pcommon.NewValueString("resource-hostname-2"))

	rms.AppendEmpty()

	logger, _ := zap.NewProduction()
	tr := newTranslator(t, logger)

	ctx := context.Background()
	consumer := NewConsumer()
	assert.NoError(t, tr.MapMetrics(ctx, ms, consumer))

	runningHostnames := []string{}
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

func TestTagsMetrics(t *testing.T) {
	ms := pmetric.NewMetrics()
	rms := ms.ResourceMetrics()

	rm := rms.AppendEmpty()
	baseAttrs := testutils.NewAttributeMap(map[string]string{
		conventions.AttributeCloudProvider:      conventions.AttributeCloudProviderAWS,
		conventions.AttributeCloudPlatform:      conventions.AttributeCloudPlatformAWSECS,
		conventions.AttributeAWSECSTaskFamily:   "example-task-family",
		conventions.AttributeAWSECSTaskRevision: "example-task-revision",
		conventions.AttributeAWSECSLaunchtype:   conventions.AttributeAWSECSLaunchtypeFargate,
	})
	baseAttrs.CopyTo(rm.Resource().Attributes())
	rm.Resource().Attributes().InsertString(conventions.AttributeAWSECSTaskARN, "task-arn-1")

	rm = rms.AppendEmpty()
	baseAttrs.CopyTo(rm.Resource().Attributes())
	rm.Resource().Attributes().InsertString(conventions.AttributeAWSECSTaskARN, "task-arn-2")

	rm = rms.AppendEmpty()
	baseAttrs.CopyTo(rm.Resource().Attributes())
	rm.Resource().Attributes().InsertString(conventions.AttributeAWSECSTaskARN, "task-arn-3")

	logger, _ := zap.NewProduction()
	tr := newTranslator(t, logger)

	ctx := context.Background()
	consumer := NewConsumer()
	assert.NoError(t, tr.MapMetrics(ctx, ms, consumer))

	runningMetrics := consumer.runningMetrics(0, component.BuildInfo{})
	runningTags := []string{}
	runningHostnames := []string{}
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
