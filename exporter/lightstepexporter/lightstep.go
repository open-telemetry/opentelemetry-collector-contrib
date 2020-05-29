// Copyright 2020 OpenTelemetry Authors
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

package lightstepexporter

import (
	"context"
	"fmt"

	"github.com/lightstep/opentelemetry-exporter-go/lightstep"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/otel/api/kv"
)

// LightStepExporter contains a pointer to a LightStep opentelemetry exporter.
type LightStepExporter struct {
	exporter *lightstep.Exporter
}

func newLightStepTraceExporter(cfg *Config) (component.TraceExporterOld, error) {
	fmt.Println("LS CONFIG", cfg)
	exporter, err := lightstep.NewExporter(
		lightstep.WithAccessToken(cfg.AccessToken),
		lightstep.WithHost(cfg.SatelliteHost),
		lightstep.WithPort(cfg.SatellitePort),
		lightstep.WithServiceName(cfg.ServiceName),
		lightstep.WithPlainText(cfg.PlainText),
	)

	if err != nil {
		return nil, err
	}

	lsExporter := LightStepExporter{
		exporter: exporter,
	}

	return exporterhelper.NewTraceExporterOld(
		cfg,
		lsExporter.pushTraceData,
		exporterhelper.WithShutdown(lsExporter.Shutdown))
}

func (e *LightStepExporter) pushTraceData(ctx context.Context, td consumerdata.TraceData) (int, error) {
	var errs []error
	goodSpans := 0

	for _, span := range td.Spans {
		sd, err := lightstep.OCProtoSpanToOTelSpanData(span)
		if err == nil {
			if td.Node != nil {
				if td.Node.ServiceInfo != nil {
					lightstepServiceName := kv.Key("lightstep.component_name")
					sd.Attributes = append(sd.Attributes, lightstepServiceName.String(td.Node.ServiceInfo.Name))
				}
				if td.Node.Identifier != nil {
					lightstepHostName := kv.Key("lightstep.hostname")
					sd.Attributes = append(sd.Attributes, lightstepHostName.String(td.Node.Identifier.HostName))
				}
			}
			e.exporter.ExportSpan(ctx, sd)
			goodSpans++
		} else {
			errs = append(errs, err)
		}
	}

	return len(td.Spans) - goodSpans, componenterror.CombineErrors(errs)
}

// Shutdown closes the LightStep opentelemetry exporter.
func (e *LightStepExporter) Shutdown(context.Context) error {
	e.exporter.Close()
	return nil
}
