// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package otto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/exporter/otlpexporter"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/components"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachereceiver"
)

func TestFactoriesToComponentTypes(t *testing.T) {
	factories, err := components.Components()
	require.NoError(t, err)
	types := factoriesToComponentTypes(factories)
	assert.NotEmpty(t, types.Metrics.Receivers)
	assert.NotEmpty(t, types.Logs.Processors)
	assert.NotEmpty(t, types.Traces.Exporters)
}

func TestReceiverFactoryToSupportedString(t *testing.T) {
	apacheFactory := apachereceiver.NewFactory()
	apachePipelines := receiverSupportedPipelines(apacheFactory)
	assert.True(t, apachePipelines.metrics)
	assert.False(t, apachePipelines.logs)
	assert.False(t, apachePipelines.traces)

	fpFactory := filterprocessor.NewFactory()
	fpPipelines := processorSupportedPipelines(fpFactory)
	assert.True(t, fpPipelines.metrics)
	assert.True(t, fpPipelines.logs)
	assert.True(t, fpPipelines.traces)

	otlpFactory := otlpexporter.NewFactory()
	otlpPipelines := exporterSupportedPipelines(otlpFactory)
	assert.True(t, otlpPipelines.metrics)
	assert.True(t, otlpPipelines.logs)
	assert.True(t, otlpPipelines.traces)
}
