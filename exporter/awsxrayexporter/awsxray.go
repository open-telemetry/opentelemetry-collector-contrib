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
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter/translator"
)

const (
	maxSegmentsPerPut = int(50) // limit imposed by PutTraceSegments API
)

// NewTraceExporter creates an component.TraceExporterOld that converts to an X-Ray PutTraceSegments
// request and then posts the request to the configured region's X-Ray endpoint.
func NewTraceExporter(config configmodels.Exporter, logger *zap.Logger, cn connAttr) (component.TraceExporter, error) {
	typeLog := zap.String("type", string(config.Type()))
	nameLog := zap.String("name", config.Name())
	awsConfig, session, err := GetAWSConfigSession(logger, cn, config.(*Config))
	if err != nil {
		return nil, err
	}
	xrayClient := NewXRay(logger, awsConfig, session)
	return exporterhelper.NewTraceExporter(
		config,
		func(ctx context.Context, td pdata.Traces) (totalDroppedSpans int, err error) {
			logger.Debug("TraceExporter", typeLog, nameLog, zap.Int("#spans", td.SpanCount()))
			totalDroppedSpans = 0
			documents := make([]*string, 0, td.SpanCount())
			for i := 0; i < td.ResourceSpans().Len(); i++ {
				rspans := td.ResourceSpans().At(i)
				if rspans.IsNil() {
					continue
				}

				resource := rspans.Resource()
				for j := 0; j < rspans.InstrumentationLibrarySpans().Len(); j++ {
					ispans := rspans.InstrumentationLibrarySpans().At(j)
					if ispans.IsNil() {
						continue
					}

					spans := ispans.Spans()
					for k := 0; k < spans.Len(); k++ {
						span := spans.At(k)
						if span.IsNil() {
							continue
						}

						document, localErr := translator.MakeSegmentDocumentString(span, resource)
						if localErr != nil {
							totalDroppedSpans++
							continue
						}
						documents = append(documents, &document)
					}
				}
			}
			for offset := 0; offset < len(documents); offset += maxSegmentsPerPut {
				nextOffset := offset + maxSegmentsPerPut
				if nextOffset > td.SpanCount() {
					nextOffset = td.SpanCount()
				}
				input := xray.PutTraceSegmentsInput{TraceSegmentDocuments: documents[offset:nextOffset]}
				logger.Debug("request: " + input.String())
				output, localErr := xrayClient.PutTraceSegments(&input)
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

func wrapErrorIfBadRequest(err *error) error {
	_, ok := (*err).(awserr.RequestFailure)
	if ok && (*err).(awserr.RequestFailure).StatusCode() < 500 {
		return consumererror.Permanent(*err)
	}
	return *err
}
