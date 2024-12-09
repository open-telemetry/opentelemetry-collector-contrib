package auto

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/gogo/protobuf/proto"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler/cwlog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler/cwlog/compression"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler/cwmetricstream"
)

const (
	TypeStr         = "auto"
	recordDelimiter = "\n"
)

var (
	errInvalidRecords         = errors.New("record format invalid")
	errUnsupportedContentType = errors.New("content type not supported")
	errInvalidFormatStart     = errors.New("unable to decode data length from message")
)

// Unmarshaler for the CloudWatch Log JSON record format.
type Unmarshaler struct {
	logger *zap.Logger
}

var _ unmarshaler.Unmarshaler = (*Unmarshaler)(nil)

// NewUnmarshaler creates a new instance of the Unmarshaler.
func NewUnmarshaler(logger *zap.Logger) *Unmarshaler {
	return &Unmarshaler{logger}
}

// isCloudWatchLog checks if the data has the entries needed to be considered a cloudwatch log
func isCloudWatchLog(data []byte) bool {
	if !bytes.Contains(data, []byte(`"owner":`)) {
		return false
	}
	if !bytes.Contains(data, []byte(`"logGroup":`)) {
		return false
	}
	if !bytes.Contains(data, []byte(`"logStream":`)) {
		return false
	}
	return true
}

// isCloudwatchMetrics checks if the data has the entries needed to be considered a cloudwatch metric
func isCloudwatchMetrics(data []byte) bool {
	if !bytes.Contains(data, []byte(`"metric_name":`)) {
		return false
	}
	if !bytes.Contains(data, []byte(`"namespace":`)) {
		return false
	}
	if !bytes.Contains(data, []byte(`"unit":`)) {
		return false
	}
	if !bytes.Contains(data, []byte(`"value":`)) {
		return false
	}
	return true
}

// addCloudwatchLog unmarshalls the record to a cwlog.CWLog and adds
// it to the logs
func (u *Unmarshaler) addCloudwatchLog(
	record []byte,
	resourceLogs map[cwlog.ResourceAttributes]*cwlog.ResourceLogsBuilder,
	ld plog.Logs,
) error {
	var log cwlog.CWLog
	if err := json.Unmarshal(record, &log); err != nil {
		return err
	}
	attrs := cwlog.ResourceAttributes{
		Owner:     log.Owner,
		LogGroup:  log.LogGroup,
		LogStream: log.LogStream,
	}
	lb, exists := resourceLogs[attrs]
	if !exists {
		lb = cwlog.NewResourceLogsBuilder(ld, attrs)
		resourceLogs[attrs] = lb
	}
	lb.AddLog(log)
	return nil
}

// addCloudwatchMetric unmarshalls the record to a cwmetric.CWMetric and adds
// it to the metrics
func (u *Unmarshaler) addCloudwatchMetric(
	record []byte,
	resourceMetrics map[cwmetricstream.ResourceAttributes]*cwmetricstream.ResourceMetricsBuilder,
	md pmetric.Metrics,
) error {
	var metric cwmetricstream.CWMetric
	if err := json.Unmarshal(record, &metric); err != nil {
		return err
	}
	attrs := cwmetricstream.ResourceAttributes{
		MetricStreamName: metric.MetricStreamName,
		Namespace:        metric.Namespace,
		AccountID:        metric.AccountID,
		Region:           metric.Region,
	}
	mb, exists := resourceMetrics[attrs]
	if !exists {
		mb = cwmetricstream.NewResourceMetricsBuilder(md, attrs)
		resourceMetrics[attrs] = mb
	}
	mb.AddMetric(metric)
	return nil
}

