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
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/otel/api/core"
	"go.uber.org/zap"
)

const oTelCollectorUserAgentStr = "Honeycomb-OpenTelemetry-Collector"

type HoneycombExporter struct {
	exporter *honeycomb.Exporter
	logger   *zap.Logger
}

func newHoneycombTraceExporter(cfg *Config, logger *zap.Logger) (component.TraceExporterOld, error) {
	exporter, err := honeycomb.NewExporter(honeycomb.Config{
		APIKey: cfg.APIKey,
	},
		honeycomb.TargetingDataset(cfg.Dataset),
		honeycomb.WithAPIURL(cfg.APIURL),
		honeycomb.WithUserAgentAddendum(oTelCollectorUserAgentStr),
		honeycomb.CallingOnError(func(err error) {
			logger.Warn(err.Error())
		}),
	)
	if err != nil {
		return nil, err
	}

	hce := HoneycombExporter{
		exporter: exporter,
		logger:   logger,
	}
	return exporterhelper.NewTraceExporterOld(
		cfg,
		hce.pushTraceData,
		exporterhelper.WithShutdown(hce.Shutdown))
}

func (e *HoneycombExporter) pushTraceData(ctx context.Context, td consumerdata.TraceData) (int, error) {
	var errs []error
	goodSpans := 0

	ctx, cancel := context.WithCancel(ctx)
	go e.exporter.RunErrorLogger(ctx)
	defer cancel()
	for _, span := range td.Spans {
		sd, err := honeycomb.OCProtoSpanToOTelSpanData(span)
		if err == nil {
			if td.Node != nil && td.Node.ServiceInfo != nil {
				serviceName := core.Key("service_name")
				sd.Attributes = append(sd.Attributes,
					serviceName.String(td.Node.ServiceInfo.Name))
			}
			if td.Resource != nil {
				resourceType := td.Resource.GetType()
				if resourceType != "" {
					resourceTypeAttr := core.Key("opencensus.resourcetype")
					sd.Attributes = append(sd.Attributes,
						resourceTypeAttr.String(resourceType))
				}
				sd.Attributes = append(sd.Attributes,
					createAttributes(td.Resource.GetLabels())...)
			}
			if !sd.ParentSpanID.IsValid() || sd.HasRemoteParent {
				if td.Node != nil && td.Node.Attributes != nil {
					sd.Attributes = append(sd.Attributes,
						createAttributes(td.Node.Attributes)...)
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

func createAttributes(attributes map[string]string) []core.KeyValue {
	result := make([]core.KeyValue, len(attributes))
	index := 0
	for key, val := range attributes {
		result[index] = core.Key(key).String(val)
		index++
	}
	return result
}

func (e *HoneycombExporter) Shutdown(context.Context) error {
	e.exporter.Close()
	return nil
}
