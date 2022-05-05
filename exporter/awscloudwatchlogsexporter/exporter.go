// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// nolint:errcheck
package awscloudwatchlogsexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awscloudwatchlogsexporter"

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/google/uuid"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/cwlogs"
)

type exporter struct {
	Config           *Config
	logger           *zap.Logger
	retryCount       int
	collectorID      string
	svcStructuredLog *cwlogs.Client
	pusher           cwlogs.Pusher
}

func newCwLogsPusher(expConfig *Config, params component.ExporterCreateSettings) (component.LogsExporter, error) {
	if expConfig == nil {
		return nil, errors.New("awscloudwatchlogs exporter config is nil")
	}

	expConfig.logger = params.Logger

	// create AWS session
	awsConfig, session, err := awsutil.GetAWSConfigSession(params.Logger, &awsutil.Conn{}, &expConfig.AWSSessionSettings)
	if err != nil {
		return nil, err
	}

	// create CWLogs client with aws session config
	svcStructuredLog := cwlogs.NewClient(params.Logger, awsConfig, params.BuildInfo, expConfig.LogGroupName, session)
	collectorIdentifier, err := uuid.NewRandom()

	if err != nil {
		return nil, err
	}

	expConfig.Validate()

	pusher := cwlogs.NewPusher(aws.String(expConfig.LogGroupName), aws.String(expConfig.LogStreamName), *awsConfig.MaxRetries, *svcStructuredLog, params.Logger)

	logsExporter := &exporter{
		svcStructuredLog: svcStructuredLog,
		Config:           expConfig,
		logger:           params.Logger,
		retryCount:       *awsConfig.MaxRetries,
		collectorID:      collectorIdentifier.String(),
		pusher:           pusher,
	}
	return logsExporter, nil
}

func newCwLogsExporter(config config.Exporter, params component.ExporterCreateSettings) (component.LogsExporter, error) {
	expConfig := config.(*Config)
	logsExporter, err := newCwLogsPusher(expConfig, params)
	if err != nil {
		return nil, err
	}
	return exporterhelper.NewLogsExporter(
		config,
		params,
		logsExporter.ConsumeLogs,
		exporterhelper.WithQueue(expConfig.enforcedQueueSettings()),
		exporterhelper.WithRetry(expConfig.RetrySettings),
	)

}

func (e *exporter) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	cwLogsPusher := e.pusher
	logEvents, _ := logsToCWLogs(e.logger, ld)
	if len(logEvents) == 0 {
		return nil
	}

	for _, logEvent := range logEvents {
		logEvent := &cwlogs.Event{
			InputLogEvent: logEvent,
			GeneratedTime: time.Now(),
		}
		e.logger.Debug("Adding log event", zap.Any("event", logEvent))
		err := cwLogsPusher.AddLogEntry(logEvent)
		if err != nil {
			e.logger.Error("Failed ", zap.Int("num_of_events", len(logEvents)))
		}
	}
	e.logger.Debug("Log events are successfully put")
	flushErr := cwLogsPusher.ForceFlush()
	if flushErr != nil {
		e.logger.Error("Error force flushing logs. Skipping to next logPusher.", zap.Error(flushErr))
		return flushErr
	}
	return nil
}

func (e *exporter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (e *exporter) Shutdown(ctx context.Context) error {
	if e.pusher != nil {
		e.pusher.ForceFlush()
	}
	return nil
}

func (e *exporter) Start(ctx context.Context, host component.Host) error {
	return nil
}

func logsToCWLogs(logger *zap.Logger, ld plog.Logs) ([]*cloudwatchlogs.InputLogEvent, int) {
	n := ld.ResourceLogs().Len()
	if n == 0 {
		return []*cloudwatchlogs.InputLogEvent{}, 0
	}

	var dropped int
	out := make([]*cloudwatchlogs.InputLogEvent, 0) // TODO(jbd): set a better capacity

	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		rl := rls.At(i)
		resourceAttrs := attrsValue(rl.Resource().Attributes())

		sls := rl.ScopeLogs()
		for j := 0; j < sls.Len(); j++ {
			sl := sls.At(j)
			logs := sl.LogRecords()
			for k := 0; k < logs.Len(); k++ {
				log := logs.At(k)
				event, err := logToCWLog(resourceAttrs, log)
				if err != nil {
					logger.Debug("Failed to convert to CloudWatch Log", zap.Error(err))
					dropped++
				} else {
					out = append(out, event)
				}
			}
		}
	}
	return out, dropped
}

type cwLogBody struct {
	Body                   interface{}            `json:"body,omitempty"`
	SeverityNumber         int32                  `json:"severity_number,omitempty"`
	SeverityText           string                 `json:"severity_text,omitempty"`
	DroppedAttributesCount uint32                 `json:"dropped_attributes_count,omitempty"`
	Flags                  uint32                 `json:"flags,omitempty"`
	TraceID                string                 `json:"trace_id,omitempty"`
	SpanID                 string                 `json:"span_id,omitempty"`
	Attributes             map[string]interface{} `json:"attributes,omitempty"`
	Resource               map[string]interface{} `json:"resource,omitempty"`
}

func logToCWLog(resourceAttrs map[string]interface{}, log plog.LogRecord) (*cloudwatchlogs.InputLogEvent, error) {
	// TODO(jbd): Benchmark and improve the allocations.
	// Evaluate go.elastic.co/fastjson as a replacement for encoding/json.
	body := cwLogBody{
		Body:                   attrValue(log.Body()),
		SeverityNumber:         int32(log.SeverityNumber()),
		SeverityText:           log.SeverityText(),
		DroppedAttributesCount: log.DroppedAttributesCount(),
		Flags:                  log.Flags(),
	}
	if traceID := log.TraceID(); !traceID.IsEmpty() {
		body.TraceID = traceID.HexString()
	}
	if spanID := log.SpanID(); !spanID.IsEmpty() {
		body.SpanID = spanID.HexString()
	}
	body.Attributes = attrsValue(log.Attributes())
	body.Resource = resourceAttrs

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return &cloudwatchlogs.InputLogEvent{
		Timestamp: aws.Int64(int64(log.Timestamp()) / int64(time.Millisecond)), // in milliseconds
		Message:   aws.String(string(bodyJSON)),
	}, nil
}

func attrsValue(attrs pcommon.Map) map[string]interface{} {
	if attrs.Len() == 0 {
		return nil
	}
	out := make(map[string]interface{}, attrs.Len())
	attrs.Range(func(k string, v pcommon.Value) bool {
		out[k] = attrValue(v)
		return true
	})
	return out
}

func attrValue(value pcommon.Value) interface{} {
	switch value.Type() {
	case pcommon.ValueTypeInt:
		return value.IntVal()
	case pcommon.ValueTypeBool:
		return value.BoolVal()
	case pcommon.ValueTypeDouble:
		return value.DoubleVal()
	case pcommon.ValueTypeString:
		return value.StringVal()
	case pcommon.ValueTypeMap:
		values := map[string]interface{}{}
		value.MapVal().Range(func(k string, v pcommon.Value) bool {
			values[k] = attrValue(v)
			return true
		})
		return values
	case pcommon.ValueTypeSlice:
		arrayVal := value.SliceVal()
		values := make([]interface{}, arrayVal.Len())
		for i := 0; i < arrayVal.Len(); i++ {
			values[i] = attrValue(arrayVal.At(i))
		}
		return values
	case pcommon.ValueTypeEmpty:
		return nil
	default:
		return nil
	}
}
