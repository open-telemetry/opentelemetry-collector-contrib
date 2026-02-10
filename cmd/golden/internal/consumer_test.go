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

func TestConsumeMetrics_MultipleAttempts(t *testing.T) {
	s, err := NewConsumer(&Config{
		ExpectedFile:  filepath.Join("testdata", "expected.yaml"),
		WriteExpected: false,
		CompareOptions: []pmetrictest.CompareMetricsOption{
			pmetrictest.IgnoreTimestamp(),
			pmetrictest.IgnoreStartTimestamp(),
		},
	})
	require.NoError(t, err)

	// Attempt 1: Wrong metric name
	m1 := pmetric.NewMetrics()
	mm1 := m1.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	mm1.SetName("wrong_metric_1")
	mm1.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(1)
	err = s.ConsumeMetrics(t.Context(), m1)
	require.NoError(t, err) // ConsumeMetrics stores errors, doesn't return them

	// Attempt 2: Different wrong metric
	m2 := pmetric.NewMetrics()
	mm2 := m2.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	mm2.SetName("wrong_metric_2")
	mm2.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(2)
	err = s.ConsumeMetrics(t.Context(), m2)
	require.NoError(t, err)

	// Attempt 3: Another wrong metric
	m3 := pmetric.NewMetrics()
	mm3 := m3.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	mm3.SetName("wrong_metric_3")
	mm3.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(3)
	err = s.ConsumeMetrics(t.Context(), m3)
	require.NoError(t, err)

	// Verify all 3 errors were collected
	require.Len(t, s.Errors, 3, "should have collected 3 errors")
	require.Equal(t, 1, s.Errors[0].AttemptNumber)
	require.Equal(t, 2, s.Errors[1].AttemptNumber)
	require.Equal(t, 3, s.Errors[2].AttemptNumber)

	// Verify each error is different
	require.NotEqual(t, s.Errors[0].Error.Error(), s.Errors[1].Error.Error())
	require.NotEqual(t, s.Errors[1].Error.Error(), s.Errors[2].Error.Error())
}

func TestConsumeMetrics_ErrorsClearedOnSuccess(t *testing.T) {
	s, err := NewConsumer(&Config{
		ExpectedFile:  filepath.Join("testdata", "expected.yaml"),
		WriteExpected: false,
		CompareOptions: []pmetrictest.CompareMetricsOption{
			pmetrictest.IgnoreTimestamp(),
			pmetrictest.IgnoreStartTimestamp(),
		},
	})
	require.NoError(t, err)

	// Attempt 1: Send wrong metrics
	m1 := pmetric.NewMetrics()
	mm1 := m1.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	mm1.SetName("wrong_metric")
	mm1.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(999)
	err = s.ConsumeMetrics(t.Context(), m1)
	require.NoError(t, err)
	require.Len(t, s.Errors, 1, "should have 1 error after failed attempt")

	// Attempt 2: Send correct metrics (matching expected.yaml)
	m2 := pmetric.NewMetrics()
	mm2 := m2.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	mm2.SetName("foo")
	mm2.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(2)
	err = s.ConsumeMetrics(t.Context(), m2)
	require.NoError(t, err)

	// Errors should be cleared after success
	require.Empty(t, s.Errors, "errors should be cleared on successful comparison")
}

func TestConsumeMetrics_AttemptCounterIncrementsCorrectly(t *testing.T) {
	s, err := NewConsumer(&Config{
		ExpectedFile:  filepath.Join("testdata", "expected.yaml"),
		WriteExpected: false,
		CompareOptions: []pmetrictest.CompareMetricsOption{
			pmetrictest.IgnoreTimestamp(),
			pmetrictest.IgnoreStartTimestamp(),
		},
	})
	require.NoError(t, err)

	// Send 5 wrong attempts
	for i := 1; i <= 5; i++ {
		m := pmetric.NewMetrics()
		mm := m.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
		mm.SetName("wrong")
		mm.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(int64(i))
		err = s.ConsumeMetrics(t.Context(), m)
		require.NoError(t, err)
	}

	require.Len(t, s.Errors, 5)
	for i := range 5 {
		require.Equal(t, i+1, s.Errors[i].AttemptNumber, "attempt number should increment correctly")
	}
}
