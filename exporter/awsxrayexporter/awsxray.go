// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsxrayexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter"

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/xray"
	"github.com/aws/smithy-go"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter/internal/translator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"
	awsxray "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray/telemetry"
)

const (
	maxSegmentsPerPut = int(50) // limit imposed by PutTraceSegments API
)

// newTracesExporter creates an exporter.Traces that converts to an X-Ray PutTraceSegments
// request and then posts the request to the configured region's X-Ray endpoint.
func newTracesExporter(ctx context.Context, cfg *Config, set exporter.Settings, registry telemetry.Registry) (exporter.Traces, error) {
	typeLog := zap.String("type", set.ID.Type().String())
	nameLog := zap.String("name", set.ID.String())
	logger := set.Logger
	awsConfig, err := awsutil.GetAWSConfig(logger, &cfg.AWSSessionSettings)
	if err != nil {
		return nil, err
	}
	xrayClient := awsxray.NewXRayClient(logger, awsConfig, set.BuildInfo)
	sender := telemetry.NewNopSender()
	if cfg.TelemetryConfig.Enabled {
		opts := telemetry.ToOptions(ctx, cfg.TelemetryConfig, awsConfig, &cfg.AWSSessionSettings)
		opts = append(opts, telemetry.WithLogger(set.Logger))
		sender = registry.Register(set.ID, cfg.TelemetryConfig, xrayClient, opts...)
	}
	return exporterhelper.NewTraces(context.Background(), set, cfg,
		func(ctx context.Context, td ptrace.Traces) error {
			var err error
			logger.Debug("TracesExporter", typeLog, nameLog, zap.Int("#spans", td.SpanCount()))

			documents := extractResourceSpans(cfg, logger, td)

			for offset := 0; offset < len(documents); offset += maxSegmentsPerPut {
				var nextOffset int
				if offset+maxSegmentsPerPut > len(documents) {
					nextOffset = len(documents)
				} else {
					nextOffset = offset + maxSegmentsPerPut
				}
				input := &xray.PutTraceSegmentsInput{TraceSegmentDocuments: documents[offset:nextOffset]}
				logger.Debug("request: " + fmt.Sprintf("%+v", input))
				output, localErr := xrayClient.PutTraceSegments(ctx, input)
				if localErr != nil {
					logger.Debug("response error", zap.Error(localErr))
					err = wrapErrorIfBadRequest(localErr) // record error
					sender.RecordConnectionError(localErr)
				} else {
					sender.RecordSegmentsSent(len(input.TraceSegmentDocuments))
				}
				if output != nil {
					logger.Debug("response: " + fmt.Sprintf("%+v", output))
				}
				if err != nil {
					break
				}
			}
			return err
		},
		exporterhelper.WithStart(func(context.Context, component.Host) error {
			sender.Start(ctx)
			return nil
		}),
		exporterhelper.WithShutdown(func(context.Context) error {
			sender.Stop()
			_ = logger.Sync()
			return nil
		}),
	)
}

func extractResourceSpans(config component.Config, logger *zap.Logger, td ptrace.Traces) []string {
	documents := make([]string, 0, td.SpanCount())

	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rspans := td.ResourceSpans().At(i)
		resource := rspans.Resource()
		for j := 0; j < rspans.ScopeSpans().Len(); j++ {
			spans := rspans.ScopeSpans().At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				documentsForSpan, localErr := translator.MakeSegmentDocuments(
					spans.At(k), resource,
					config.(*Config).IndexedAttributes,
					config.(*Config).IndexAllAttributes,
					config.(*Config).LogGroupNames,
					config.(*Config).skipTimestampValidation)

				if localErr != nil {
					logger.Debug("Error translating span.", zap.Error(localErr))
					continue
				}

				documents = append(documents, documentsForSpan...)
			}
		}
	}
	return documents
}

func wrapErrorIfBadRequest(err error) error {
	var ae smithy.APIError
	if errors.As(err, &ae) && ae.ErrorFault() == smithy.FaultClient {
		return consumererror.NewPermanent(err)
	}

	return err
}
