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

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/stanza"
)

var (
	cfg = &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
		BaseConfig: stanza.BaseConfig{
			ReceiverSettings: config.ReceiverSettings{},
			Operators: stanza.OperatorConfigs{
				map[string]interface{}{
					"type":  "regex_parser",
					"regex": "^(?P<time>\\d{4}-\\d{2}-\\d{2}) (?P<sev>[A-Z]*) (?P<msg>.*)$",
					"severity": map[string]interface{}{
						"parse_from": "body.sev",
					},
					"timestamp": map[string]interface{}{
						"layout":     "%Y-%m-%d",
						"parse_from": "body.time",
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
	val, _ := time.Parse("%Y-%m-%d", "2022-01-01")
	return &val
}

type testLogMessage struct {
	body               *pdata.Value
	time               *time.Time
	severity           pdata.SeverityNumber
	severityText       *string
	spanId             *pdata.SpanID
	attributes         *map[string]pdata.Value
	resourceAttributes *map[string]pdata.Value
}

func TestLogsTransformProcessor(t *testing.T) {
	baseMessage := pdata.NewValueString("2022-01-01 INFO this is a test")
	tests := []struct {
		name          string
		config        *Config
		sourceMessage testLogMessage
		parsedMessage testLogMessage
	}{
		{
			name:   "simpleTest",
			config: cfg,
			sourceMessage: testLogMessage{
				body: &baseMessage,
			},
			parsedMessage: testLogMessage{
				body:     &baseMessage,
				severity: pdata.SeverityNumberINFO,
				attributes: &map[string]pdata.Value{
					"message": pdata.NewValueString("this is a test"),
				},
				time: parseTime("%Y-%m-%d", "2022-01-01"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test logs consumer
			tln := new(consumertest.LogsSink)
			factory := NewFactory()
			ltp, err := factory.CreateLogsProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), tt.config, tln)
			require.NoError(t, err)
			assert.True(t, ltp.Capabilities().MutatesData)

			sourceLogData := generateLogData(tt.sourceMessage)
			wantLogData := generateLogData(tt.parsedMessage)
			err = ltp.ConsumeLogs(context.Background(), sourceLogData)
			require.NoError(t, err)
			logs := tln.AllLogs()
			require.Len(t, logs, 1)

			logs[0].ResourceLogs().At(0).Resource().Attributes().Sort()
			assert.EqualValues(t, wantLogData, logs[0])
		})
	}
}

func generateLogData(content testLogMessage) pdata.Logs {
	ld := testdata.GenerateLogsOneEmptyResourceLogs()
	log := ld.ResourceLogs().At(0).ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	if content.body != nil {
		content.body.CopyTo(log.Body())
	}
	if content.time != nil {
		log.SetTimestamp(pdata.NewTimestampFromTime(*content.time))
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

	resource := ld.ResourceLogs().At(0).Resource()
	if content.resourceAttributes != nil {
		for k, v := range *content.resourceAttributes {
			resource.Attributes().Insert(k, v)
		}
		resource.Attributes().Sort()
	}
	return ld
}
