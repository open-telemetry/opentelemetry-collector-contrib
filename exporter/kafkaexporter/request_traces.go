// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"

import (
	"context"
	"errors"
	"fmt"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter/exporterhelper/xexporterhelper"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/kafkaclient"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/kafkarequest"
)

// newTracesRequestConverter builds the converter for the Request-type path.
// It marshals traces to kgo.Records at request-creation time so the queue
// holds pre-serialized data with a precomputed BytesSize.
//
// Records are not pooled here (unlike the immediate-send exportData path),
// because the queue may hold a request well past the converter's return.
func newTracesRequestConverter(exp *kafkaExporter[ptrace.Traces]) xexporterhelper.RequestConverterFunc[ptrace.Traces] {
	return func(ctx context.Context, td ptrace.Traces) (xexporterhelper.Request, error) {
		if exp.messenger == nil {
			return nil, errors.New("kafkaexporter: not started")
		}
		var records []*kgo.Record
		metadataKey := exp.messenger.getMessageKey(ctx)
		for partitionKey, sub := range exp.messenger.partitionData(td) {
			topic := exp.messenger.getTopic(ctx, sub)
			err := exp.messenger.marshalData(sub, func(key, value []byte) {
				if partitionKey != nil {
					key = partitionKey
				} else if metadataKey != nil {
					key = metadataKey
				}
				records = append(records, &kgo.Record{
					Topic: topic,
					Key:   key,
					Value: value,
				})
			})
			if err != nil {
				err = fmt.Errorf("error exporting to topic %q: %w", topic, err)
				exp.logger.Error("kafka traces marshal failed (request path)",
					zap.String("topic", topic),
					zap.Error(err),
				)
				return nil, consumererror.NewPermanent(err)
			}
		}
		return kafkarequest.New(records), nil
	}
}

// newTracesRequestPusher hands the request's records to the producer.
// Headers (config + context-metadata) are applied by ExportData itself,
// matching the immediate-send path behavior.
func newTracesRequestPusher(exp *kafkaExporter[ptrace.Traces]) xexporterhelper.RequestConsumeFunc {
	return func(ctx context.Context, req xexporterhelper.Request) error {
		if exp.producer == nil {
			return errors.New("kafkaexporter: not started")
		}
		kr, ok := req.(*kafkarequest.Request)
		if !ok {
			return fmt.Errorf("kafkaexporter: pusher got %T, expected *kafkarequest.Request", req)
		}
		err := exp.producer.ExportData(ctx, kr.Records())
		if err == nil {
			return nil
		}
		var msgTooLarge *kafkaclient.MessageTooLargeError
		if errors.As(err, &msgTooLarge) {
			exp.logger.Error("kafka record exceeds max message size",
				zap.Int("actual_message_bytes", msgTooLarge.RecordBytes),
				zap.Int("max_message_bytes", msgTooLarge.MaxMessageBytes),
			)
		}
		return err
	}
}
