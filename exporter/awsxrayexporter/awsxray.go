// Copyright 2019, OpenTelemetry Authors
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

package awsxrayexporter

import (
	"context"

	"github.com/aws/aws-sdk-go/service/xray"
	"github.com/open-telemetry/opentelemetry-collector/config/configmodels"
	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"
	"github.com/open-telemetry/opentelemetry-collector/exporter"
	"github.com/open-telemetry/opentelemetry-collector/exporter/exporterhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter/translator"
)

// NewTraceExporter creates an exporter.TraceExporter that converts to an X-Ray PutTraceSegments
// request and then posts the request to the configured region's X-Ray endpoint.
func NewTraceExporter(config configmodels.Exporter, logger *zap.Logger, cn connAttr) (exporter.TraceExporter, error) {
	typeLog := zap.String("type", config.Type())
	nameLog := zap.String("name", config.Name())
	awsConfig, session, err := GetAWSConfigSession(logger, cn, config.(*Config))
	if err != nil {
		return nil, err
	}
	xrayClient := NewXRay(logger, awsConfig, session)
	return exporterhelper.NewTraceExporter(
		config,
		func(ctx context.Context, td consumerdata.TraceData) (int, error) {
			logger.Debug("TraceExporter", typeLog, nameLog, zap.Int("#spans", len(td.Spans)))
			droppedSpans, input := assembleRequest(td, logger)
			logger.Debug("request: " + input.String())
			output, err := xrayClient.PutTraceSegments(input)
			if config.(*Config).LocalMode {
				err = nil // test mode, ignore errors
			}
			logger.Debug("response: " + output.String())
			if output != nil && output.UnprocessedTraceSegments != nil {
				droppedSpans += len(output.UnprocessedTraceSegments)
			}
			return droppedSpans, err
		},
		exporterhelper.WithTracing(true),
		exporterhelper.WithMetrics(false),
		exporterhelper.WithShutdown(logger.Sync),
	)
}

func assembleRequest(td consumerdata.TraceData, logger *zap.Logger) (int, *xray.PutTraceSegmentsInput) {
	documents := make([]*string, len(td.Spans))
	droppedSpans := int(0)
	for i, span := range td.Spans {
		if span == nil || span.Name == nil {
			droppedSpans++
			continue
		}
		spanName := span.Name.Value
		jsonStr, err := translator.MakeSegmentDocumentString(spanName, span)
		if err != nil {
			droppedSpans++
			logger.Warn("Unable to convert span", zap.Error(err))
		}
		logger.Debug(jsonStr)
		documents[i] = &jsonStr
	}
	return droppedSpans, &xray.PutTraceSegmentsInput{TraceSegmentDocuments: documents}
}
