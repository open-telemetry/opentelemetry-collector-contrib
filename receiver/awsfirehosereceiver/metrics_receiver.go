// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsfirehosereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver"

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler/cwmetricstream"
)

const defaultMetricsRecordType = cwmetricstream.TypeStr

// The metricsConsumer implements the firehoseConsumer
// to use a metrics consumer and unmarshaler.
type metricsConsumer struct {
	// consumer passes the translated metrics on to the
	// next consumer.
	consumer consumer.Metrics
	// unmarshaler is the configured pmetric.Unmarshaler
	// to use when processing the records.
	unmarshaler pmetric.Unmarshaler
}

var _ firehoseConsumer = (*metricsConsumer)(nil)

// newMetricsReceiver creates a new instance of the receiver
// with a metricsConsumer.
func newMetricsReceiver(
	config *Config,
	set receiver.Settings,
	unmarshalers map[string]pmetric.Unmarshaler,
	nextConsumer consumer.Metrics,
) (receiver.Metrics, error) {
	recordType := config.RecordType
	if recordType == "" {
		recordType = defaultMetricsRecordType
	}
	configuredUnmarshaler := unmarshalers[recordType]
	if configuredUnmarshaler == nil {
		return nil, fmt.Errorf("%w: recordType = %s", errUnrecognizedRecordType, recordType)
	}

	c := &metricsConsumer{
		consumer:    nextConsumer,
		unmarshaler: configuredUnmarshaler,
	}
	return &firehoseReceiver{
		settings: set,
		config:   config,
		consumer: c,
	}, nil
}

// Consume uses the configured unmarshaler to deserialize the records into a
// single pmetric.Metrics. If there are common attributes available, then it will
// attach those to each of the pcommon.Resources. It will send the final result
// to the next consumer.
func (c *metricsConsumer) Consume(ctx context.Context, nextRecord nextRecordFunc, commonAttributes map[string]string) (int, error) {
	for {
		record, err := nextRecord()
		if errors.Is(err, io.EOF) {
			break
		}
		metrics, err := c.unmarshaler.UnmarshalMetrics(record)
		if err != nil {
			return http.StatusBadRequest, err
		}

		if commonAttributes != nil {
			for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
				rm := metrics.ResourceMetrics().At(i)
				for k, v := range commonAttributes {
					if _, found := rm.Resource().Attributes().Get(k); !found {
						rm.Resource().Attributes().PutStr(k, v)
					}
				}
			}
		}

		if err := c.consumer.ConsumeMetrics(ctx, metrics); err != nil {
			if consumererror.IsPermanent(err) {
				return http.StatusBadRequest, err
			}
			return http.StatusServiceUnavailable, err
		}
	}
	return http.StatusOK, nil
}
