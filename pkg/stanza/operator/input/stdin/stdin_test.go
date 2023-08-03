// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package stdin

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func TestStdin(t *testing.T) {
	cfg := NewConfig("")
	cfg.OutputIDs = []string{"fake"}

	op, err := cfg.Build(testutil.Logger(t))
	require.NoError(t, err)

	fake := testutil.NewFakeOutput(t)
	require.NoError(t, op.SetOutputs([]operator.Operator{fake}))

	r, w, err := os.Pipe()
	require.NoError(t, err)

	stdin := op.(*Input)
	stdin.stdin = r

	require.NoError(t, stdin.Start(testutil.NewMockPersister("test")))
	defer func() {
		require.NoError(t, stdin.Stop())
	}()

	_, err = w.WriteString("test")
	require.NoError(t, err)
	w.Close()
	fake.ExpectBody(t, "test")
}
