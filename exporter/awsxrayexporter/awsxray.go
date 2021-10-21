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
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter/internal/translator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"
)

const (
	maxSegmentsPerPut = int(50) // limit imposed by PutTraceSegments API
)

// newTracesExporter creates an component.TracesExporter that converts to an X-Ray PutTraceSegments
// request and then posts the request to the configured region's X-Ray endpoint.
func newTracesExporter(
	config config.Exporter, set component.ExporterCreateSettings, cn awsutil.ConnAttr) (component.TracesExporter, error) {
	typeLog := zap.String("type", string(config.ID().Type()))
	nameLog := zap.String("name", config.ID().String())
	logger := set.Logger
	awsConfig, session, err := awsutil.GetAWSConfigSession(logger, cn, &config.(*Config).AWSSessionSettings)
	if err != nil {
		return nil, err
	}
	xrayClient := newXRay(logger, awsConfig, set.BuildInfo, session)
	return exporterhelper.NewTracesExporter(
		config,
		set,
		func(ctx context.Context, td pdata.Traces) error {
			var err error
			logger.Debug("TracesExporter", typeLog, nameLog, zap.Int("#spans", td.SpanCount()))
			documents := make([]*string, 0, td.SpanCount())
			for i := 0; i < td.ResourceSpans().Len(); i++ {
				rspans := td.ResourceSpans().At(i)
				resource := rspans.Resource()
				for j := 0; j < rspans.InstrumentationLibrarySpans().Len(); j++ {
					spans := rspans.InstrumentationLibrarySpans().At(j).Spans()
					for k := 0; k < spans.Len(); k++ {
						document, localErr := translator.MakeSegmentDocumentString(spans.At(k), resource,
							config.(*Config).IndexedAttributes, config.(*Config).IndexAllAttributes)
						if localErr != nil {
							logger.Debug("Error translating span.", zap.Error(localErr))
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
				if localErr != nil {
					logger.Debug("response error", zap.Error(localErr))
					err = wrapErrorIfBadRequest(&localErr) // record error
				}
				if output != nil {
					logger.Debug("response: " + output.String())
				}
				if err != nil {
					break
				}
			}
			return err
		},
		exporterhelper.WithShutdown(func(context.Context) error {
			_ = logger.Sync()
			return nil
		}),
	)
}

func wrapErrorIfBadRequest(err *error) error {
	_, ok := (*err).(awserr.RequestFailure)
	if ok && (*err).(awserr.RequestFailure).StatusCode() < 500 {
		return consumererror.NewPermanent(*err)
	}
	return *err
}
