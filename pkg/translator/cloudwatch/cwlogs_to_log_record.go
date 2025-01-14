package cloudwatch

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"time"

	"go.opentelemetry.io/collector/pdata/plog"
	"io"
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

func addRecord(log cloudwatchLog, records plog.LogRecordSlice) {
	logRecord := records.AppendEmpty()
	logRecord.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(log.Timestamp)))
	logRecord.Body().SetStr(log.Message)
}

func UnmarshalLogs(record []byte) (plog.LogRecordSlice, error) {
	logs := plog.NewLogRecordSlice()
	decoder := json.NewDecoder(bytes.NewReader(record))
	for datumIndex := 0; ; datumIndex++ {
		var log cloudwatchLog
		if err := decoder.Decode(&log); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return plog.LogRecordSlice{},
				fmt.Errorf("unable to unmarshal datum [%d] into cloudwatch log: %w", datumIndex, err)
		}
		if valid, err := isLogValid(log); !valid {
			return plog.LogRecordSlice{},
				fmt.Errorf("cloudwatch log from datum [%d] is invalid: %w", datumIndex, err)
		}
		addRecord(log, logs)
	}

	if logs.Len() == 0 {
		return logs, errors.New("no log records could be obtained from the record")
	}
	return logs, nil
}
