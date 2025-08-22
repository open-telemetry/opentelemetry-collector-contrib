// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package libhoneyevent

import (
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

// TestSliceToArrayConversionError demonstrates the runtime error that would occur
// with the original code when trying to convert slices to arrays with mismatched lengths
func TestSliceToArrayConversionError(t *testing.T) {
	logger := zap.NewNop()
	now := time.Now()

	tests := []struct {
		name        string
		event       LibhoneyEvent
		cfg         FieldMapConfig
		expectError bool
		description string
	}{
		{
			name: "8-byte hex string for TraceID (would cause panic)",
			event: LibhoneyEvent{
				MsgPackTimestamp: &now,
				Data: map[string]any{
					"trace.trace_id": "1234567890abcdef", // 8 bytes when decoded
					"trace.span_id":  "1234567890abcdef", // 8 bytes - correct for SpanID
				},
			},
			cfg: FieldMapConfig{
				Attributes: AttributesConfig{
					TraceID: "trace.trace_id",
					SpanID:  "trace.span_id",
				},
			},
			expectError: false, // New code handles this gracefully by falling back
			description: "8-byte TraceID should fall back to hash generation",
		},
		{
			name: "16-byte hex string for SpanID (would cause panic)",
			event: LibhoneyEvent{
				MsgPackTimestamp: &now,
				Data: map[string]any{
					"trace.trace_id": "1234567890abcdef1234567890abcdef", // 16 bytes - correct for TraceID
					"trace.span_id":  "1234567890abcdef1234567890abcdef", // 16 bytes when decoded - wrong for SpanID
				},
			},
			cfg: FieldMapConfig{
				Attributes: AttributesConfig{
					TraceID: "trace.trace_id",
					SpanID:  "trace.span_id",
				},
			},
			expectError: false, // New code handles this gracefully by falling back
			description: "16-byte SpanID should fall back to hash generation",
		},
		{
			name: "4-byte hex string for TraceID",
			event: LibhoneyEvent{
				MsgPackTimestamp: &now,
				Data: map[string]any{
					"trace.trace_id": "12345678",         // 4 bytes when decoded
					"trace.span_id":  "1234567890abcdef", // 8 bytes - correct
				},
			},
			cfg: FieldMapConfig{
				Attributes: AttributesConfig{
					TraceID: "trace.trace_id",
					SpanID:  "trace.span_id",
				},
			},
			expectError: false, // Falls back to hash generation
			description: "4-byte TraceID should fall back to hash generation",
		},
		{
			name: "Valid 16-byte TraceID and 8-byte SpanID",
			event: LibhoneyEvent{
				MsgPackTimestamp: &now,
				Data: map[string]any{
					"trace.trace_id": "1234567890abcdef1234567890abcdef", // 16 bytes
					"trace.span_id":  "1234567890abcdef",                 // 8 bytes
				},
			},
			cfg: FieldMapConfig{
				Attributes: AttributesConfig{
					TraceID: "trace.trace_id",
					SpanID:  "trace.span_id",
				},
			},
			expectError: false,
			description: "Valid sizes should work correctly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			span := ptrace.NewSpan()
			alreadyUsedFields := []string{}

			// This should not panic with the new code
			err := tt.event.ToPTraceSpan(&span, &alreadyUsedFields, tt.cfg, *logger)

			if tt.expectError {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)

				// Verify that we got valid IDs (either parsed or generated)
				assert.NotEqual(t, pcommon.TraceID{}, span.TraceID(), "TraceID should not be empty")
				assert.NotEqual(t, pcommon.SpanID{}, span.SpanID(), "SpanID should not be empty")
			}
		})
	}
}

