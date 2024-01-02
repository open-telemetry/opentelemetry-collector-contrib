// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsfirehosereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver"

import (
	"context"
	"net/http"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler"
)

// The metricsConsumer implements the firehoseConsumer
// to use a metrics consumer and unmarshaler.
type metricsConsumer struct {
	// consumer passes the translated metrics on to the
	// next consumer.
	consumer consumer.Metrics
	// unmarshaler is the configured MetricsUnmarshaler
	// to use when processing the records.
	unmarshaler unmarshaler.MetricsUnmarshaler
}

var _ firehoseConsumer = (*metricsConsumer)(nil)

// newMetricsReceiver creates a new instance of the receiver
// with a metricsConsumer.
func newMetricsReceiver(
	config *Config,
	set receiver.CreateSettings,
	unmarshalers map[string]unmarshaler.MetricsUnmarshaler,
	nextConsumer consumer.Metrics,
) (receiver.Metrics, error) {
	if nextConsumer == nil {
		return nil, component.ErrNilNextConsumer
	}

	configuredUnmarshaler := unmarshalers[config.RecordType]
	if configuredUnmarshaler == nil {
		return nil, errUnrecognizedRecordType
	}

	mc := &metricsConsumer{
		consumer:    nextConsumer,
		unmarshaler: configuredUnmarshaler,
	}

	return &firehoseReceiver{
		settings: set,
		config:   config,
		consumer: mc,
	}, nil
}

// Consume uses the configured unmarshaler to deserialize the records into a
// single pmetric.Metrics. If there are common attributes available, then it will
// attach those to each of the pcommon.Resources. It will send the final result
// to the next consumer.
func (mc *metricsConsumer) Consume(ctx context.Context, records [][]byte, commonAttributes map[string]string) (int, error) {
	md, err := mc.unmarshaler.Unmarshal(records)
	if err != nil {
		return http.StatusBadRequest, err
	}

	if commonAttributes != nil {
		for i := 0; i < md.ResourceMetrics().Len(); i++ {
			rm := md.ResourceMetrics().At(i)
			for k, v := range commonAttributes {
				if _, found := rm.Resource().Attributes().Get(k); !found {
					rm.Resource().Attributes().PutStr(k, v)
				}
			}
		}
	}

	err = mc.consumer.ConsumeMetrics(ctx, md)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return http.StatusOK, nil
}
