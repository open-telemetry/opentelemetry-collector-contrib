// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sattributesprocessor

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"
)

func TestNodeUIDWithoutNodeName(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	// This is the specific configuration causing the bug:
	// Asking for UID but NOT asking for Name.
	cfg.Extract.Metadata = []string{"k8s.node.uid"}

	settings := processortest.NewNopSettings(factory.Type())
	// using t.Context() to satisfy the linter
	tp, err := factory.CreateTraces(t.Context(), settings, cfg, consumertest.NewNop())

	// If the fix is working, this should NOT return an error.
	require.NoError(t, err)
	require.NotNil(t, tp)
}