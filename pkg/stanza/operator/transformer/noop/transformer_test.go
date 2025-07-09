// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package noop

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func TestProcess(t *testing.T) {
	cfg := NewConfigWithID("test")
	cfg.OutputIDs = []string{"fake"}
	set := componenttest.NewNopTelemetrySettings()
	op, err := cfg.Build(set)
	require.NoError(t, err)

	fake := testutil.NewFakeOutput(t)
	require.NoError(t, op.SetOutputs([]operator.Operator{fake}))

	val := entry.New()
	val.AddAttribute("label", "value")
	val.AddResourceKey("resource", "value")
	val.TraceID = []byte{0x01}
	val.SpanID = []byte{0x01}
	val.TraceFlags = []byte{0x01}

	expected := val.Copy()
	err = op.ProcessBatch(context.Background(), []*entry.Entry{val})
	require.NoError(t, err)

	fake.ExpectEntry(t, expected)
}
