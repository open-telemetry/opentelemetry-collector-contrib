// Copyright The OpenTelemetry Authors
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

package datadogexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

func TestNewTraceExporter(t *testing.T) {
	cfg := &Config{}
	cfg.API.Key = "ddog_32_characters_long_api_key1"
	logger := zap.NewNop()

	// The client should have been created correctly
	exp, err := newTraceExporter(logger, cfg)
	assert.NoError(t, err)
	assert.NotNil(t, exp)
}

func TestPushTraceData(t *testing.T) {
	cfg := &Config{
		TagsConfig: TagsConfig{
			Hostname: "test_host",
			Env:      "test_env",
			Tags:     []string{"key:val"},
		},
		Traces: TracesConfig{
			SampleRate: 1,
		},
	}
	logger := zap.NewNop()

	exp, err := newTraceExporter(logger, cfg)

	assert.NoError(t, err)

	tracesLength, err := exp.pushTraceData(context.Background(), func() pdata.Traces {
		traces := pdata.NewTraces()
		resourceSpans := traces.ResourceSpans()
		resourceSpans.Resize(1)
		resourceSpans.At(0).InitEmpty()
		resourceSpans.At(0).InstrumentationLibrarySpans().Resize(1)
		resourceSpans.At(0).InstrumentationLibrarySpans().At(0).Spans().Resize(1)
		return traces
	}())

	assert.Nil(t, err)
	assert.Equal(t, 1, tracesLength)

}
