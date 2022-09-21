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

package otto

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestMarshalTracesEnvelope(t *testing.T) {
	e := wsMessageEnvelope{
		Payload: mkTraces(),
		Error:   nil,
	}
	jsonBytes, err := json.Marshal(e)
	require.NoError(t, err)
	strings.HasPrefix(string(jsonBytes), `{"Payload":{"`)
}

func TestMarshalMetricsEnvelope(t *testing.T) {
	e := wsMessageEnvelope{
		Payload: mkMetrics(),
		Error:   nil,
	}
	jsonBytes, err := json.Marshal(e)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(string(jsonBytes), `{"Payload":{"`))
	println(string(jsonBytes))
}

func mkTraces() traces {
	tr := ptrace.NewTraces()
	tr.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetName("foo")
	return traces(tr)
}

func mkMetrics() metrics {
	m := pmetric.NewMetrics()
	m.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName("foo")
	return metrics(m)
}
