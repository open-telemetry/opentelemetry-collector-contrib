// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package generate

import (
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

// TestGenerateKeepsGoingAfterConsumeError checks that a failed delivery does not
// stop the generator before it reaches its count.
func TestGenerateKeepsGoingAfterConsumeError(t *testing.T) {
	cfg := NewConfig("test_operator_id")
	cfg.OutputIDs = []string{"fake"}
	cfg.Count = 3
	cfg.Entry = entry.Entry{Body: "test message"}
	op, err := cfg.Build(componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)

	received := make(chan any, 3)
	output := &testutil.Operator{}
	output.On("ID").Return("fake")
	output.On("CanProcess").Return(true)
	output.On("Process", mock.Anything, mock.Anything).Return(&helper.ConsumeError{}).Once()
	output.On("Process", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		received <- args.Get(1).(*entry.Entry).Body
	}).Return(nil)
	require.NoError(t, op.SetOutputs([]operator.Operator{output}))
	require.NoError(t, op.Start(testutil.NewUnscopedMockPersister()))
	defer func() { require.NoError(t, op.Stop()) }()

	// The failed entry counts as one of the three attempts.
	for range 2 {
		select {
		case body := <-received:
			require.Equal(t, "test message", body)
		case <-time.After(time.Second):
			t.Fatal("generate stopped after a consume error")
		}
	}
}
