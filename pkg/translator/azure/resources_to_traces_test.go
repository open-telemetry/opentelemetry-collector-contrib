// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azure // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/azure"

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestUnmarshalTracesSkipsRecordsWithInvalidIDs(t *testing.T) {
	const validRecord = `{
		"time": "2024-01-01T00:00:00Z",
		"resourceId": "/RESOURCE_ID",
		"OperationId": "0123456789abcdef0123456789abcdef",
		"Id": "0123456789abcdef",
		"ParentId": "",
		"Name": "valid-span",
		"Url": "https://example.com/path"
	}`

	tests := []struct {
		name          string
		invalidRecord string
	}{
		{
			name: "invalid trace id",
			invalidRecord: `{
				"time": "2024-01-01T00:00:00Z",
				"OperationId": "not-hex",
				"Id": "0123456789abcdef",
				"Name": "bad-span"
			}`,
		},
		{
			name: "invalid span id",
			invalidRecord: `{
				"time": "2024-01-01T00:00:00Z",
				"OperationId": "0123456789abcdef0123456789abcdef",
				"Id": "not-hex",
				"Name": "bad-span"
			}`,
		},
		{
			name: "invalid parent id",
			invalidRecord: `{
				"time": "2024-01-01T00:00:00Z",
				"OperationId": "0123456789abcdef0123456789abcdef",
				"Id": "0123456789abcdef",
				"ParentId": "not-hex",
				"Name": "bad-span"
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The invalid record comes first, so the valid record after it must still be converted.
			buf := fmt.Sprintf(`{"records": [%s, %s]}`, tt.invalidRecord, validRecord)

			unmarshaler := TracesUnmarshaler{Version: "1.2.3", Logger: zap.NewNop()}
			traces, err := unmarshaler.UnmarshalTraces([]byte(buf))
			require.NoError(t, err)

			require.Equal(t, 1, traces.SpanCount())
			span := traces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
			assert.Equal(t, "valid-span", span.Name())

			resourceID, ok := traces.ResourceSpans().At(0).Resource().Attributes().Get(traceAzureResourceID)
			require.True(t, ok)
			assert.Equal(t, "/RESOURCE_ID", resourceID.Str())
		})
	}
}
