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

package skywalkingreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSwProtoToTraces(t *testing.T) {
	swSpan := mockGrpcTraceSegment(1)
	td := SkywalkingToTraces(swSpan)

	assert.Equal(t, 1, td.ResourceSpans().Len())
	assert.Equal(t, 2, td.ResourceSpans().At(0).InstrumentationLibrarySpans().At(0).Spans().Len())

}
