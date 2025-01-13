package cloudwatch

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/pdatautil"

	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
	"io"
	"time"
)

type cloudwatchLog struct {
	MessageType         string   `json:"messageType"`
	Owner               string   `json:"owner"`
	LogGroup            string   `json:"logGroup"`
	LogStream           string   `json:"logStream"`
	SubscriptionFilters []string `json:"subscriptionFilters"`
	LogEvents           []struct {
		ID        string `json:"id"`
		Timestamp int64  `json:"timestamp"`
		Message   string `json:"message"`
	} `json:"logEvents"`
}

const (
	attributeAWSCloudWatchLogGroupName  = "aws.cloudwatch.log_group_name"
	attributeAWSCloudWatchLogStreamName = "aws.cloudwatch.log_stream_name"
)

var (
	errInvalidLogRecord = errors.New("no resource logs were obtained from the compressed record")
)

// isLogValid validates that the cloudwatch log has been unmarshalled correctly
func isLogValid(log cloudwatchLog) bool {
	return log.Owner != "" && log.LogGroup != "" && log.LogStream != ""
}

func UnmarshalLogs(compressedRecord []byte, logger *zap.Logger) (plog.Logs, error) {
	r, err := gzip.NewReader(bytes.NewReader(compressedRecord))
	if err != nil {
		return plog.Logs{}, fmt.Errorf("failed to create gzip reader from compressed record: %w", err)
	}
	decoder := json.NewDecoder(r)

	logs := plog.NewLogs()
	for datumIndex := 0; ; datumIndex++ {
		var log cloudwatchLog
		if err = decoder.Decode(&log); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			logger.Error(
				"Unable to unmarshal input",
				zap.Int("datum_index", datumIndex),
				zap.Error(err),
			)
			continue
		}
		if !isLogValid(log) {
			logger.Error(
				"Invalid cloudwatch log",
				zap.Int("datum_index", datumIndex),
			)
			continue
		}

		rl := logs.ResourceLogs().AppendEmpty()
		resourceAttrs := rl.Resource().Attributes()
		resourceAttrs.PutStr(conventions.AttributeCloudAccountID, log.Owner)
		resourceAttrs.PutStr(attributeAWSCloudWatchLogGroupName, log.LogGroup)
		resourceAttrs.PutStr(attributeAWSCloudWatchLogStreamName, log.LogStream)

		logRecords := rl.ScopeLogs().AppendEmpty().LogRecords()
		for _, event := range log.LogEvents {
			logRecord := logRecords.AppendEmpty()
			// pcommon.Timestamp is a time specified as UNIX Epoch time in nanoseconds
			// but timestamp in cloudwatch logs are in milliseconds.
			logRecord.SetTimestamp(pcommon.Timestamp(event.Timestamp * int64(time.Millisecond)))
			logRecord.Body().SetStr(event.Message)
		}
	}

	if logs.ResourceLogs().Len() == 0 {
		return logs, errInvalidLogRecord
	}
	pdatautil.GroupByResourceLogs(logs.ResourceLogs())
	return logs, nil
}
