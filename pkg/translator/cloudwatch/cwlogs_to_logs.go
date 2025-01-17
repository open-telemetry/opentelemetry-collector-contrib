// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudwatch // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/cloudwatch"

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"

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
	decoder := json.NewDecoder(bytes.NewReader(record))
	var log cloudwatchLog
	if err := decoder.Decode(&log); err != nil {
		return plog.Logs{},
			fmt.Errorf("unable to unmarshal data into cloudwatch log: %w", err)
	}
	if valid, err := isLogValid(log); !valid {
		return plog.Logs{},
			fmt.Errorf("cloudwatch log is invalid: %w", err)
	}
	addRecord(log, logs)
	return logs, nil
}
