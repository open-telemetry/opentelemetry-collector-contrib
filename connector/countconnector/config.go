// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package countconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/countconnector"

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanevent"
)

// Default metrics are emitted if no conditions are specified.
const (
	defaultMetricNameSpans      = "trace.span.count"
	defaultMetricDescSpans      = "The number of spans observed."
	defaultMetricNameSpanEvents = "trace.span.event.count"
	defaultMetricDescSpanEvents = "The number of span events observed."

	defaultMetricNameMetrics    = "metric.count"
	defaultMetricDescMetrics    = "The number of metrics observed."
	defaultMetricNameDataPoints = "metric.datapoint.count"
	defaultMetricDescDataPoints = "The number of data points observed."

	defaultMetricNameLogRecords = "log.record.count"
	defaultMetricDescLogRecords = "The number of log records observed."
)

// Config for the connector
type Config struct {
	Spans      map[string]MetricInfo `mapstructure:"spans"`
	SpanEvents map[string]MetricInfo `mapstructure:"spanevents"`
	Metrics    map[string]MetricInfo `mapstructure:"metrics"`
	DataPoints map[string]MetricInfo `mapstructure:"datapoints"`
	Logs       map[string]MetricInfo `mapstructure:"logs"`
}

// MetricInfo for a data type
type MetricInfo struct {
	Description string   `mapstructure:"description"`
	Conditions  []string `mapstructure:"conditions"`
}

func (c *Config) Validate() error {
	for name, info := range c.Spans {
		if name == "" {
			return fmt.Errorf("span: metric name missing")
		}
		parser := ottlspan.NewParser(ottlFunctions[ottlspan.TransformContext](), component.TelemetrySettings{})
		if _, err := parseConditions(parser, info.Conditions); err != nil {
			return fmt.Errorf("span condition: %w", err)
		}
	}
	for name, info := range c.SpanEvents {
		if name == "" {
			return fmt.Errorf("span event: metric name missing")
		}
		parser := ottlspanevent.NewParser(ottlFunctions[ottlspanevent.TransformContext](), component.TelemetrySettings{})
		if _, err := parseConditions(parser, info.Conditions); err != nil {
			return fmt.Errorf("span event condition: %w", err)
		}
	}
	for name, info := range c.Metrics {
		if name == "" {
			return fmt.Errorf("metric: metric name missing")
		}
		parser := ottlmetric.NewParser(ottlFunctions[ottlmetric.TransformContext](), component.TelemetrySettings{})
		if _, err := parseConditions(parser, info.Conditions); err != nil {
			return fmt.Errorf("metric condition: %w", err)
		}
	}

	for name, info := range c.DataPoints {
		if name == "" {
			return fmt.Errorf("datapoint: metric name missing")
		}

		parser := ottldatapoint.NewParser(ottlFunctions[ottldatapoint.TransformContext](), component.TelemetrySettings{})
		if _, err := parseConditions(parser, info.Conditions); err != nil {
			return fmt.Errorf("datapoint condition: %w", err)
		}
	}
	for name, info := range c.Logs {
		if name == "" {
			return fmt.Errorf("log: metric name missing")
		}
		parser := ottllog.NewParser(ottlFunctions[ottllog.TransformContext](), component.TelemetrySettings{})
		if _, err := parseConditions(parser, info.Conditions); err != nil {
			return fmt.Errorf("log condition: %w", err)
		}
	}
	return nil
}

var _ confmap.Unmarshaler = (*Config)(nil)

// Unmarshal with custom logic to set default values.
// This is necessary to ensure that default metrics are
// not configured if the user has specified any custom metrics.
func (c *Config) Unmarshal(componentParser *confmap.Conf) error {
	if componentParser == nil {
		// Nothing to do if there is no config given.
		return nil
	}

	if err := componentParser.Unmarshal(c); err != nil {
		return err
	}

	// Set default metric only if no custom metrics are specified.
	if !componentParser.IsSet("spans") {
		c.Spans = createDefaultSpansConfig()
	}
	if !componentParser.IsSet("spanevents") {
		c.SpanEvents = createDefaultSpanEventsConfig()
	}
	if !componentParser.IsSet("metrics") {
		c.Metrics = createDefaultMetricsConfig()
	}
	if !componentParser.IsSet("datapoints") {
		c.DataPoints = createDefaultDataPointsConfig()
	}
	if !componentParser.IsSet("logs") {
		c.Logs = createDefaultLogsConfig()
	}

	return nil
}