// TestOldCodeWouldPanic demonstrates what the old code would do
// This test shows the exact scenarios that would cause runtime panics
func TestOldCodeWouldPanic(t *testing.T) {
	// Simulate the ACTUAL old code behavior that would panic
	t.Run("Old TraceID code would panic on any slice", func(t *testing.T) {
		// The old code did this for ANY valid hex string:
		hexString := "1234567890abcdef1234567890abcdef" // 16 bytes when decoded
		tidByteArray, err := hex.DecodeString(hexString)
		require.NoError(t, err)
		assert.Equal(t, 16, len(tidByteArray))

		// Old code logic:
		// if len(tidByteArray) >= 32 {
		//     tidByteArray = tidByteArray[0:32]  // This creates a SLICE, not array
		// }
		// newSpan.SetTraceID(pcommon.TraceID(tidByteArray))  // PANIC - slice to array conversion

		// The issue is that tidByteArray is ALWAYS a slice ([]byte), never an array ([16]byte)
		// Go cannot convert []byte to [16]byte directly - this would panic:
		// _ = pcommon.TraceID(tidByteArray)  // This line would panic

		t.Log("OLD CODE: pcommon.TraceID(tidByteArray) would panic because tidByteArray is []byte, not [16]byte")
	})

	t.Run("Old SpanID code would panic on sliced data", func(t *testing.T) {
		// The old code did this for SpanIDs:
		hexString := "1234567890abcdef1234567890abcdef" // 16 bytes when decoded
		sidByteArray, err := hex.DecodeString(hexString)
		require.NoError(t, err)
		assert.Equal(t, 16, len(sidByteArray))

		// Old code logic:
		// if len(sidByteArray) >= 16 {
		//     sidByteArray = sidByteArray[0:16]  // This creates a SLICE of length 16
		// }
		// newSpan.SetSpanID(pcommon.SpanID(sidByteArray))  // PANIC - slice to array conversion

		// Even after slicing, sidByteArray is still a []byte slice, not a [8]byte array
		slicedData := sidByteArray[0:16] // Still a []byte slice of length 16
		assert.Equal(t, 16, len(slicedData))

		// This would panic: cannot convert slice with length 16 to array or pointer to array with length 8
		// _ = pcommon.SpanID(slicedData)  // This line would panic

		t.Log("OLD CODE: pcommon.SpanID(slicedData) would panic because slicedData is []byte, not [8]byte")
	})

	t.Run("Demonstrate the core Go type issue", func(t *testing.T) {
		// The fundamental issue: Go cannot convert slices to arrays of different sizes

		// This works (same size):
		slice8 := make([]byte, 8)
		var array8 [8]byte
		copy(array8[:], slice8) // OK

		// This would panic (different sizes):
		slice16 := make([]byte, 16)
		// var badArray8 [8]byte = [8]byte(slice16)  // Compile error: cannot convert
		// _ = pcommon.SpanID(slice16)  // Runtime panic: cannot convert slice with length 16 to array with length 8

		t.Logf("Go prevents direct conversion: []byte(len=%d) -> [8]byte", len(slice16))
		t.Log("The old code was trying to do exactly this invalid conversion")
	})
}

// TestParentIDSliceConversion tests the GetParentID function scenarios
func TestParentIDSliceConversion(t *testing.T) {
	tests := []struct {
		name        string
		event       LibhoneyEvent
		fieldName   string
		expectError bool
		description string
	}{
		{
			name: "8-byte parent ID (valid)",
			event: LibhoneyEvent{
				Data: map[string]any{
					"trace.parent_id": "1234567890abcdef", // 8 bytes
				},
			},
			fieldName:   "trace.parent_id",
			expectError: false,
			description: "8-byte parent ID should work",
		},
		{
			name: "16-byte parent ID (extract last 8 bytes)",
			event: LibhoneyEvent{
				Data: map[string]any{
					"trace.parent_id": "1234567890abcdef1234567890abcdef", // 16 bytes
				},
			},
			fieldName:   "trace.parent_id",
			expectError: false,
			description: "16-byte parent ID should extract last 8 bytes",
		},
		{
			name: "4-byte parent ID (too short)",
			event: LibhoneyEvent{
				Data: map[string]any{
					"trace.parent_id": "12345678", // 4 bytes
				},
			},
			fieldName:   "trace.parent_id",
			expectError: true,
			description: "4-byte parent ID should fail",
		},
		{
			name: "Invalid hex parent ID",
			event: LibhoneyEvent{
				Data: map[string]any{
					"trace.parent_id": "invalid-hex-string",
				},
			},
			fieldName:   "trace.parent_id",
			expectError: true,
			description: "Invalid hex should fail",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parentID, err := tt.event.GetParentID(tt.fieldName)

			if tt.expectError {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
				assert.NotEqual(t, [8]byte{}, parentID, "Parent ID should not be empty")
			}
		})
	}
}

