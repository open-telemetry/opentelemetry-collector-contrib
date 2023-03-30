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

package prometheusexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry"
)

const (
	// The value of "type" key in configuration.
	typeStr = "prometheus"
	// The stability level of the exporter.
	stability = component.StabilityLevelBeta
)

// NewFactory creates a new Prometheus exporter factory.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		typeStr,
		createDefaultConfig,
		exporter.WithMetrics(createMetricsExporter, stability))
}

func createDefaultConfig() component.Config {
	return &Config{
		ConstLabels:       map[string]string{},
		SendTimestamps:    false,
		MetricExpiration:  time.Minute * 5,
		EnableOpenMetrics: false,
	}
}

func createMetricsExporter(
	ctx context.Context,
	set exporter.CreateSettings,
	cfg component.Config,
) (exporter.Metrics, error) {
	pcfg := cfg.(*Config)

	prometheus, err := newPrometheusExporter(pcfg, set)
	if err != nil {
		return nil, err
	}

	exporter, err := exporterhelper.NewMetricsExporter(
		ctx,
		set,
		cfg,
		prometheus.ConsumeMetrics,
		exporterhelper.WithStart(prometheus.Start),
		exporterhelper.WithShutdown(prometheus.Shutdown),
	)
	if err != nil {
		return nil, err
	}

	return &wrapMetricsExporter{
		Metrics:  resourcetotelemetry.WrapMetricsExporter(pcfg.ResourceToTelemetrySettings, exporter),
		exporter: prometheus,
	}, nil
}

type wrapMetricsExporter struct {
	exporter.Metrics
	exporter *prometheusExporter
}
