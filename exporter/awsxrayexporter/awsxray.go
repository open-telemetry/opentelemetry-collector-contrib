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

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/xray"
	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/open-telemetry/opentelemetry-collector/component"
	"github.com/open-telemetry/opentelemetry-collector/config/configmodels"
	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"
	"github.com/open-telemetry/opentelemetry-collector/consumer/consumererror"
	"github.com/open-telemetry/opentelemetry-collector/exporter/exporterhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter/translator"
)

const (
	maxSegmentsPerPut = int(50) // limit imposed by PutTraceSegments API
)

// NewTraceExporter creates an component.TraceExporterOld that converts to an X-Ray PutTraceSegments
// request and then posts the request to the configured region's X-Ray endpoint.
func NewTraceExporter(config configmodels.Exporter, logger *zap.Logger, cn connAttr) (component.TraceExporterOld, error) {
	typeLog := zap.String("type", string(config.Type()))
	nameLog := zap.String("name", config.Name())
	awsConfig, session, err := GetAWSConfigSession(logger, cn, config.(*Config))
	if err != nil {
		return nil, err
	}
	xrayClient := NewXRay(logger, awsConfig, session)
	return exporterhelper.NewTraceExporterOld(
		config,
		func(ctx context.Context, td consumerdata.TraceData) (totalDroppedSpans int, err error) {
			logger.Debug("TraceExporter", typeLog, nameLog, zap.Int("#spans", len(td.Spans)))
			totalDroppedSpans = 0
			totalSpans := len(td.Spans)
			for offset := 0; offset < totalSpans; offset += maxSegmentsPerPut {
				nextOffset := offset + maxSegmentsPerPut
				if nextOffset > totalSpans {
					nextOffset = totalSpans
				}
				droppedSpans, input := assembleRequest(td.Spans[offset:nextOffset], logger)
				totalDroppedSpans += droppedSpans
				logger.Debug("request: " + input.String())
				output, localErr := xrayClient.PutTraceSegments(input)
				if localErr != nil && !config.(*Config).LocalMode {
					err = wrapErrorIfBadRequest(&localErr) // not test mode, so record error
				}
				if output != nil {
					logger.Debug("response: " + output.String())
					if output.UnprocessedTraceSegments != nil {
						totalDroppedSpans += len(output.UnprocessedTraceSegments)
					}
				}
				if err != nil {
					break
				}
			}
			return totalDroppedSpans, err
		},
		exporterhelper.WithShutdown(func(context.Context) error {
			return logger.Sync()
		}),
	)
}

func assembleRequest(spans []*tracepb.Span, logger *zap.Logger) (int, *xray.PutTraceSegmentsInput) {
	documents := make([]*string, len(spans))
	droppedSpans := int(0)
	for i, span := range spans {
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

func wrapErrorIfBadRequest(err *error) error {
	_, ok := (*err).(awserr.RequestFailure)
	if ok && (*err).(awserr.RequestFailure).StatusCode() < 500 {
		return consumererror.Permanent(*err)
	}
	return *err
}
