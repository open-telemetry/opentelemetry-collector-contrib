// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package noop

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func TestBuildValid(t *testing.T) {
	cfg := NewConfigWithID("test")
	op, err := cfg.Build(testutil.Logger(t))
	require.NoError(t, err)
	require.IsType(t, &Transformer{}, op)
}

func TestBuildInvalid(t *testing.T) {
	cfg := NewConfigWithID("test")
	_, err := cfg.Build(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "build context is missing a logger")
}

func TestProcess(t *testing.T) {
	cfg := NewConfigWithID("test")
	cfg.OutputIDs = []string{"fake"}
	op, err := cfg.Build(testutil.Logger(t))
	require.NoError(t, err)

	fake := testutil.NewFakeOutput(t)
	require.NoError(t, op.SetOutputs([]operator.Operator{fake}))

	entry := entry.New()
	entry.AddAttribute("label", "value")
	entry.AddResourceKey("resource", "value")
	entry.TraceID = []byte{0x01}
	entry.SpanID = []byte{0x01}
	entry.TraceFlags = []byte{0x01}

	expected := entry.Copy()
	err = op.Process(context.Background(), entry)
	require.NoError(t, err)

	fake.ExpectEntry(t, expected)
}
