// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package alibabacloudlogserviceexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

func TestNewTraceExporter(t *testing.T) {

	got, err := newTraceExporter(zap.NewNop(), &Config{
		Endpoint: "cn-hangzhou.log.aliyuncs.com",
		Project:  "demo-project",
		Logstore: "demo-logstore",
	})
	assert.NoError(t, err)
	require.NotNil(t, got)

	traceData := pdata.NewTraces()
	spanData := pdata.NewResourceSpans()
	spanData.InitEmpty()
	spans := pdata.NewInstrumentationLibrarySpans()
	spans.InitEmpty()
	span := pdata.NewSpan()
	span.InitEmpty()
	spans.Spans().Append(span)
	spanData.InstrumentationLibrarySpans().Append(spans)
	traceData.ResourceSpans().Append(spanData)

	// This will put trace data to send buffer and return success.
	err = got.ConsumeTraces(context.Background(), traceData)
	assert.Error(t, err)
	assert.Nil(t, got.Shutdown(context.Background()))
}

func TestNewFailsWithEmptyTraceExporterName(t *testing.T) {

	got, err := newTraceExporter(zap.NewNop(), &Config{})
	assert.Error(t, err)
	require.Nil(t, got)
}
