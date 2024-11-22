package logs

// import (
// 	"context"
// 	"fmt"
// 	"testing"

// 	"github.com/stretchr/testify/assert"
// 	"go.opentelemetry.io/collector/pdata/pcommon"
// 	"go.opentelemetry.io/collector/pdata/plog"

// 	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
// )

// func Test_unroll(t *testing.T) {
// 	tests := []struct {
// 		name        string
// 		inputData   map[string]interface{}
// 		expected    []plog.LogRecord
// 		expectError bool
// 	}{
// 		{
// 			name: "simple_array",
// 			inputData: map[string]interface{}{
// 				"body": map[string]interface{}{
// 					"items": []interface{}{"one", "two", "three"},
// 				},
// 			},
// 			expected: []plog.LogRecord{
// 				func() plog.LogRecord {
// 					lr := plog.NewLogRecord()
// 					bodyMap := lr.Body().SetEmptyMap()
// 					bodyMap.PutStr("item", "one")
// 					return lr
// 				}(),
// 				func() plog.LogRecord {
// 					lr := plog.NewLogRecord()
// 					bodyMap := lr.Body().SetEmptyMap()
// 					bodyMap.PutStr("item", "two")
// 					return lr
// 				}(),
// 				func() plog.LogRecord {
// 					lr := plog.NewLogRecord()
// 					bodyMap := lr.Body().SetEmptyMap()
// 					bodyMap.PutStr("item", "three")
// 					return lr
// 				}(),
// 			},
// 			expectError: false,
// 		},
// 		{
// 			name: "empty_array",
// 			inputData: map[string]interface{}{
// 				"body": map[string]interface{}{
// 					"items": []interface{}{},
// 				},
// 			},
// 			expected:    []plog.LogRecord{},
// 			expectError: false,
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			lr := plog.NewLogRecord()
// 			m := pcommon.NewMap()
// 			err := m.FromRaw(tt.inputData)
// 			assert.NoError(t, err)
// 			lr.Attributes().FromRaw(tt.inputData)

// 			arrayMap := pcommon.NewMap()
// 			arraySlice := arrayMap.PutEmptySlice("items")
// 			items := tt.inputData["body"].(map[string]interface{})["items"].([]interface{})
// 			for _, item := range items {
// 				switch v := item.(type) {
// 				case string:
// 					arraySlice.AppendEmpty().SetStr(v)
// 				case map[string]interface{}:
// 					newMap := arraySlice.AppendEmpty().SetEmptyMap()
// 					err := newMap.FromRaw(v)
// 					assert.NoError(t, err)
// 				default:
// 					arraySlice.AppendEmpty().SetStr(fmt.Sprintf("%v", v))
// 				}
// 			}

// 			from := ottl.StandardGetSetter[plog.LogRecord]{
// 				Getter: func(ctx context.Context, tCtx plog.LogRecord) (interface{}, error) {
// 					return arrayMap, nil
// 				},
// 			}

// 			to := ottl.StandardGetSetter[plog.LogRecord]{
// 				Setter: func(ctx context.Context, tCtx plog.LogRecord, val interface{}) error {
// 					bodyMap := tCtx.Body().SetEmptyMap()
// 					switch v := val.(type) {
// 					case string:
// 						bodyMap.PutStr("item", v)
// 					case map[string]interface{}:
// 						itemsMap := bodyMap.PutEmptyMap("item")
// 						return itemsMap.FromRaw(v)
// 					default:
// 						bodyMap.PutStr("item", fmt.Sprintf("%v", v))
// 					}
// 					return nil
// 				},
// 			}

// 			exprFunc, err := unroll[plog.LogRecord](from, to)
// 			if tt.expectError {
// 				assert.Error(t, err)
// 				return
// 			}
// 			assert.NoError(t, err)

// 			result, err := exprFunc(context.Background(), lr)
// 			if tt.expectError {
// 				assert.Error(t, err)
// 				return
// 			}
// 			assert.NoError(t, err)

// 			// Verify results
// 			results, ok := result.([]plog.LogRecord)
// 			assert.True(t, ok)
// 			assert.Equal(t, len(tt.expected), len(results))

// 			for i, expected := range tt.expected {
// 				assert.Equal(t, expected.Body().AsRaw(), results[i].Body().AsRaw())
// 			}
// 		})
// 	}
// }