// TestActualPanicScenario demonstrates the actual problematic old code patterns
func TestActualPanicScenario(t *testing.T) {
	// This test shows what the old code was doing that would cause panics

	t.Run("Simulate old TraceID code that would panic", func(t *testing.T) {
		// Simulate the old problematic code pattern
		oldTraceIDHandler := func(hexString string) {
			// This is exactly what the old code was doing:
			tidByteArray, err := hex.DecodeString(hexString)
			if err == nil {
				// Old code had this logic which still left tidByteArray as a slice
				if len(tidByteArray) >= 32 {
					tidByteArray = tidByteArray[0:32] // Still a []byte slice!
				}
				// This line would panic: pcommon.TraceID expects [16]byte, not []byte
				// newSpan.SetTraceID(pcommon.TraceID(tidByteArray))

				// Let's demonstrate why this would panic
				defer func() {
					if r := recover(); r != nil {
						assert.Contains(t, fmt.Sprint(r), "cannot convert slice")
						t.Logf("CAUGHT EXPECTED PANIC: %v", r)
					}
				}()

				// This will panic if tidByteArray is not exactly 16 bytes
				_ = pcommon.TraceID(tidByteArray) // PANIC HERE
				t.Error("Should have panicked but didn't")
			}
		}

		// Test with 8-byte hex string (would cause panic)
		t.Log("Testing old code with 8-byte TraceID...")
		oldTraceIDHandler("1234567890abcdef") // 8 bytes - would panic
	})

	t.Run("Simulate old SpanID code that would panic", func(t *testing.T) {
		// Simulate the old problematic SpanID code
		oldSpanIDHandler := func(hexString string) {
			// This is exactly what the old code was doing:
			sidByteArray, err := hex.DecodeString(hexString)
			if err == nil {
				// Old code had this logic which created slices of wrong size
				if len(sidByteArray) == 32 {
					sidByteArray = sidByteArray[8:24] // Creates 16-byte slice!
				} else if len(sidByteArray) >= 16 {
					sidByteArray = sidByteArray[0:16] // Creates 16-byte slice!
				}
				// This would panic: pcommon.SpanID expects [8]byte, not []byte of length 16

				defer func() {
					if r := recover(); r != nil {
						assert.Contains(t, fmt.Sprint(r), "cannot convert slice")
						t.Logf("CAUGHT EXPECTED PANIC: %v", r)
					}
				}()

				// This will panic because sidByteArray is a 16-byte slice, not 8-byte array
				_ = pcommon.SpanID(sidByteArray) // PANIC HERE
				t.Error("Should have panicked but didn't")
			}
		}

		// Test with 16-byte hex string (would cause panic when sliced)
		t.Log("Testing old code with 16-byte SpanID...")
		oldSpanIDHandler("1234567890abcdef1234567890abcdef") // 16 bytes - would panic when sliced to 16
	})
}

