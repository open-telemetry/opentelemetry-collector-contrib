// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudwatch // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/cloudwatch"

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/pdatautil"

	"go.opentelemetry.io/collector/pdata/plog"
)

type cloudwatchLog struct {
	Timestamp int64  `json:"timestamp"`
	Message   string `json:"message"`
}

// isLogValid validates that the cloudwatch log has been unmarshalled correctly
func isLogValid(log cloudwatchLog) (bool, error) {
	if log.Timestamp == 0 {
		return false, errors.New("cloudwatch log is missing timestamp field")
	}
	if len(log.Message) == 0 {
		return false, errors.New("cloudwatch log message field is empty")
	}
	return true, nil
}

func addRecord(log cloudwatchLog, logs plog.Logs) {
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	logRecord := sl.LogRecords().AppendEmpty()
	logRecord.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(log.Timestamp)))
	logRecord.Body().SetStr(log.Message)
}

func UnmarshalLogs(record []byte) (plog.Logs, error) {
	logs := plog.NewLogs()

	// TODO Check if format (otel or json) matters
	// for cloudwatch logs
	cwLogs := bytes.Split(record, []byte("\n"))
	for datumIndex, datum := range cwLogs {
		var log cloudwatchLog
		decoder := json.NewDecoder(bytes.NewReader(datum))
		if err := decoder.Decode(&log); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return plog.Logs{},
				fmt.Errorf("unable to unmarshal datum [%d] into cloudwatch log: %w", datumIndex, err)
		}
		if valid, err := isLogValid(log); !valid {
			return plog.Logs{},
				fmt.Errorf("cloudwatch log from datum [%d] is invalid: %w", datumIndex, err)
		}
		addRecord(log, logs)
	}

	if logs.LogRecordCount() == 0 {
		return logs, errors.New("no log records could be obtained from the record")
	}
	pdatautil.GroupByResourceLogs(logs.ResourceLogs())
	return logs, nil
}
