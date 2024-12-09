// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsfirehosereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver"

import (
	"context"
	"net/http"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler/cwlog"
)

const defaultLogsRecordType = cwlog.TypeStr

// logsConsumer implements the firehoseConsumer
// to use a logs consumer and unmarshaler.
type logsConsumer struct {
	// consumer passes the translated logs on to the
	// next consumer.
	consumer consumer.Logs
	// unmarshaler is the configured LogsUnmarshaler
	// to use when processing the records.
	unmarshaler unmarshaler.LogsUnmarshaler
}

var _ firehoseConsumer = (*logsConsumer)(nil)

// newLogsReceiver creates a new instance of the receiver
// with a logsConsumer.
func newLogsReceiver(
	config *Config,
	set receiver.Settings,
	unmarshalers map[string]unmarshaler.LogsUnmarshaler,
	nextConsumer consumer.Logs,
) (receiver.Logs, error) {
	recordType := config.RecordType
	if recordType == "" {
		recordType = defaultLogsRecordType
	}
	configuredUnmarshaler := unmarshalers[recordType]
	if configuredUnmarshaler == nil {
		return nil, errUnrecognizedRecordType
	}

	mc := &logsConsumer{
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
// single plog.Logs. It will send the final result
// to the next consumer.
func (mc *logsConsumer) Consume(
	ctx context.Context,
	contentType string,
	records [][]byte,
	commonAttributes map[string]string,
) (int, error) {
	md, err := mc.unmarshaler.Unmarshal(records)

	if err != nil {
		return http.StatusBadRequest, err
	}

	if commonAttributes != nil {
		for i := 0; i < md.ResourceLogs().Len(); i++ {
			rm := md.ResourceLogs().At(i)
			for k, v := range commonAttributes {
				if _, found := rm.Resource().Attributes().Get(k); !found {
					rm.Resource().Attributes().PutStr(k, v)
				}
			}
		}
	}

	err = mc.consumer.ConsumeLogs(ctx, md)
	if err != nil {
		if consumererror.IsPermanent(err) {
			return http.StatusBadRequest, err
		}
		return http.StatusServiceUnavailable, err
	}
	return http.StatusOK, nil
}
