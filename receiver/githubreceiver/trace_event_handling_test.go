// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver"

import (
	// "context"
	// "net/http"
	// "net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	// "go.opentelemetry.io/collector/pdata/ptrace"
	// "go.opentelemetry.io/collector/component/componenttest"
	// "go.opentelemetry.io/collector/consumer/consumertest"
	// "go.opentelemetry.io/collector/receiver/receivertest"
)

func TestNewParentSpanID(t *testing.T) {
    tests := []struct {
        name        string
        runID      int64
        runAttempt int
        wantError  bool
    }{
        {
            name:        "basic span ID generation",
            runID:      12345,
            runAttempt: 1,
            wantError:  false,
        },
        {
            name:        "different run ID",
            runID:      54321,
            runAttempt: 1,
            wantError:  false,
        },
        {
            name:        "different attempt",
            runID:      12345,
            runAttempt: 2,
            wantError:  false,
        },
        {
            name:        "zero values",
            runID:      0,
            runAttempt: 0,
            wantError:  false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // First call to get span ID
            spanID1, err1 := newParentSpanID(tt.runID, tt.runAttempt)

            if tt.wantError {
                require.Error(t, err1)
                return
            }
            require.NoError(t, err1)

            // Verify span ID is not empty
            require.NotEqual(t, pcommon.SpanID{}, spanID1, "span ID should not be empty")

            // Verify consistent results for same input
            spanID2, err2 := newParentSpanID(tt.runID, tt.runAttempt)
            require.NoError(t, err2)
            require.Equal(t, spanID1, spanID2, "same inputs should generate same span ID")

            // Verify different inputs generate different span IDs
            differentSpanID, err3 := newParentSpanID(tt.runID+1, tt.runAttempt)
            require.NoError(t, err3)
            require.NotEqual(t, spanID1, differentSpanID, "different inputs should generate different span IDs")
        })
    }
}

func TestNewParentSpanID_Consistency(t *testing.T) {
    // Test that generates the same span ID for same inputs across multiple calls
    runID := int64(12345)
    runAttempt := 1

    spanID1, err1 := newParentSpanID(runID, runAttempt)
    require.NoError(t, err1)

    for i := 0; i < 5; i++ {
        spanID2, err2 := newParentSpanID(runID, runAttempt)
        require.NoError(t, err2)
        require.Equal(t, spanID1, spanID2, "span ID should be consistent across multiple calls")
    }
}
