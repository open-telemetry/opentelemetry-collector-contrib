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
	cfg.Extract.Metadata = []string{"k8s.node.uid"}

	settings := processortest.NewNopSettings(factory.Type())
	tp, err := factory.CreateTraces(t.Context(), settings, cfg, consumertest.NewNop())

	require.NoError(t, err)
	require.NotNil(t, tp)
}
