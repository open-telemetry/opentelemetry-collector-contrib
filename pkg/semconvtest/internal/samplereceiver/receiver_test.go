// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package samplereceiver_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/semconvtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/semconvtest/internal/samplereceiver"
)

// TestSemconvCompliance demonstrates how a component author would use
// semconvtest to validate their component's telemetry against semantic
// conventions: produce pdata with the component, then hand it to
// semconvtest.TestMetrics along with the testing.TB.
func TestSemconvCompliance(t *testing.T) {
	factory := samplereceiver.NewFactory()
	sink := &consumertest.MetricsSink{}
	settings := receivertest.NewNopSettings(component.MustNewType("sample_http"))
	recv, err := factory.CreateMetrics(t.Context(), settings, factory.CreateDefaultConfig(), sink)
	require.NoError(t, err)

	require.NoError(t, recv.Start(t.Context(), componenttest.NewNopHost()))
	defer func() { require.NoError(t, recv.Shutdown(t.Context())) }()

	require.Len(t, sink.AllMetrics(), 1, "expected the receiver to produce one batch of metrics")

	semconvtest.TestMetrics(t, sink.AllMetrics()[0])
}
