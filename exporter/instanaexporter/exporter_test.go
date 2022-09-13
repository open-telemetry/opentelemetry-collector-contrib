// Copyright 2022, OpenTelemetry Authors
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

package instanaexporter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestPushConvertedTraces(t *testing.T) {
	traceServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusAccepted)
	}))
	defer traceServer.Close()

	cfg := Config{
		AgentKey:           "key11",
		HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: traceServer.URL},
		Endpoint:           traceServer.URL,
		ExporterSettings:   config.NewExporterSettings(config.NewComponentIDWithName(typeStr, "valid")),
	}

	instanaExporter := newInstanaExporter(&cfg, componenttest.NewNopExporterCreateSettings())
	ctx := context.Background()
	err := instanaExporter.start(ctx, componenttest.NewNopHost())
	assert.NoError(t, err)

	err = instanaExporter.pushConvertedTraces(ctx, newTestTraces())
	assert.NoError(t, err)
}

func newTestTraces() ptrace.Traces {
	traces := ptrace.NewTraces()
	rspans := traces.ResourceSpans().AppendEmpty()
	rspans.Resource().Attributes().UpsertString("instana.agent", "agent1")
	span := rspans.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetTraceID([16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 2, 3, 4})
	span.SetSpanID([8]byte{0, 0, 0, 0, 1, 2, 3, 4})
	return traces
}
