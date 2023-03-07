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

package traceutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestSpanKindStr(t *testing.T) {
	assert.EqualValues(t, "SPAN_KIND_UNSPECIFIED", SpanKindStr(ptrace.SpanKindUnspecified))
	assert.EqualValues(t, "SPAN_KIND_INTERNAL", SpanKindStr(ptrace.SpanKindInternal))
	assert.EqualValues(t, "SPAN_KIND_SERVER", SpanKindStr(ptrace.SpanKindServer))
	assert.EqualValues(t, "SPAN_KIND_CLIENT", SpanKindStr(ptrace.SpanKindClient))
	assert.EqualValues(t, "SPAN_KIND_PRODUCER", SpanKindStr(ptrace.SpanKindProducer))
	assert.EqualValues(t, "SPAN_KIND_CONSUMER", SpanKindStr(ptrace.SpanKindConsumer))
	assert.EqualValues(t, "", SpanKindStr(ptrace.SpanKind(100)))
}

func TestStatusCodeStr(t *testing.T) {
	assert.EqualValues(t, "STATUS_CODE_UNSET", StatusCodeStr(ptrace.StatusCodeUnset))
	assert.EqualValues(t, "STATUS_CODE_OK", StatusCodeStr(ptrace.StatusCodeOk))
	assert.EqualValues(t, "STATUS_CODE_ERROR", StatusCodeStr(ptrace.StatusCodeError))
	assert.EqualValues(t, "", StatusCodeStr(ptrace.StatusCode(100)))
}
