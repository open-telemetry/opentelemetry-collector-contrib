package auto

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/gogo/protobuf/proto"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler/cwlog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler/cwmetricstream"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.uber.org/zap"
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

// isJSON returns true if record starts with { and ends with }
func isJSON(record []byte) bool {
	if len(record) < 2 {
		return false
	}
	return record[0] == '{' && record[len(record)-1] == '}'
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

func (u *Unmarshaler) UnmarshalLogs(records [][]byte) (plog.Logs, error) {
	ld := plog.NewLogs()
	cloudwatchLogs := make(map[cwlog.ResourceAttributes]*cwlog.ResourceLogsBuilder)
	for i, record := range records {
		if isJSON(record) {
			if isCloudWatchLog(record) {
				if err := u.addCloudwatchLog(record, cloudwatchLogs, ld); err != nil {
					u.logger.Error(
						"Unable to unmarshal record to cloudwatch log",
						zap.Error(err),
						zap.Int("record_index", i),
					)
				}
			} else {
				u.logger.Error(
					"Unsupported log type",
					zap.Int("record_index", i),
				)
			}
		} else {
			u.logger.Error(
				"Unsupported log type",
				zap.Int("record_index", i),
			)
		}
	}

	if ld.LogRecordCount() == 0 {
		return ld, errInvalidRecords
	}
	return ld, nil
}

func (u *Unmarshaler) addOTLPMetric(record []byte, md pmetric.Metrics) error {
	dataLen, pos := len(record), 0
	for pos < dataLen {
		n, nLen := proto.DecodeVarint(record)
		if nLen == 0 && n == 0 {
			return errors.New("unable to decode data length from message")
		}
		req := pmetricotlp.NewExportRequest()
		pos += nLen
		if err := req.UnmarshalProto(record[pos : pos+int(n)]); err != nil {
			return err
		}
		pos += int(n)
		req.Metrics().ResourceMetrics().MoveAndAppendTo(md.ResourceMetrics())
	}
	return nil
}

func (u *Unmarshaler) UnmarshalMetrics(records [][]byte) (pmetric.Metrics, error) {
	md := pmetric.NewMetrics()
	cloudwatchMetrics := make(map[cwmetricstream.ResourceAttributes]*cwmetricstream.ResourceMetricsBuilder)
	for i, record := range records {
		if isJSON(record) {
			if isCloudwatchMetrics(record) {
				if err := u.addCloudwatchMetric(record, cloudwatchMetrics, md); err != nil {
					u.logger.Error(
						"Unable to unmarshal input",
						zap.Error(err),
						zap.Int("record_index", i),
					)
				}
			} else {
				u.logger.Error(
					"Unsupported metric type",
					zap.Int("record_index", i),
				)
			}
		} else { // is protobuf
			// OTLP metric is the only option currently supported
			if err := u.addOTLPMetric(record, md); err != nil {
				u.logger.Error(
					"failed to unmarshall ExportRequest from proto bytes",
					zap.Int("record_index", i),
					zap.Error(err),
				)
			} else {
				u.logger.Error(
					"Unsupported metric type",
					zap.Int("record_index", i),
				)
			}
		}
	}

	if md.MetricCount() == 0 {
		return md, errInvalidRecords
	}
	return md, nil
}

func (u *Unmarshaler) Type() string {
	return TypeStr
}
