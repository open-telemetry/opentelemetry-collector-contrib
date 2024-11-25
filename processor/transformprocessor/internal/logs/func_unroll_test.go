package logs

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
)

func TestUnroll(t *testing.T) {
	tests := []struct {
		name        string
		inputData   map[string]interface{}
		expected    []plog.LogRecord
		expectError bool
	}{
		// {
		// 	name: "simple_array",
		// 	inputData: map[string]interface{}{
		// 		"body": map[string]interface{}{
		// 			"items": []interface{}{"one", "two", "three"},
		// 		},
		// 	},
		// 	expected: []plog.LogRecord{
		// 		func() plog.LogRecord {
		// 			lr := plog.NewLogRecord()
		// 			lr.Body().SetStr("one")
		// 			return lr
		// 		}(),
		// 		func() plog.LogRecord {
		// 			lr := plog.NewLogRecord()
		// 			lr.Body().SetStr("two")
		// 			return lr
		// 		}(),
		// 		func() plog.LogRecord {
		// 			lr := plog.NewLogRecord()
		// 			lr.Body().SetStr("three")
		// 			return lr
		// 		}(),
		// 	},
		// 	expectError: false,
		// },
		{
			name: "numeric_array",
			inputData: map[string]interface{}{
				"body": map[string]interface{}{
					"items": []interface{}{1, 2, 3},
				},
			},
			expected: []plog.LogRecord{
				func() plog.LogRecord {
					lr := plog.NewLogRecord()
					lr.Body().SetInt(1)
					return lr
				}(),
				func() plog.LogRecord {
					lr := plog.NewLogRecord()
					lr.Body().SetInt(2)
					return lr
				}(),
				func() plog.LogRecord {
					lr := plog.NewLogRecord()
					lr.Body().SetInt(3)
					return lr
				}(),
			},
		},
		{
			name: "empty_array",
			inputData: map[string]interface{}{
				"body": map[string]interface{}{
					"items": []interface{}{},
				},
			},
			expected:    []plog.LogRecord{},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lr := plog.NewLogRecord()
			m := pcommon.NewMap()
			err := m.FromRaw(tt.inputData)
			require.NoError(t, err)
			lr.Attributes().FromRaw(tt.inputData)

			arrayMap := pcommon.NewMap()
			arraySlice := arrayMap.PutEmptySlice("items")
			items := tt.inputData["body"].(map[string]any)["items"].([]any)
			for _, item := range items {
				switch v := item.(type) {
				case string:
					arraySlice.AppendEmpty().SetStr(v)
				case map[string]any:
					newMap := arraySlice.AppendEmpty().SetEmptyMap()
					err := newMap.FromRaw(v)
					assert.NoError(t, err)
				default:
					arraySlice.AppendEmpty().SetStr(fmt.Sprintf("%v", v))
				}
			}

			field := ottl.StandardGetSetter[ottllog.TransformContext]{
				Getter: func(ctx context.Context, tCtx ottllog.TransformContext) (any, error) {
					return arraySlice, nil
				},
			}

			exprFunc, err := unroll(field)
			if tt.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			scopeLogs := plog.NewScopeLogs()

			_, err = exprFunc(context.Background(), ottllog.NewTransformContext(lr, plog.NewScopeLogs().Scope(), plog.NewResourceLogs().Resource(), scopeLogs, plog.NewResourceLogs()))
			if tt.expectError {
				require.Error(t, err)
				return
			}
			require.Equal(t, scopeLogs.LogRecords().Len(), len(tt.expected))

			for i, expected := range tt.expected {
				require.Equal(t, expected.Body().AsRaw(), scopeLogs.LogRecords().At(i).Body().AsRaw())
			}

			// Verify results
			// scopeLogs
			// require.True(t, ok)
			// require.Equal(t, len(tt.expected), len(results.Slice().AsRaw()))

			// for i, expected := range tt.expected {
			// 	require.Equal(t, expected.Body().AsRaw(), results.Slice().At(i).AsRaw())
			// }
		})
	}
}
