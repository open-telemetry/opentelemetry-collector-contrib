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

package awscloudwatchlogsexporter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

type exporter struct {
	config *Config
	logger *zap.Logger

	startOnce sync.Once
	client    *cloudwatchlogs.CloudWatchLogs // available after startOnce

	seqTokenMu sync.Mutex
	seqToken   string
}

func (e *exporter) Start(ctx context.Context, host component.Host) error {
	var startErr error
	e.startOnce.Do(func() {
		awsConfig := &aws.Config{}
		if e.config.Region != "" {
			awsConfig.Region = aws.String(e.config.Region)
		}
		if e.config.Endpoint != "" {
			awsConfig.Endpoint = aws.String(e.config.Endpoint)
		}
		if e.config.MaxRetries > 0 {
			awsConfig.MaxRetries = aws.Int(e.config.MaxRetries)
		}
		sess, err := session.NewSession(awsConfig)
		if err != nil {
			startErr = err
			return
		}
		e.client = cloudwatchlogs.New(sess)

		e.logger.Debug("Retrieving Cloud Watch sequence token")
		out, err := e.client.DescribeLogStreams(&cloudwatchlogs.DescribeLogStreamsInput{
			LogGroupName:        aws.String(e.config.LogGroupName),
			LogStreamNamePrefix: aws.String(e.config.LogStreamName),
		})
		if err != nil {
			startErr = err
			return
		}
		if len(out.LogStreams) == 0 {
			startErr = errors.New("cannot find log group and stream")
			return
		}
		stream := out.LogStreams[0]

		e.seqTokenMu.Lock()
		e.seqToken = *stream.UploadSequenceToken
		e.seqTokenMu.Unlock()
	})
	return startErr
}

func (e *exporter) Shutdown(ctx context.Context) error {
	// TODO(jbd): Signal shutdown to flush the logs.
	return nil
}

func (e *exporter) ConsumeLogs(ctx context.Context, ld pdata.Logs) error {
	logEvents, dropped, err := logsToCWLogs(e.logger, ld)
	if err != nil {
		return err
	}
	if dropped > 0 {
		e.logger.Warn("CloudWatch Logs exporter dropped log records", zap.Any("count", dropped))
	}
	if len(logEvents) == 0 {
		return nil
	}

	var seqToken string
	e.seqTokenMu.Lock()
	seqToken = e.seqToken
	e.seqTokenMu.Unlock()

	e.logger.Debug("Putting log events", zap.Int("num_of_events", len(logEvents)))
	input := &cloudwatchlogs.PutLogEventsInput{
		LogGroupName:  aws.String(e.config.LogGroupName),
		LogStreamName: aws.String(e.config.LogStreamName),
		LogEvents:     logEvents,
		SequenceToken: aws.String(seqToken),
	}
	out, err := e.client.PutLogEvents(input)
	if err != nil {
		return err
	}
	if info := out.RejectedLogEventsInfo; info != nil {
		return fmt.Errorf("log event rejected")
	}
	e.logger.Debug("Log events are successfully put")

	// TODO(jbd): Investigate how the concurrency model of exporters
	// impact the use of sequence tokens.
	e.seqTokenMu.Lock()
	e.seqToken = *out.NextSequenceToken
	e.seqTokenMu.Unlock()
	return nil
}

func logsToCWLogs(logger *zap.Logger, ld pdata.Logs) ([]*cloudwatchlogs.InputLogEvent, int, error) {
	n := ld.ResourceLogs().Len()
	if n == 0 {
		return []*cloudwatchlogs.InputLogEvent{}, 0, nil
	}

	var dropped int
	out := make([]*cloudwatchlogs.InputLogEvent, 0) // TODO(jbd): set a better capacity

	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		rl := rls.At(i)
		ills := rl.InstrumentationLibraryLogs()
		for j := 0; j < ills.Len(); j++ {
			ils := ills.At(j)
			logs := ils.Logs()
			for k := 0; k < logs.Len(); k++ {
				log := logs.At(k)
				event, err := logToCWLog(rl.Resource(), log)
				if err != nil {
					logger.Debug("Failed to convert to CloudWatch Log", zap.Error(err))
					dropped++
				} else {
					out = append(out, event)
				}
			}
		}
	}
	return out, dropped, nil
}

type cwLogBody struct {
	Name                   string                 `json:"name,omitempty"`
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

func logToCWLog(resource pdata.Resource, log pdata.LogRecord) (*cloudwatchlogs.InputLogEvent, error) {
	// TODO(jbd): Benchmark and improve the allocations.
	// Evaluate go.elastic.co/fastjson as a replacement for encoding/json.
	body := &cwLogBody{
		Name:                   log.Name(),
		Body:                   attrValue(log.Body()),
		SeverityNumber:         int32(log.SeverityNumber()),
		SeverityText:           log.SeverityText(),
		DroppedAttributesCount: uint32(log.DroppedAttributesCount()),
		Flags:                  log.Flags(),
	}
	if traceID := log.TraceID(); traceID.IsValid() {
		body.TraceID = traceID.HexString()
	}
	if spanID := log.SpanID(); spanID.IsValid() {
		body.TraceID = spanID.HexString()
	}

	if log.Attributes().Len() > 0 {
		attrs := make(map[string]interface{}, log.Attributes().Len())
		log.Attributes().ForEach(func(k string, v pdata.AttributeValue) {
			attrs[k] = attrValue(v)
		})
		body.Attributes = attrs
	}

	// Add resource attributes.
	if resource.Attributes().Len() > 0 {
		attrs := make(map[string]interface{}, log.Attributes().Len())
		resource.Attributes().ForEach(func(k string, v pdata.AttributeValue) {
			attrs[k] = attrValue(v)
		})
		body.Resource = attrs
	}

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return &cloudwatchlogs.InputLogEvent{
		Timestamp: aws.Int64(int64(log.Timestamp()) / (1000 * 1000)), // milliseconds
		Message:   aws.String(string(bodyJSON)),
	}, nil
}

func attrValue(value pdata.AttributeValue) interface{} {
	switch value.Type() {
	case pdata.AttributeValueINT:
		return value.IntVal()
	case pdata.AttributeValueBOOL:
		return value.BoolVal()
	case pdata.AttributeValueDOUBLE:
		return value.DoubleVal()
	case pdata.AttributeValueSTRING:
		return value.StringVal()
	case pdata.AttributeValueMAP:
		values := map[string]interface{}{}
		value.MapVal().ForEach(func(k string, v pdata.AttributeValue) {
			values[k] = attrValue(v)
		})
		return values
	case pdata.AttributeValueARRAY:
		arrayVal := value.ArrayVal()
		values := make([]interface{}, arrayVal.Len())
		for i := 0; i < arrayVal.Len(); i++ {
			values[i] = attrValue(arrayVal.At(i))
		}
		return values
	case pdata.AttributeValueNULL:
		return nil
	default:
		return nil
	}
}
