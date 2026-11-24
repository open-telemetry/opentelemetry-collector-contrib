// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package namedpipe

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

// TestPipeKeepsBufferedLinesAfterConsumeError checks that a failed delivery does
// not throw away lines the scanner already read from the pipe. They cannot be
// read from a pipe a second time.
func TestPipeKeepsBufferedLinesAfterConsumeError(t *testing.T) {
	conf := NewConfig()
	conf.Path = filename(t)
	conf.Permissions = 0o666
	conf.OutputIDs = []string{"fake"}
	op, err := conf.Build(componenttest.NewNopTelemetrySettings())
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

	pipe, err := os.OpenFile(conf.Path, os.O_WRONLY, 0)
	require.NoError(t, err)
	defer func() { require.NoError(t, pipe.Close()) }()

	// One write, so the scanner reads all three lines into its buffer at once.
	_, err = pipe.WriteString("log1\nlog2\nlog3\n")
	require.NoError(t, err)

	for _, want := range []string{"log2", "log3"} {
		select {
		case body := <-received:
			require.Equal(t, want, body)
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for %s", want)
		}
	}
}
