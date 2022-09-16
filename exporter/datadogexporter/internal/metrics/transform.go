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

// Transformer is able to transform a set of metrics by either modifying or adding
// new ones.
type Transformer struct {
	names    map[string]struct{}
	consumer func(ctx context.Context, md pmetric.Metrics) error
}

// Transform applies a set of changes to the given set of metrics.
func (t *Transformer) Transform(ctx context.Context, md pmetric.Metrics) error {
	return t.consumer(ctx, md)
}

// Has reports whether the metric with the given name was or would be added by the
// transformer.
func (t *Transformer) Has(name string) bool {
	_, ok := t.names[name]
	return ok
}

// NewInfraTransformer returns a new Transformer which converts OpenTelemetry system and process metrics
// to Datadog compatible ones. It is be based on the given ExporterCreateSettings and its name is derived
// from the given component id.
func NewInfraTransformer(ctx context.Context, set component.ExporterCreateSettings, id config.ComponentID) (*Transformer, error) {
	cid := config.NewComponentIDWithName(config.MetricsDataType, id.String()+"/infra-metricstransformer")
	cfg := &metricstransformprocessor.Config{
		ProcessorSettings: config.NewProcessorSettings(cid),
	}
	if err := yamlToConfig(cfg); err != nil {
		return nil, fmt.Errorf("error unmarshalling yaml to config: %w", err)
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
	proc, err := metricstransformprocessor.NewFactory().CreateMetricsProcessor(ctx, pset, cfg, noop)
	if err != nil {
		return nil, err
	}
	transformer := Transformer{
		names:    make(map[string]struct{}),
		consumer: proc.ConsumeMetrics,
	}
	for _, t := range cfg.Transforms {
		if t.NewName != "" {
			transformer.names[t.NewName] = struct{}{}
		}
	}
	return &transformer, err
}

func yamlToConfig(cfg *metricstransformprocessor.Config) error {
	var raw map[string]interface{}
	if err := yaml.Unmarshal(transformYaml, &raw); err != nil {
		return err
	}
	return confmap.NewFromStringMap(raw).Unmarshal(&cfg)
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
  experimental_match_labels: {"state": "system"}
  match_type: strict
  action: insert
  new_name: system.cpu.system
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
- include: system.network.io
  experimental_match_labels: {"direction": "transmit"}
  match_type: strict
  action: insert
  new_name: system.net.bytes_sent
- include: system.paging.usage
  experimental_match_labels: {"state": "free"}
  match_type: strict
  action: insert
  new_name: system.swap.free
- include: system.paging.usage
  experimental_match_labels: {"state": "used"}
  match_type: strict
  action: insert
  new_name: system.swap.used
- include: system.filesystem.utilization
  match_type: strict
  action: insert
  new_name: system.disk.in_use
`)
