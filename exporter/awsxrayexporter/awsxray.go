// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsxrayexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter"

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/xray"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter/internal/translator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"
	awsxray "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray/telemetry"
)

const (
	maxSegmentsPerPut               = int(50) // limit imposed by PutTraceSegments API
	otlpFormatPrefix                = "T1S"   // X-Ray PutTraceSegment API uses this prefix to detect the format
	otlpFormatKeyIndexAllAttributes = "aws.xray.exporter.config.index_all_attributes"
	otlpFormatKeyIndexAttributes    = "aws.xray.exporter.config.indexed_attributes"
)

// newTracesExporter creates an exporter.Traces that converts to an X-Ray PutTraceSegments
// request and then posts the request to the configured region's X-Ray endpoint.
func newTracesExporter(
	cfg *Config,
	set exporter.Settings,
	cn awsutil.ConnAttr,
	registry telemetry.Registry,
) (exporter.Traces, error) {
	typeLog := zap.String("type", set.ID.Type().String())
	nameLog := zap.String("name", set.ID.String())
	logger := set.Logger

	var xrayClient awsxray.XRayClient
	var sender telemetry.Sender = telemetry.NewNopSender()

	return exporterhelper.NewTracesExporter(
		context.TODO(),
		set,
		cfg,
		func(_ context.Context, td ptrace.Traces) error {
			var err error
			logger.Debug("TracesExporter", typeLog, nameLog, zap.Int("#spans", td.SpanCount()))

			var documents []*string
			if cfg.TransitSpansInOtlpFormat {
				documents, err = encodeOtlpAsBase64(td, cfg)
				if err != nil {
					return err
				}
			} else { // by default use xray format
				documents = extractResourceSpans(cfg, logger, td)
			}

			for offset := 0; offset < len(documents); offset += maxSegmentsPerPut {
				var nextOffset int
				if offset+maxSegmentsPerPut > len(documents) {
					nextOffset = len(documents)
				} else {
					nextOffset = offset + maxSegmentsPerPut
				}
				input := xray.PutTraceSegmentsInput{TraceSegmentDocuments: documents[offset:nextOffset]}
				logger.Debug("request: " + input.String())
				output, localErr := xrayClient.PutTraceSegments(&input)
				if localErr != nil {
					logger.Debug("response error", zap.Error(localErr))
					err = wrapErrorIfBadRequest(localErr) // record error
					sender.RecordConnectionError(localErr)
				} else {
					sender.RecordSegmentsSent(len(input.TraceSegmentDocuments))
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
		exporterhelper.WithStart(func(ctx context.Context, host component.Host) error {
			awsConfig, session, err := awsutil.GetAWSConfigSession(logger, cn, &cfg.AWSSessionSettings)
			if err != nil {
				return err
			}
			xrayClient = awsxray.NewXRayClient(logger, awsConfig, set.BuildInfo, session)

			if cfg.TelemetryConfig.Enabled {
				opts := telemetry.ToOptions(cfg.TelemetryConfig, session, &cfg.AWSSessionSettings)
				opts = append(opts, telemetry.WithLogger(set.Logger))
				sender = registry.Register(set.ID, cfg.TelemetryConfig, xrayClient, opts...)
			}

			sender.Start()
			if cfg.MiddlewareID != nil {
				awsmiddleware.TryConfigure(logger, host, *cfg.MiddlewareID, awsmiddleware.SDKv1(xrayClient.Handlers()))
			}
			return nil
		}),
		exporterhelper.WithShutdown(func(context.Context) error {
			sender.Stop()
			_ = logger.Sync()
			return nil
		}),
	)
}

func extractResourceSpans(config component.Config, logger *zap.Logger, td ptrace.Traces) []*string {
	documents := make([]*string, 0, td.SpanCount())

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

				for l := range documentsForSpan {
					documents = append(documents, &documentsForSpan[l])
				}
			}
		}
	}
	return documents
}

func wrapErrorIfBadRequest(err error) error {
	var rfErr awserr.RequestFailure
	if errors.As(err, &rfErr) && rfErr.StatusCode() < 500 {
		return consumererror.NewPermanent(err)
	}
	return err
}

// encodeOtlpAsBase64 builds bytes from traces and generate base64 value for them
func encodeOtlpAsBase64(td ptrace.Traces, cfg *Config) ([]*string, error) {
	var documents []*string
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		// 1. build a new trace with one resource span
		singleTrace := ptrace.NewTraces()
		td.ResourceSpans().At(i).CopyTo(singleTrace.ResourceSpans().AppendEmpty())

		// 2. append index configuration to resource span as attributes, such that X-Ray Service build indexes based on them.
		injectIndexConfigIntoOtlpPayload(singleTrace.ResourceSpans().At(0), cfg)

		// 3. Marshal single trace into proto bytes
		bytes, err := ptraceotlp.NewExportRequestFromTraces(singleTrace).MarshalProto()
		if err != nil {
			return nil, fmt.Errorf("failed to marshal traces: %w", err)
		}

		// 4. build bytes into base64 and append with PROTOCOL HEADER at the beginning
		base64Str := otlpFormatPrefix + base64.StdEncoding.EncodeToString(bytes)
		documents = append(documents, &base64Str)
	}

	return documents, nil
}

func injectIndexConfigIntoOtlpPayload(resourceSpan ptrace.ResourceSpans, cfg *Config) {
	attributes := resourceSpan.Resource().Attributes()
	attributes.PutBool(otlpFormatKeyIndexAllAttributes, cfg.IndexAllAttributes)
	indexAttributes := attributes.PutEmptySlice(otlpFormatKeyIndexAttributes)
	for _, indexAttribute := range cfg.IndexedAttributes {
		indexAttributes.AppendEmpty().SetStr(indexAttribute)
	}
}
