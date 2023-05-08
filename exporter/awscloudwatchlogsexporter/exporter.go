// Copyright The OpenTelemetry Authors
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

package awscloudwatchlogsexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awscloudwatchlogsexporter"

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/google/uuid"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	exp "go.opentelemetry.io/collector/exporter"
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
	pusherMap        map[cwlogs.PusherKey]cwlogs.Pusher
	pusherMapLock    sync.RWMutex
}

type awsMetadata struct {
	LogGroupName  string `json:"logGroupName,omitempty"`
	LogStreamName string `json:"logStreamName,omitempty"`
}

type emfMetadata struct {
	AWSMetadata   *awsMetadata `json:"_aws,omitempty"`
	LogGroupName  string       `json:"log_group_name,omitempty"`
	LogStreamName string       `json:"log_stream_name,omitempty"`
}

func newCwLogsPusher(expConfig *Config, params exp.CreateSettings) (*exporter, error) {
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
	svcStructuredLog := cwlogs.NewClient(params.Logger, awsConfig, params.BuildInfo, expConfig.LogGroupName, expConfig.LogRetention, session)
	collectorIdentifier, err := uuid.NewRandom()

	if err != nil {
		return nil, err
	}

	pusherKey := cwlogs.PusherKey{
		LogGroupName:  expConfig.LogGroupName,
		LogStreamName: expConfig.LogStreamName,
	}

	pusher := cwlogs.NewPusher(pusherKey, *awsConfig.MaxRetries, *svcStructuredLog, params.Logger)

	pusherMap := make(map[cwlogs.PusherKey]cwlogs.Pusher)

	pusherMap[pusherKey] = pusher

	logsExporter := &exporter{
		svcStructuredLog: svcStructuredLog,
		Config:           expConfig,
		logger:           params.Logger,
		retryCount:       *awsConfig.MaxRetries,
		collectorID:      collectorIdentifier.String(),
		pusherMap:        pusherMap,
	}
	return logsExporter, nil
}

func newCwLogsExporter(config component.Config, params exp.CreateSettings) (exp.Logs, error) {
	expConfig := config.(*Config)
	logsPusher, err := newCwLogsPusher(expConfig, params)
	if err != nil {
		return nil, err
	}
	return exporterhelper.NewLogsExporter(
		context.TODO(),
		params,
		config,
		logsPusher.consumeLogs,
		exporterhelper.WithQueue(expConfig.enforcedQueueSettings()),
		exporterhelper.WithRetry(expConfig.RetrySettings),
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		exporterhelper.WithShutdown(logsPusher.shutdown),
	)
}

func (e *exporter) consumeLogs(_ context.Context, ld plog.Logs) error {
	logEvents, _ := logsToCWLogs(e.logger, ld, e.Config)
	if len(logEvents) == 0 {
		return nil
	}

	logPushersUsed := make(map[cwlogs.PusherKey]cwlogs.Pusher)
	for _, logEvent := range logEvents {
		pusherKey := cwlogs.PusherKey{
			LogGroupName:  logEvent.LogGroupName,
			LogStreamName: logEvent.LogStreamName,
		}
		cwLogsPusher := e.getLogPusher(logEvent)
		e.logger.Debug("Adding log event", zap.Any("event", logEvent))
		err := cwLogsPusher.AddLogEntry(logEvent)
		if err != nil {
			e.logger.Error("Failed ", zap.Int("num_of_events", len(logEvents)))
		}
		logPushersUsed[pusherKey] = cwLogsPusher
	}
	var flushErrArray []error
	for _, pusher := range logPushersUsed {
		flushErr := pusher.ForceFlush()
		if flushErr != nil {
			e.logger.Error("Error force flushing logs. Skipping to next logPusher.", zap.Error(flushErr))
			flushErrArray = append(flushErrArray, flushErr)
		}
	}
	if len(flushErrArray) != 0 {
		errorString := ""
		for _, err := range flushErrArray {
			errorString += err.Error()
		}
		return errors.New(errorString)
	}
	return nil
}

