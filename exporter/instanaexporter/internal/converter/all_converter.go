// Copyright 2022, OpenTelemetry Authors
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

package converter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/instanaexporter/internal/converter"

import (
	"fmt"

	instanaacceptor "github.com/instana/go-sensor/acceptor"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/instanaexporter/internal/converter/model"
)

var _ Converter = (*ConvertAllConverter)(nil)

type ConvertAllConverter struct {
	converters []Converter
	logger     *zap.Logger
}

func (c *ConvertAllConverter) AcceptsMetrics(attributes pcommon.Map, metricSlice pmetric.MetricSlice) bool {
	return true
}

func (c *ConvertAllConverter) ConvertMetrics(attributes pcommon.Map, metricSlice pmetric.MetricSlice) []instanaacceptor.PluginPayload {
	plugins := make([]instanaacceptor.PluginPayload, 0)

	for i := 0; i < len(c.converters); i++ {
		if !c.converters[i].AcceptsMetrics(attributes, metricSlice) {
			c.logger.Debug(fmt.Sprintf("Converter %s didnt Accept", c.converters[i].Name()))
			continue
		}

		plugins = append(plugins, c.converters[i].ConvertMetrics(attributes, metricSlice)...)
	}

	return plugins
}

func (c *ConvertAllConverter) AcceptsSpans(attributes pcommon.Map, spanSlice ptrace.SpanSlice) bool {
	return true
}

func (c *ConvertAllConverter) ConvertSpans(attributes pcommon.Map, spanSlice ptrace.SpanSlice) model.Bundle {
	bundle := model.NewBundle()

	for i := 0; i < len(c.converters); i++ {
		if !c.converters[i].AcceptsSpans(attributes, spanSlice) {
			c.logger.Warn(fmt.Sprintf("Converter %q didn't accept spans", c.converters[i].Name()))

			continue
		}

		converterBundle := c.converters[i].ConvertSpans(attributes, spanSlice)
		if len(converterBundle.Spans) > 0 {
			bundle.Spans = append(bundle.Spans, converterBundle.Spans...)
		}
	}

	return bundle
}

func (c *ConvertAllConverter) Name() string {
	return "AllConverter"
}

func NewConvertAllConverter(logger *zap.Logger) Converter {
	return &ConvertAllConverter{
		converters: []Converter{
			&DockerContainerMetricConverter{},
			&HostMetricConverter{},
			&ProcessMetricConverter{},
			&CustomMetricsConverter{},
			&CollectorMetricsConverter{},

			// Runtimes
			&RuntimeGoConverter{},
			&RuntimeJavaConverter{},
			&RuntimePythonConverter{},

			&SpanConverter{logger: logger},
		},
		logger: logger,
	}
}
