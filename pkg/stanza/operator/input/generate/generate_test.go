// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package generate

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func TestInputGenerate(t *testing.T) {
	cfg := NewConfig("test_operator_id")
	cfg.OutputIDs = []string{"fake"}
	cfg.Count = 5
	cfg.Entry = entry.Entry{
		Body: "test message",
	}

	op, err := cfg.Build(testutil.Logger(t))
	require.NoError(t, err)

	fake := testutil.NewFakeOutput(t)
	err = op.SetOutputs([]operator.Operator{fake})
	require.NoError(t, err)

	require.NoError(t, op.Start(testutil.NewMockPersister("test")))
	defer func() {
		require.NoError(t, op.Stop())
	}()

	for i := 0; i < 5; i++ {
		fake.ExpectBody(t, "test message")
	}
}
