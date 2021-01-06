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

package lokiexporter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"
	"go.uber.org/zap"
)

const (
	validEndpoint = "https://loki:3100/loki/api/v1/push"
)

var (
	validLabels = []string{"app", "level"}
)

func createLogData(numberOfLogs int) pdata.Logs {
	logs := pdata.NewLogs()
	logs.ResourceLogs().Resize(1)
	rl := logs.ResourceLogs().At(0)
	rl.InstrumentationLibraryLogs().Resize(1)
	ill := rl.InstrumentationLibraryLogs().At(0)

	for i := 0; i < numberOfLogs; i++ {
		ts := pdata.TimestampUnixNano(int64(i) * time.Millisecond.Nanoseconds())
		logRecord := pdata.NewLogRecord()
		logRecord.Body().SetStringVal("mylog")
		logRecord.Attributes().InsertString(conventions.AttributeServiceName, "myapi")
		logRecord.Attributes().InsertString(conventions.AttributeContainerName, "api")
		logRecord.Attributes().InsertString(conventions.AttributeContainerImage, "nginx")
		logRecord.Attributes().InsertString(conventions.AttributeHostName, "myhost")
		logRecord.Attributes().InsertString("custom", "custom")
		logRecord.SetTimestamp(ts)

		ill.Logs().Append(logRecord)
	}

	return logs
}

func TestEmptyEndpointReturnsError(t *testing.T) {
	config := &Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: "",
		},
	}
	f := NewFactory()
	params := component.ExporterCreateParams{Logger: zap.NewNop()}
	_, err := f.CreateLogsExporter(context.Background(), params, config)
	require.Error(t, err)
}

func TestNoAllowedLabelsReturnsError(t *testing.T) {
	config := &Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: validEndpoint,
		},
		AttributesForLabels: nil,
	}
	f := NewFactory()
	params := component.ExporterCreateParams{Logger: zap.NewNop()}
	_, err := f.CreateLogsExporter(context.Background(), params, config)
	require.Error(t, err)
}

func TestPushLogDataReturnsWithoutErrorWhileNotImplemented(t *testing.T) {
	config := &Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: validEndpoint,
		},
		AttributesForLabels: validLabels,
	}
	e, err := newExporter(config)
	assert.NoError(t, err)

	_, err = e.pushLogData(context.Background(), createLogData(1))
	assert.NoError(t, err)
}

func TestExporterStartAlwaysReturnsNil(t *testing.T) {
	config := &Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: validEndpoint,
		},
		AttributesForLabels: validLabels,
	}
	e, err := newExporter(config)
	assert.NoError(t, err)
	assert.NoError(t, e.start(context.Background(), componenttest.NewNopHost()))
}

func TestExporterStopAlwaysReturnsNil(t *testing.T) {
	config := &Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: validEndpoint,
		},
		AttributesForLabels: validLabels,
	}
	e, err := newExporter(config)
	assert.NoError(t, err)
	assert.NoError(t, e.stop(context.Background()))
}