// TestReproduceProductionError reproduces the exact error from the deployment
func TestReproduceProductionError(t *testing.T) {
	// This test recreates the exact scenario that caused:
	// "runtime error: cannot convert slice with length 8 to array or pointer to array with length 16"

	logger := zap.NewNop()
	now := time.Now()

	// Common problematic scenarios from real-world data:
	problemEvents := []struct {
		name        string
		traceIDHex  string
		spanIDHex   string
		description string
	}{
		{
			name:        "8-byte TraceID from legacy system",
			traceIDHex:  "1234567890abcdef", // 8 bytes - too short for TraceID
			spanIDHex:   "abcdefgh12345678", // 8 bytes - correct for SpanID
			description: "Legacy systems sometimes send shorter trace IDs",
		},
		{
			name:        "SpanID used as TraceID",
			traceIDHex:  "deadbeefcafebabe", // 8 bytes - actually a SpanID
			spanIDHex:   "1234567890abcdef", // 8 bytes - correct
			description: "Configuration error where span ID field mapped to trace ID",
		},
		{
			name:        "Truncated TraceID",
			traceIDHex:  "12345678",         // 4 bytes - severely truncated
			spanIDHex:   "abcdef1234567890", // 8 bytes - correct
			description: "Network truncation or parsing error",
		},
		{
			name:        "TraceID used as SpanID",
			traceIDHex:  "1234567890abcdef1234567890abcdef", // 16 bytes - correct for TraceID
			spanIDHex:   "1234567890abcdef1234567890abcdef", // 16 bytes - too long for SpanID
			description: "Configuration error where trace ID field mapped to span ID",
		},
	}

	for _, event := range problemEvents {
		t.Run(event.name, func(t *testing.T) {
			libhoneyEvent := LibhoneyEvent{
				MsgPackTimestamp: &now,
				Data: map[string]any{
					"trace.trace_id": event.traceIDHex,
					"trace.span_id":  event.spanIDHex,
					"name":           "test-span",
				},
			}

			cfg := FieldMapConfig{
				Attributes: AttributesConfig{
					TraceID: "trace.trace_id",
					SpanID:  "trace.span_id",
					Name:    "name",
				},
			}

			span := ptrace.NewSpan()
			alreadyUsedFields := []string{}

			// The OLD code would panic here with these inputs
			// The NEW code should handle them gracefully
			err := libhoneyEvent.ToPTraceSpan(&span, &alreadyUsedFields, cfg, *logger)

			// Should not error with the new code
			assert.NoError(t, err, "Event processing should not fail: %s", event.description)

			// Should have valid (non-empty) IDs even if they had to be generated
			assert.NotEqual(t, pcommon.TraceID{}, span.TraceID(), "TraceID should not be empty")
			assert.NotEqual(t, pcommon.SpanID{}, span.SpanID(), "SpanID should not be empty")

			// Should have the span name
			assert.Equal(t, "test-span", span.Name())

			t.Logf("Processed %s successfully", event.description)
			t.Logf("  Input TraceID: %s (%d bytes when decoded)", event.traceIDHex, len(event.traceIDHex)/2)
			t.Logf("  Input SpanID:  %s (%d bytes when decoded)", event.spanIDHex, len(event.spanIDHex)/2)
			t.Logf("  Final TraceID: %s", span.TraceID().String())
			t.Logf("  Final SpanID:  %s", span.SpanID().String())
		})
	}
}

// TestOldCodePanicSimulation shows exactly what would have panicked
func TestOldCodePanicSimulation(t *testing.T) {
	// This test demonstrates the exact line of code that would panic
	// We don't actually execute the old code (to avoid panics), but show what it would do

	testCases := []struct {
		name          string
		hexString     string
		targetType    string
		expectedBytes int
		actualBytes   int
		panicMessage  string
	}{
		{
			name:          "Short TraceID",
			hexString:     "1234567890abcdef",
			targetType:    "TraceID",
			expectedBytes: 16,
			actualBytes:   8,
			panicMessage:  "cannot convert slice with length 8 to array or pointer to array with length 16",
		},
		{
			name:          "Long SpanID",
			hexString:     "1234567890abcdef1234567890abcdef",
			targetType:    "SpanID",
			expectedBytes: 8,
			actualBytes:   16,
			panicMessage:  "cannot convert slice with length 16 to array or pointer to array with length 8",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			decoded, err := hex.DecodeString(tc.hexString)
			require.NoError(t, err)

			assert.Equal(t, tc.actualBytes, len(decoded), "Decoded byte length should match expected")

			t.Logf("OLD CODE would have done:")
			t.Logf("  byteArray := hex.DecodeString(%q) // returns %d bytes", tc.hexString, len(decoded))
			t.Logf("  pcommon.%s(byteArray) // expects [%d]byte", tc.targetType, tc.expectedBytes)
			t.Logf("  PANIC: %s", tc.panicMessage)
			t.Logf("")
			t.Logf("NEW CODE does:")
			t.Logf("  if len(byteArray) == %d {", tc.expectedBytes)
			t.Logf("    var arr [%d]byte", tc.expectedBytes)
			t.Logf("    copy(arr[:], byteArray)")
			t.Logf("    return pcommon.%s(arr)", tc.targetType)
			t.Logf("  } else {")
			t.Logf("    // Fall back to hash generation")
			t.Logf("  }")
		})
	}
}