func (e *exporter) getLogPusher(logEvent *cwlogs.Event) cwlogs.Pusher {
	e.pusherMapLock.Lock()
	defer e.pusherMapLock.Unlock()
	pusherKey := cwlogs.PusherKey{
		LogGroupName:  logEvent.LogGroupName,
		LogStreamName: logEvent.LogStreamName,
	}
	if e.pusherMap[pusherKey] == nil {
		pusher := cwlogs.NewPusher(pusherKey, e.retryCount, *e.svcStructuredLog, e.logger)
		e.pusherMap[pusherKey] = pusher
	}
	return e.pusherMap[pusherKey]
}

func (e *exporter) shutdown(_ context.Context) error {
	if e.pusherMap != nil {
		for _, pusher := range e.pusherMap {
			pusher.ForceFlush()
		}
	}
	return nil
}

func logsToCWLogs(logger *zap.Logger, ld plog.Logs, config *Config) ([]*cwlogs.Event, int) {
	n := ld.ResourceLogs().Len()
	if n == 0 {
		return []*cwlogs.Event{}, 0
	}

	var dropped int
	var out []*cwlogs.Event

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
				event, err := logToCWLog(resourceAttrs, log, config)
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

func logToCWLog(resourceAttrs map[string]interface{}, log plog.LogRecord, config *Config) (*cwlogs.Event, error) {
	// TODO(jbd): Benchmark and improve the allocations.
	// Evaluate go.elastic.co/fastjson as a replacement for encoding/json.
	logGroupName := config.LogGroupName
	logStreamName := config.LogStreamName

	var bodyJSON []byte
	var err error
	if config.RawLog {
		// Check if this is an emf log
		var metadata emfMetadata
		bodyString := log.Body().AsString()
		err = json.Unmarshal([]byte(bodyString), &metadata)
		// v1 emf json
		if err == nil && metadata.AWSMetadata != nil && metadata.AWSMetadata.LogGroupName != "" {
			logGroupName = metadata.AWSMetadata.LogGroupName
			if metadata.AWSMetadata.LogStreamName != "" {
				logStreamName = metadata.AWSMetadata.LogStreamName
			}
		} else /* v0 emf json */ if err == nil && metadata.LogGroupName != "" {
			logGroupName = metadata.LogGroupName
			if metadata.LogStreamName != "" {
				logStreamName = metadata.LogStreamName
			}
		}
		bodyJSON = []byte(bodyString)
	} else {
		body := cwLogBody{
			Body:                   log.Body().AsRaw(),
			SeverityNumber:         int32(log.SeverityNumber()),
			SeverityText:           log.SeverityText(),
			DroppedAttributesCount: log.DroppedAttributesCount(),
			Flags:                  uint32(log.Flags()),
		}
		if traceID := log.TraceID(); !traceID.IsEmpty() {
			body.TraceID = hex.EncodeToString(traceID[:])
		}
		if spanID := log.SpanID(); !spanID.IsEmpty() {
			body.SpanID = hex.EncodeToString(spanID[:])
		}
		body.Attributes = attrsValue(log.Attributes())
		body.Resource = resourceAttrs

		bodyJSON, err = json.Marshal(body)
		if err != nil {
			return &cwlogs.Event{}, err
		}
	}

	return &cwlogs.Event{
		InputLogEvent: &cloudwatchlogs.InputLogEvent{
			Timestamp: aws.Int64(int64(log.Timestamp()) / int64(time.Millisecond)), // in milliseconds
			Message:   aws.String(string(bodyJSON)),
		},
		LogGroupName:  logGroupName,
		LogStreamName: logStreamName,
		GeneratedTime: time.Now(),
	}, nil
}

func attrsValue(attrs pcommon.Map) map[string]interface{} {
	if attrs.Len() == 0 {
		return nil
	}
	out := make(map[string]interface{}, attrs.Len())
	attrs.Range(func(k string, v pcommon.Value) bool {
		out[k] = v.AsRaw()
		return true
	})
	return out
}
