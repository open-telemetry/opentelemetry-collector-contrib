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
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler/cwlog"
)

const defaultLogsRecordType = cwlog.TypeStr

// logsConsumer implements the firehoseConsumer
// to use a logs consumer and unmarshaler.
type logsConsumer struct {
	// consumer passes the translated logs on to the
	// next consumer.
	consumer consumer.Logs
	// unmarshaler is the configured plog.Unmarshaler
	// to use when processing the records.
	unmarshaler plog.Unmarshaler
}

var _ firehoseConsumer = (*logsConsumer)(nil)

// newLogsReceiver creates a new instance of the receiver
// with a logsConsumer.
func newLogsReceiver(
	config *Config,
	set receiver.Settings,
	unmarshalers map[string]plog.Unmarshaler,
	nextConsumer consumer.Logs,
) (receiver.Logs, error) {
	recordType := config.RecordType
	if recordType == "" {
		recordType = defaultLogsRecordType
	}
	configuredUnmarshaler := unmarshalers[recordType]
	if configuredUnmarshaler == nil {
		return nil, fmt.Errorf("%w: recordType = %s", errUnrecognizedRecordType, recordType)
	}

	c := &logsConsumer{
		consumer:    nextConsumer,
		unmarshaler: configuredUnmarshaler,
	}
	return &firehoseReceiver{
		settings: set,
		config:   config,
		consumer: c,
	}, nil
}

// Consume uses the configured unmarshaler to unmarshal each record
// into a plog.Logs and pass it to the next consumer, one record at a time.
func (c *logsConsumer) Consume(ctx context.Context, nextRecord nextRecordFunc, commonAttributes map[string]string) (int, error) {
	for {
		record, err := nextRecord()
		if errors.Is(err, io.EOF) {
			break
		}
		logs, err := c.unmarshaler.UnmarshalLogs(record)
		if err != nil {
			return http.StatusBadRequest, err
		}

		if commonAttributes != nil {
			for i := 0; i < logs.ResourceLogs().Len(); i++ {
				rm := logs.ResourceLogs().At(i)
				for k, v := range commonAttributes {
					if _, found := rm.Resource().Attributes().Get(k); !found {
						rm.Resource().Attributes().PutStr(k, v)
					}
				}
			}
		}

		if err := c.consumer.ConsumeLogs(ctx, logs); err != nil {
			if consumererror.IsPermanent(err) {
				return http.StatusBadRequest, err
			}
			return http.StatusServiceUnavailable, err
		}
	}
	return http.StatusOK, nil
}
