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
	"go.opentelemetry.io/collector/pdata/ptrace"
	// "go.opentelemetry.io/collector/component/componenttest"
	// "go.opentelemetry.io/collector/consumer/consumertest"
	// "go.opentelemetry.io/collector/receiver/receivertest"
)

func TestNewParentSpanID(t *testing.T) {
	tests := []struct {
		runID                int64
		runAttempt           int
		expectedParentSpanID string
		expectedError        error
	}{
		{
			runID:                12345,
			runAttempt:           1,
			expectedParentSpanID: pcommon.SpanID{},
			expectedError:        nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.expectedParentSpanID, func(t *testing.T) {
			parentSpanID, err := newParentSpanID(tc.runID, tc.runAttempt)
			require.Equal(t, tc.expectedParentSpanID, parentSpanID)
			require.Equal(t, tc.expectedError, err)
		})
	}

}
