// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func TestConsumeMetrics(t *testing.T) {
	s, err := NewConsumer(&Config{
		ExpectedFile:  filepath.Join("testdata", "expected.yaml"),
		WriteExpected: false,
		CompareOptions: []pmetrictest.CompareMetricsOption{
			pmetrictest.IgnoreTimestamp(),
			pmetrictest.IgnoreStartTimestamp(),
		},
		OTLPEndpoint:    "localhost:4317",
		OTLPHTTPEndoint: "localhost:4318",
		Timeout:         0,
	})
	require.NoError(t, err)
	m := pmetric.NewMetrics()
	mm := m.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	mm.SetName("foo")
	mm.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(2)
	err = s.ConsumeMetrics(t.Context(), m)
	require.NoError(t, err)
}
