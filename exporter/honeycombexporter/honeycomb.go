// Copyright 2019 OpenTelemetry Authors
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

package honeycombexporter

import (
	"context"

	"github.com/honeycombio/opentelemetry-exporter-go/honeycomb"
	"github.com/open-telemetry/opentelemetry-collector/component"
	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"
	"github.com/open-telemetry/opentelemetry-collector/exporter/exporterhelper"
	"github.com/open-telemetry/opentelemetry-collector/oterr"
	"go.opentelemetry.io/otel/api/core"
)

const oTelCollectorUserAgentStr = "Honeycomb-OpenTelemetry-Collector"

type HoneycombExporter struct {
	exporter *honeycomb.Exporter
}

func newHoneycombTraceExporter(cfg *Config) (component.TraceExporterOld, error) {
	exporter, err := honeycomb.NewExporter(honeycomb.Config{
		APIKey: cfg.APIKey,
	},
		honeycomb.TargetingDataset(cfg.Dataset),
		honeycomb.WithAPIURL(cfg.APIURL),
		honeycomb.WithUserAgentAddendum(oTelCollectorUserAgentStr),
	)
	if err != nil {
		return nil, err
	}

	hce := HoneycombExporter{
		exporter: exporter,
	}
	return exporterhelper.NewTraceExporterOld(
		cfg,
		hce.pushTraceData,
		exporterhelper.WithShutdown(hce.Shutdown))
}

func (e *HoneycombExporter) pushTraceData(ctx context.Context, td consumerdata.TraceData) (int, error) {
	var errs []error
	goodSpans := 0

	for _, span := range td.Spans {
		sd, err := honeycomb.OCProtoSpanToOTelSpanData(span)
		if err == nil {
			serviceName := core.Key("service_name")
			sd.Attributes = append(sd.Attributes, serviceName.String(td.Node.ServiceInfo.Name))
			e.exporter.ExportSpan(ctx, sd)
			goodSpans++
		} else {
			errs = append(errs, err)
		}
	}

	return len(td.Spans) - goodSpans, oterr.CombineErrors(errs)
}

func (e *HoneycombExporter) Shutdown() error {
	e.exporter.Close()
	return nil
}
