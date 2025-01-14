package firehoseencodingextension

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
	expmetrics "github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/pdatautil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/cloudwatch"
	"github.com/tidwall/gjson"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"
	"io"
)

var (
	_ encoding.LogsUnmarshalerExtension    = (*cloudwatchExtension)(nil)
	_ encoding.MetricsUnmarshalerExtension = (*cloudwatchExtension)(nil)
)

type cloudwatchExtension struct {
	config *Config
	logger *zap.Logger
}

type payload struct {
	RequestID string   `json:"requestId"`
	Timestamp int64    `json:"timestamp"`
	Records   []record `json:"records"`
}

type record struct {
	Data string `json:"data"`
}

type cloudwatchLog struct {
	Owner     string `json:"owner"`
	LogGroup  string `json:"logGroup"`
	LogStream string `json:"logStream"`

	// LogEvents should have the logs data.
	// It is left as raw, so the translator
	// for cloudwatch log can unmarshal it.
	LogEvents json.RawMessage `json:"logEvents"`
}

const (
	attributeAWSCloudWatchLogGroupName  = "aws.cloudwatch.log_group_name"
	attributeAWSCloudWatchLogStreamName = "aws.cloudwatch.log_stream_name"
)

func createExtension(_ context.Context, settings extension.Settings, config component.Config) (extension.Extension, error) {
	return &cloudwatchExtension{
		config: config.(*Config),
		logger: settings.Logger,
	}, nil
}

func (c *cloudwatchExtension) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (c *cloudwatchExtension) Shutdown(_ context.Context) error {
	return nil
}

func decompress(decoded []byte, encoding contentEncoding) ([]byte, error) {
	switch encoding {
	case NoEncoding:
		return decoded, nil
	case GZipEncoded:
		reader, err := gzip.NewReader(bytes.NewReader(decoded))
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer reader.Close()

		decompressed, err := io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("failed to read from gzip reader: %w", err)
		}
		return decompressed, nil
	default:
		// not possible, prevented by config.Validate
		return nil, nil
	}
}

func getRecordsData(buf []byte, encoding contentEncoding) ([][]byte, error) {
	var p payload
	if err := json.Unmarshal(buf, &p); err != nil {
		return nil, fmt.Errorf("failed to unmarshall firehose payload: %w", err)
	}

	var records [][]byte
	for i, rec := range p.Records {
		decoded, err := base64.StdEncoding.DecodeString(rec.Data)
		if err != nil {
			return nil, fmt.Errorf("failed to base64 decode data from record [%d]: %w", i, err)
		}
		decompressed, err := decompress(decoded, encoding)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress decoded data from record [%d]: %w", i, err)
		}
		records = append(records, decompressed)
	}

	return records, nil
}

func isCloudwatchLogValid(log cloudwatchLog) (bool, error) {
	if log.Owner == "" {
		return false, errors.New("cloudwatch log from firehose record data is missing owner field")
	}
	if log.LogGroup == "" {
		return false, errors.New("cloudwatch log from firehose record data is missing log group field")
	}
	if log.LogStream == "" {
		return false, errors.New("cloudwatch log from firehose record data is missing log stream field")
	}
	return true, nil
}

func addLog(log cloudwatchLog, logs plog.Logs) error {
	// Log events is a list of cloudwatch logs.
	// It needs to be sent to the translator
	// as a bytes array that has a new line
	// splitting all logs.
	result := gjson.ParseBytes(log.LogEvents)
	var buf bytes.Buffer
	result.ForEach(func(_, value gjson.Result) bool {
		buf.WriteString(value.Raw)
		buf.WriteByte('\n')
		return true
	})

	logRecords, err := cloudwatch.UnmarshalLogs(buf.Bytes())
	if err != nil {
		return err
	}

	rl := logs.ResourceLogs().AppendEmpty()
	lrs := rl.ScopeLogs().AppendEmpty().LogRecords()
	logRecords.CopyTo(lrs)
	resourceAttrs := rl.Resource().Attributes()
	resourceAttrs.PutStr(conventions.AttributeCloudAccountID, log.Owner)
	resourceAttrs.PutStr(attributeAWSCloudWatchLogGroupName, log.LogGroup)
	resourceAttrs.PutStr(attributeAWSCloudWatchLogStreamName, log.LogStream)
	return nil
}

func (c *cloudwatchExtension) UnmarshalLogs(buf []byte) (plog.Logs, error) {
	allData, e := getRecordsData(buf, c.config.LogsEncoding)
	if e != nil {
		return plog.Logs{}, e
	}

	logs := plog.NewLogs()
	for i, data := range allData {
		var log cloudwatchLog
		if err := json.Unmarshal(data, &log); err != nil {
			return plog.Logs{}, fmt.Errorf("failed to unmarshall cloudwatch log from record data [%d]: %w", i, err)
		}
		if valid, err := isCloudwatchLogValid(log); !valid {
			return plog.Logs{}, fmt.Errorf("cloudwatch log from record data [%d] is invalid: %w", i, err)
		}
		if err := addLog(log, logs); err != nil {
			return plog.Logs{}, fmt.Errorf("failed to add cloudwatch log from record data [%d]: %w", i, err)
		}
	}

	if logs.ResourceLogs().Len() == 0 {
		return logs, errors.New("no resource logs could be obtained from the firehose records")
	}
	pdatautil.GroupByResourceLogs(logs.ResourceLogs())
	return logs, nil
}

func (c *cloudwatchExtension) UnmarshalMetrics(buf []byte) (pmetric.Metrics, error) {
	allData, e := getRecordsData(buf, c.config.MetricsEncoding)
	if e != nil {
		return pmetric.Metrics{}, e
	}

	metrics := pmetric.NewMetrics()
	for i, data := range allData {
		recordMetrics, err := cloudwatch.UnmarshalMetrics(data)
		if err != nil {
			return pmetric.Metrics{}, fmt.Errorf("failed to unmarshall metrics from record data [%d]: %w", i, err)
		}
		metrics = expmetrics.Merge(metrics, recordMetrics)
	}

	if metrics.MetricCount() == 0 {
		return metrics, errors.New("no resource metrics could be obtained from the record")
	}

	metrics = expmetrics.Merge(pmetric.NewMetrics(), metrics)
	return metrics, nil
}
