// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cwlog // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler/cwlog"

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"sync"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/klauspost/compress/gzip"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"
)

const (
	TypeStr = "cwlogs"

	attributeAWSCloudWatchLogGroupName  = "aws.cloudwatch.log_group_name"
	attributeAWSCloudWatchLogStreamName = "aws.cloudwatch.log_stream_name"
)

var errInvalidRecords = errors.New("record format invalid")

// Unmarshaler for the CloudWatch Log JSON record format.
type Unmarshaler struct {
	logger   *zap.Logger
	gzipPool sync.Pool
}

var _ plog.Unmarshaler = (*Unmarshaler)(nil)

// NewUnmarshaler creates a new instance of the Unmarshaler.
func NewUnmarshaler(logger *zap.Logger) *Unmarshaler {
	return &Unmarshaler{logger: logger}
}

// Unmarshal deserializes the records into cWLogs and uses the
// resourceLogsBuilder to group them into a single plog.Logs.
// Skips invalid cWLogs received in the record and
func (u *Unmarshaler) UnmarshalLogs(compressedRecord []byte) (plog.Logs, error) {
	var err error
	r, _ := u.gzipPool.Get().(*gzip.Reader)
	if r == nil {
		r, err = gzip.NewReader(bytes.NewReader(compressedRecord))
	} else {
		err = r.Reset(bytes.NewReader(compressedRecord))
	}
	if err != nil {
		return plog.Logs{}, fmt.Errorf("failed to decompress record: %w", err)
	}
	defer u.gzipPool.Put(r)

	type resourceKey struct {
		owner     string
		logGroup  string
		logStream string
	}
	byResource := make(map[resourceKey]plog.LogRecordSlice)

	// Multiple logs in each record separated by newline character
	scanner := bufio.NewScanner(r)
	for datumIndex := 0; scanner.Scan(); datumIndex++ {
		var log cWLog
		if err := jsoniter.ConfigFastest.Unmarshal(scanner.Bytes(), &log); err != nil {
			u.logger.Error(
				"Unable to unmarshal input",
				zap.Error(err),
				zap.Int("datum_index", datumIndex),
			)
			continue
		}
		if !isValid(log) {
			u.logger.Error(
				"Invalid log",
				zap.Int("datum_index", datumIndex),
			)
			continue
		}

		key := resourceKey{
			owner:     log.Owner,
			logGroup:  log.LogGroup,
			logStream: log.LogStream,
		}
		logRecords, ok := byResource[key]
		if !ok {
			logRecords = plog.NewLogRecordSlice()
			byResource[key] = logRecords
		}

		for _, event := range log.LogEvents {
			logRecord := logRecords.AppendEmpty()
			// pcommon.Timestamp is a time specified as UNIX Epoch time in nanoseconds
			// but timestamp in cloudwatch logs are in milliseconds.
			logRecord.SetTimestamp(pcommon.Timestamp(event.Timestamp * int64(time.Millisecond)))
			logRecord.Body().SetStr(event.Message)
		}
	}
	if err := scanner.Err(); err != nil {
		return plog.Logs{}, err
	}
	if len(byResource) == 0 {
		return plog.Logs{}, errInvalidRecords
	}

	logs := plog.NewLogs()
	for resourceKey, logRecords := range byResource {
		rl := logs.ResourceLogs().AppendEmpty()
		resourceAttrs := rl.Resource().Attributes()
		resourceAttrs.PutStr(conventions.AttributeCloudAccountID, resourceKey.owner)
		resourceAttrs.PutStr(attributeAWSCloudWatchLogGroupName, resourceKey.logGroup)
		resourceAttrs.PutStr(attributeAWSCloudWatchLogStreamName, resourceKey.logStream)
		logRecords.MoveAndAppendTo(rl.ScopeLogs().AppendEmpty().LogRecords())
	}
	return logs, nil
}

// isValid validates that the cWLog has been unmarshalled correctly.
func isValid(log cWLog) bool {
	return log.Owner != "" && log.LogGroup != "" && log.LogStream != ""
}

// Type of the serialized messages.
func (u *Unmarshaler) Type() string {
	return TypeStr
}
