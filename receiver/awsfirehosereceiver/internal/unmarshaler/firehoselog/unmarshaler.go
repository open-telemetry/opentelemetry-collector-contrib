// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package firehoselog // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler/firehoselog"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

const (
	TypeStr = "firehoselogs"
)

// Unmarshaler for the Firehose log record format.
type Unmarshaler struct {
	logger *zap.Logger
}

var _ unmarshaler.LogsUnmarshaler = (*Unmarshaler)(nil)

// NewUnmarshaler creates a new instance of the Unmarshaler.
func NewUnmarshaler(logger *zap.Logger) *Unmarshaler {
	return &Unmarshaler{logger}
}

// Unmarshal deserializes the records into firehoselogs and uses the
// resourceLogsBuilder to group them into a single plog.Logs.
func (u Unmarshaler) Unmarshal(records [][]byte, commonAttributes map[string]string, firehoseARN string, timestamp int64) (plog.Logs, error) {
	md := plog.NewLogs()
	builders := make(map[resourceAttributes]*resourceLogsBuilder)
	for _, record := range records {
		var log firehoseLog
		log.Message = string(record)
		log.Timestamp = timestamp
		attrs := resourceAttributes{
			firehoseARN: firehoseARN,
		}

		lb, ok := builders[attrs]
		if !ok {
			lb = newResourceLogsBuilder(md, attrs)
			builders[attrs] = lb
		}
		lb.AddLog(log)
	}
	return md, nil
}

// Type of the serialized messages.
func (u Unmarshaler) Type() string {
	return TypeStr
}
