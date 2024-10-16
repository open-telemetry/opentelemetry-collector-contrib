// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cwlog // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler/cwlog"

import (
	"bytes"
	"encoding/json"
	"errors"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler/cwlog/compression"
)

const (
	TypeStr         = "cwlogs"
	recordDelimiter = "\n"
)

var (
	errInvalidRecords = errors.New("record format invalid")
)

// Unmarshaler for the CloudWatch Log JSON record format.
type Unmarshaler struct {
	logger *zap.Logger
}

var _ unmarshaler.LogsUnmarshaler = (*Unmarshaler)(nil)

// NewUnmarshaler creates a new instance of the Unmarshaler.
func NewUnmarshaler(logger *zap.Logger) *Unmarshaler {
	return &Unmarshaler{logger}
}

// Unmarshal deserializes the records into cWLogs and uses the
// resourceLogsBuilder to group them into a single plog.Logs.
// Skips invalid cWLogs received in the record and
func (u Unmarshaler) Unmarshal(records [][]byte) (plog.Logs, error) {
	md := plog.NewLogs()
	builders := make(map[resourceAttributes]*resourceLogsBuilder)
	for recordIndex, compressedRecord := range records {
		record, err := compression.Unzip(compressedRecord)
		if err != nil {
			u.logger.Error("Failed to unzip record",
				zap.Error(err),
				zap.Int("record_index", recordIndex),
			)
			continue
		}
		// Multiple logs in each record separated by newline character
		for datumIndex, datum := range bytes.Split(record, []byte(recordDelimiter)) {
			if len(datum) > 0 {
				var log cWLog
				err := json.Unmarshal(datum, &log)
				if err != nil {
					u.logger.Error(
						"Unable to unmarshal input",
						zap.Error(err),
						zap.Int("datum_index", datumIndex),
						zap.Int("record_index", recordIndex),
					)
					continue
				}
				if !u.isValid(log) {
					u.logger.Error(
						"Invalid log",
						zap.Int("datum_index", datumIndex),
						zap.Int("record_index", recordIndex),
					)
					continue
				}
				attrs := resourceAttributes{
					owner:     log.Owner,
					logGroup:  log.LogGroup,
					logStream: log.LogStream,
				}
				lb, ok := builders[attrs]
				if !ok {
					lb = newResourceLogsBuilder(md, attrs)
					builders[attrs] = lb
				}
				lb.AddLog(log)

			}
		}
	}

	if len(builders) == 0 {
		return plog.NewLogs(), errInvalidRecords
	}

	return md, nil
}

// isValid validates that the cWLog has been unmarshalled correctly.
func (u Unmarshaler) isValid(log cWLog) bool {
	return log.Owner != "" && log.LogGroup != "" && log.LogStream != ""
}

// Type of the serialized messages.
func (u Unmarshaler) Type() string {
	return TypeStr
}