// unmarshalJSONMetrics tries to unmarshal each JSON record as a metric
// and returns all valid metrics
func (u *Unmarshaler) unmarshalJSONMetrics(records [][]byte) (pmetric.Metrics, error) {
	md := pmetric.NewMetrics()
	resourceMetrics := make(map[cwmetricstream.ResourceAttributes]*cwmetricstream.ResourceMetricsBuilder)
	for i, compressed := range records {
		record, err := compression.Unzip(compressed)
		if err != nil {
			u.logger.Error("Failed to unzip record",
				zap.Error(err),
				zap.Int("record_index", i),
			)
			continue
		}
		for j, single := range bytes.Split(record, []byte(recordDelimiter)) {
			if len(single) == 0 {
				continue
			}

			if isCloudwatchMetrics(single) {
				if err = u.addCloudwatchMetric(single, resourceMetrics, md); err != nil {
					u.logger.Error(
						"Unable to unmarshal input",
						zap.Error(err),
						zap.Int("single_index", j),
						zap.Int("record_index", i),
					)
					continue
				}
			} else {
				u.logger.Error(
					"Unsupported metric type",
					zap.Int("record_index", i),
					zap.Int("single_index", j),
				)
				continue
			}
		}
	}

	if len(resourceMetrics) == 0 {
		return md, errInvalidRecords
	}
	return md, nil
}

// unmarshalJSONLogs tries to unmarshal each JSON record as a log
// and returns all valid logs
func (u *Unmarshaler) unmarshalJSONLogs(records [][]byte) (plog.Logs, error) {
	ld := plog.NewLogs()
	resourceLogs := make(map[cwlog.ResourceAttributes]*cwlog.ResourceLogsBuilder)
	for i, compressed := range records {
		record, err := compression.Unzip(compressed)
		if err != nil {
			u.logger.Error("Failed to unzip record",
				zap.Error(err),
				zap.Int("record_index", i),
			)
			continue
		}

		for j, single := range bytes.Split(record, []byte(recordDelimiter)) {
			if len(single) == 0 {
				continue
			}

			if isCloudWatchLog(single) {
				if err = u.addCloudwatchLog(single, resourceLogs, ld); err != nil {
					u.logger.Error(
						"Unable to unmarshal record to cloudwatch log",
						zap.Error(err),
						zap.Int("record_index", i),
						zap.Int("single_index", j),
					)
					continue
				}
			} else {
				u.logger.Error(
					"Unsupported log type",
					zap.Int("record_index", i),
					zap.Int("single_index", j),
				)
				continue
			}
		}
	}

	if len(resourceLogs) == 0 {
		return ld, errInvalidRecords
	}
	return ld, nil
}

// unmarshallProtobufMetrics tries to unmarshal every record and
// returns all valid metrics from these records
func (u *Unmarshaler) unmarshallProtobufMetrics(records [][]byte) (pmetric.Metrics, error) {
	md := pmetric.NewMetrics()
	for recordIndex, record := range records {
		dataLen, pos := len(record), 0
		for pos < dataLen {
			n, nLen := proto.DecodeVarint(record)
			if nLen == 0 && n == 0 {
				return md, errInvalidFormatStart
			}
			req := pmetricotlp.NewExportRequest()
			pos += nLen
			err := req.UnmarshalProto(record[pos : pos+int(n)])
			pos += int(n)
			if err != nil {
				u.logger.Error(
					"Unable to unmarshal input",
					zap.Error(err),
					zap.Int("record_index", recordIndex),
				)
				continue
			}
			req.Metrics().ResourceMetrics().MoveAndAppendTo(md.ResourceMetrics())
		}
	}

	return md, nil
}

func (u *Unmarshaler) UnmarshalLogs(contentType string, records [][]byte) (plog.Logs, error) {
	switch contentType {
	case "application/json":
		return u.unmarshalJSONLogs(records)
	default:
		return plog.NewLogs(), errUnsupportedContentType
	}
}

func (u *Unmarshaler) UnmarshalMetrics(contentType string, records [][]byte) (pmetric.Metrics, error) {
	switch contentType {
	case "application/json":
		return u.unmarshalJSONMetrics(records)
	case "application/x-protobuf":
		return u.unmarshallProtobufMetrics(records)
	default:
		return pmetric.NewMetrics(), errUnsupportedContentType
	}
}

func (u *Unmarshaler) Type() string {
	return TypeStr
}
