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

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metrics"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"gopkg.in/yaml.v2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstransformprocessor"
)

// TransformFunc takes a set of metrics and transforms them.
type TransformFunc func(ctx context.Context, md pmetric.Metrics) error

// NewInfraTransformFunc returns a new TransformFunc which converts OpenTelemetry system and process metrics
// to Datadog compatible ones.
func NewInfraTransformFunc(ctx context.Context, set component.ExporterCreateSettings) (TransformFunc, error) {
	cid := config.NewComponentIDWithName(config.MetricsDataType, "dd-infra-metricstransformer")
	tcfg := &metricstransformprocessor.Config{
		ProcessorSettings: config.NewProcessorSettings(cid),
		Transforms:        metricTransforms.Transforms,
	}
	pset := component.ProcessorCreateSettings(set)
	noop, err := consumer.NewMetrics(
		consumer.ConsumeMetricsFunc(
			func(_ context.Context, _ pmetric.Metrics) error { return nil },
		),
	)
	if err != nil {
		return nil, err
	}
	proc, err := metricstransformprocessor.NewFactory().CreateMetricsProcessor(ctx, pset, tcfg, noop)
	if err != nil {
		return nil, err
	}
	return proc.ConsumeMetrics, err
}

func init() {
	var raw map[string]interface{}
	if err := yaml.Unmarshal(transformYaml, &raw); err != nil {
		panic(fmt.Errorf("Error unmarshalling internal/metrics.(transformYaml): %w", err))
	}
	if err := confmap.NewFromStringMap(raw).Unmarshal(&metricTransforms); err != nil {
		panic(fmt.Errorf("Error unmarshalling confmap: %w", err))
	}
	metricTransforms.Native = make(map[string]struct{})
	for _, t := range metricTransforms.Transforms {
		if t.NewName == "" {
			continue
		}
		metricTransforms.Native[t.NewName] = struct{}{}
	}
}

// metricsTransforms specifies a set of metric transformation rules along with a list
// of their names.
//
// The structure is initialized and populated in the package's init() function.
var metricTransforms struct {
	// Transforms specifies a set of metric transformation rules.
	Transforms []metricstransformprocessor.Transform `mapstructure:"transforms"`
	// Native specifies all metric names covered by the transforms. These are considered
	// native Datadog metrics that should not be prepended by the "otel.*" prefix when
	// submitted.
	Native map[string]struct{} `mapstructure:"-"`
}

var transformYaml = []byte(`
transforms:
- include: system.cpu.load_average.1m
  match_type: strict
  action: insert
  new_name: system.load.1
- include: system.cpu.load_average.5m
  match_type: strict
  action: insert
  new_name: system.load.5
- include: system.cpu.load_average.15m
  match_type: strict
  action: insert
  new_name: system.load.15
- include: system.cpu.utilization
  experimental_match_labels: {"state": "idle"}
  match_type: strict
  action: insert
  new_name: system.cpu.idle
  operations:
  - action: experimental_scale_value
    experimental_scale: 100
- include: system.cpu.utilization
  experimental_match_labels: {"state": "user"}
  match_type: strict
  action: insert
  new_name: system.cpu.user
  operations:
  - action: experimental_scale_value
    experimental_scale: 100
- include: system.cpu.utilization
  experimental_match_labels: {"state": "wait"}
  match_type: strict
  action: insert
  new_name: system.cpu.iowait
  operations:
  - action: experimental_scale_value
    experimental_scale: 100
- include: system.cpu.utilization
  experimental_match_labels: {"state": "steal"}
  match_type: strict
  action: insert
  new_name: system.cpu.stolen
  operations:
  - action: experimental_scale_value
    experimental_scale: 100
- include: system.memory.usage
  match_type: strict
  action: insert
  new_name: system.mem.total
  operations:
  - action: experimental_scale_value
    experimental_scale: 0.000001
- include: system.memory.usage
  experimental_match_labels: {"state": "free"}
  match_type: strict
  action: insert
  new_name: system.mem.usable
- include: system.memory.usage
  experimental_match_labels: {"state": "cached"}
  match_type: strict
  action: insert
  new_name: system.mem.usable
- include: system.memory.usage
  experimental_match_labels: {"state": "buffered"}
  match_type: strict
  action: insert
  new_name: system.mem.usable
- include: system.mem.usable
  match_type: strict
  action: update
  operations:
  - action: aggregate_label_values
    label: state
    aggregated_values: [ "free", "cached", "buffered" ]
    new_value: usable
    aggregation_type: sum
  - action: experimental_scale_value
    experimental_scale: 0.000001
- include: system.network.io
  experimental_match_labels: {"direction": "receive"}
  match_type: strict
  action: insert
  new_name: system.net.bytes_rcvd
  operations:
  - action: experimental_scale_value
    experimental_scale: 0.001
- include: system.network.io
  experimental_match_labels: {"direction": "transmit"}
  match_type: strict
  action: insert
  new_name: system.net.bytes_sent
  operations:
  - action: experimental_scale_value
    experimental_scale: 0.001
- include: system.filesystem.utilization
  match_type: strict
  action: insert
  new_name: system.disk.in_use
`)
