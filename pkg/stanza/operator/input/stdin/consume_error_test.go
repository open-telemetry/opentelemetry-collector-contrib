// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package stdin

import (
	"os"
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

// TestStdinKeepsReadingAfterConsumeError checks that a failed delivery does not
// stop the input: the next line is still read and sent.
func TestStdinKeepsReadingAfterConsumeError(t *testing.T) {
	cfg := NewConfig("")
	cfg.OutputIDs = []string{"fake"}
	op, err := cfg.Build(componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)

	received := make(chan any, 2)
	output := &testutil.Operator{}
	output.On("ID").Return("fake")
	output.On("CanProcess").Return(true)
	output.On("Process", mock.Anything, mock.Anything).Return(&helper.ConsumeError{}).Once()
	output.On("Process", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		received <- args.Get(1).(*entry.Entry).Body
	}).Return(nil)
	require.NoError(t, op.SetOutputs([]operator.Operator{output}))

	r, w, err := os.Pipe()
	require.NoError(t, err)
	stdin := op.(*Input)
	stdin.stdin = r
	require.NoError(t, stdin.Start(testutil.NewUnscopedMockPersister()))
	defer func() { require.NoError(t, stdin.Stop()) }()

	_, err = w.WriteString("first\nsecond\n")
	require.NoError(t, err)
	require.NoError(t, w.Close())

	select {
	case body := <-received:
		require.Equal(t, "second", body)
	case <-time.After(time.Second):
		t.Fatal("stdin stopped reading after a consume error")
	}
}
