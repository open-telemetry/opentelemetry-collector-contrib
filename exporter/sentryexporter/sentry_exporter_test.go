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
	"fmt"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector/consumer/pdata"
	"github.com/stretchr/testify/assert"
)

func TestGenerateSentryTraceID(t *testing.T) {
	testTraceID := pdata.NewTraceID([]byte{1, 2, 3, 4, 5, 6, 7, 8, 8, 7, 6, 5, 4, 3, 2, 1})
	traceID, err := generateSentryTraceID(testTraceID)
	assert.NoError(t, err)
	assert.NotNil(t, traceID)

	fmt.Println(len(testTraceID.Bytes()))

	assert.Len(t, traceID, 32)
	assert.Equal(t, traceID[:16], "1234567887654321")
}
