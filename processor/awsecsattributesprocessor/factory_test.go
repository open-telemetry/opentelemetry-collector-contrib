// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsecsattributesprocessor

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/collector/processor/xprocessor"
)

func TestCreateProcessorsInitError(t *testing.T) {
	factory := NewFactory()
	set := processortest.NewNopSettings(typ)
	// A bad regex makes Config.init fail, so every create func must return an error.
	bad := &Config{
		CacheTTL:    60,
		Attributes:  []string{"?="},
		ContainerID: ContainerID{Sources: []string{"container.id"}},
	}

	_, err := factory.CreateLogs(t.Context(), set, bad, consumertest.NewNop())
	require.Error(t, err)

	_, err = factory.CreateMetrics(t.Context(), set, bad, consumertest.NewNop())
	require.Error(t, err)

	_, err = factory.CreateTraces(t.Context(), set, bad, consumertest.NewNop())
	require.Error(t, err)

	_, err = factory.(xprocessor.Factory).CreateProfiles(t.Context(), set, bad, consumertest.NewNop())
	require.Error(t, err)
}
