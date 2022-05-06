// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logstransformprocessor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/stanza"
)

var (
	cfg = &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
		BaseConfig: stanza.BaseConfig{
			Operators: stanza.OperatorConfigs{
				map[string]interface{}{
					"type":  "regex_parser",
					"regex": "^(?P<time>\\d{4}-\\d{2}-\\d{2}) (?P<sev>[A-Z]*) (?P<msg>.*)$",
					"severity": map[string]interface{}{
						"parse_from": "attributes.sev",
					},
					"timestamp": map[string]interface{}{
						"layout":     "%Y-%m-%d",
						"parse_from": "attributes.time",
					},
				},
			},
			Converter: stanza.ConverterConfig{
				MaxFlushCount: 100,
				FlushInterval: 100 * time.Millisecond,
			},
		},
	}
)

func parseTime(format, input string) *time.Time {
	val, _ := time.ParseInLocation(format, input, time.Local)
	return &val
}

type testLogMessage struct {
	body         *pdata.Value
	time         *time.Time
	observedTime *time.Time
	severity     pdata.SeverityNumber
	severityText *string
	spanID       *pdata.SpanID
	traceID      *pdata.TraceID
	flags        uint32
	attributes   *map[string]pdata.Value
}

// Skips test without applying unused rule: https://github.com/dominikh/go-tools/issues/633#issuecomment-606560616
var skip = func(t *testing.T) {
	t.Skip("Flaky test - See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/9761")
}

func TestLogsTransformProcessor(t *testing.T) {
	skip(t)
	baseMessage := pcommon.NewValueString("2022-01-01 INFO this is a test")
	spanID := pcommon.NewSpanID([8]byte{0x32, 0xf0, 0xa2, 0x2b, 0x6a, 0x81, 0x2c, 0xff})
	traceID := pcommon.NewTraceID([16]byte{0x48, 0x01, 0x40, 0xf3, 0xd7, 0x70, 0xa5, 0xae, 0x32, 0xf0, 0xa2, 0x2b, 0x6a, 0x81, 0x2c, 0xff})
	infoSeverityText := "Info"

	tests := []struct {
		name           string
		config         *Config
		sourceMessages []testLogMessage
		parsedMessages []testLogMessage
	}{
		{
			name:   "simpleTest",
			config: cfg,
			sourceMessages: []testLogMessage{
				{
					body:         &baseMessage,
					spanID:       &spanID,
					traceID:      &traceID,
					flags:        uint32(0x01),
					observedTime: parseTime("2006-01-02", "2022-01-02"),
				},
			},
			parsedMessages: []testLogMessage{
				{
					body:         &baseMessage,
					severity:     plog.SeverityNumberINFO,
					severityText: &infoSeverityText,
					attributes: &map[string]pdata.Value{
						"msg":  pcommon.NewValueString("this is a test"),
						"time": pcommon.NewValueString("2022-01-01"),
						"sev":  pcommon.NewValueString("INFO"),
					},
					spanID:       &spanID,
					traceID:      &traceID,
					flags:        uint32(0x01),
					observedTime: parseTime("2006-01-02", "2022-01-02"),
					time:         parseTime("2006-01-02", "2022-01-01"),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tln := new(consumertest.LogsSink)
			factory := NewFactory()
			ltp, err := factory.CreateLogsProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), tt.config, tln)
			require.NoError(t, err)
			assert.True(t, ltp.Capabilities().MutatesData)

			err = ltp.Start(context.Background(), nil)
			require.NoError(t, err)

			sourceLogData := generateLogData(tt.sourceMessages)
			wantLogData := generateLogData(tt.parsedMessages)
			err = ltp.ConsumeLogs(context.Background(), sourceLogData)
			require.NoError(t, err)
			logs := tln.AllLogs()
			require.Len(t, logs, 1)

			logs[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Sort()
			assert.EqualValues(t, wantLogData, logs[0])
		})
	}
}

func generateLogData(messages []testLogMessage) pdata.Logs {
	ld := testdata.GenerateLogsOneEmptyResourceLogs()
	for _, content := range messages {
		log := ld.ResourceLogs().At(0).ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
		if content.body != nil {
			content.body.CopyTo(log.Body())
		}
		if content.time != nil {
			log.SetTimestamp(pdata.NewTimestampFromTime(*content.time))
		}
		if content.observedTime != nil {
			log.SetObservedTimestamp(pdata.NewTimestampFromTime(*content.observedTime))
		}
		if content.severity != 0 {
			log.SetSeverityNumber(content.severity)
		}
		if content.severityText != nil {
			log.SetSeverityText(*content.severityText)
		}
		if content.attributes != nil {
			for k, v := range *content.attributes {
				log.Attributes().Insert(k, v)
			}
			log.Attributes().Sort()
		}

		if content.spanID != nil {
			log.SetSpanID(*content.spanID)
		}

		if content.traceID != nil {
			log.SetTraceID(*content.traceID)
		}

		if content.flags != uint32(0x00) {
			log.SetFlags(content.flags)
		}
	}

	return ld
}
