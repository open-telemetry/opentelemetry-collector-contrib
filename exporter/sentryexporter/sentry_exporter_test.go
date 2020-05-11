// Copyright 2020 OpenTelemetry Authors
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

package sentryexporter

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector/consumer/pdata"
	otlptrace "github.com/open-telemetry/opentelemetry-proto/gen/go/trace/v1"
	"github.com/stretchr/testify/assert"
)

// func TestGenerateSentryTraceID(t *testing.T) {
// 	td := testdata.GenerateTraceDataOneSpan()
// 	ilss := td.ResourceSpans().At(0).InstrumentationLibrarySpans().At(0)
// 	assert.EqualValues(t, 2, ilss.Spans().Len())
// }

func TestGenerateTagsFromAttributes(t *testing.T) {
	attrs := pdata.NewAttributeMap()

	attrs.InsertString("string-key", "string-value")
	attrs.InsertBool("bool-key", true)
	attrs.InsertDouble("double-key", 123.123)
	attrs.InsertInt("int-key", 321)

	tags := generateTagsFromAttributes(attrs)

	stringVal, _ := tags["string-key"]
	assert.Equal(t, stringVal, "string-value")
	boolVal, _ := tags["bool-key"]
	assert.Equal(t, boolVal, "true")
	doubleVal, _ := tags["double-key"]
	assert.Equal(t, doubleVal, "123.123")
	intVal, _ := tags["int-key"]
	assert.Equal(t, intVal, "321")
}

func TestGenerateStatusFromSpanStatus(t *testing.T) {
	spanStatus := pdata.NewSpanStatus()

	status1, message1 := generateStatusFromSpanStatus(spanStatus)

	assert.Equal(t, "unknown", status1)
	assert.Equal(t, "", message1)

	spanStatus.InitEmpty()
	spanStatus.SetMessage("message")
	spanStatus.SetCode(pdata.StatusCode(otlptrace.Status_ResourceExhausted))

	status2, message2 := generateStatusFromSpanStatus(spanStatus)

	assert.Equal(t, "resource_exhausted", status2)
	assert.Equal(t, "message", message2)
}
