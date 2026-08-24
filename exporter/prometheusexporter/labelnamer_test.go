// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusexporter

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	prometheustranslator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"
)

func TestConfigureLabelNamer(t *testing.T) {
	t.Run("gate enabled preserves multiple underscores and leading underscore", func(t *testing.T) {
		defer testutil.SetFeatureGateForTest(t, prometheustranslator.DropSanitizationGate, true)()

		namer := configureLabelNamer(&Config{})
		got, err := namer.Build("a__b")
		require.NoError(t, err)
		require.Equal(t, "a__b", got)

		got, err = namer.Build("_foo")
		require.NoError(t, err)
		require.Equal(t, "_foo", got)
	})

	t.Run("gate disabled collapses multiple underscores and prepends key to leading underscore", func(t *testing.T) {
		defer testutil.SetFeatureGateForTest(t, prometheustranslator.DropSanitizationGate, false)()

		namer := configureLabelNamer(&Config{})
		got, err := namer.Build("a__b")
		require.NoError(t, err)
		require.Equal(t, "a_b", got)

		got, err = namer.Build("_foo")
		require.NoError(t, err)
		require.Equal(t, "key_foo", got)
	})

	for _, gateEnabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("reserved double-underscore label is preserved with gate=%t", gateEnabled), func(t *testing.T) {
			defer testutil.SetFeatureGateForTest(t, prometheustranslator.DropSanitizationGate, gateEnabled)()

			namer := configureLabelNamer(&Config{})
			got, err := namer.Build("__name__")
			require.NoError(t, err)
			require.Equal(t, "__name__", got)
		})
	}
}

func TestCollectorTargetInfoLabelSanitization(t *testing.T) {
	resourceAttrs := pcommon.NewMap()
	resourceAttrs.PutStr("service.name", "my_service")
	resourceAttrs.PutStr("service.instance.id", "my_instance")
	resourceAttrs.PutStr("app__component", "backend")

	t.Run("gate enabled preserves multiple underscores in target_info", func(t *testing.T) {
		defer testutil.SetFeatureGateForTest(t, prometheustranslator.DropSanitizationGate, true)()

		c := newCollector(&Config{}, zap.NewNop())
		metrics, err := c.createTargetInfoMetrics([]pcommon.Map{resourceAttrs})
		require.NoError(t, err)
		require.Len(t, metrics, 1)

		desc := metrics[0].Desc().String()
		require.Contains(t, desc, `app__component`)
	})

	t.Run("gate disabled collapses multiple underscores in target_info", func(t *testing.T) {
		defer testutil.SetFeatureGateForTest(t, prometheustranslator.DropSanitizationGate, false)()

		c := newCollector(&Config{}, zap.NewNop())
		metrics, err := c.createTargetInfoMetrics([]pcommon.Map{resourceAttrs})
		require.NoError(t, err)
		require.Len(t, metrics, 1)

		desc := metrics[0].Desc().String()
		require.Contains(t, desc, `app_component`)
		require.NotContains(t, desc, `app__component`)
	})
}

func TestCollectorMetricLabelSanitization(t *testing.T) {
	metric := pmetric.NewMetric()
	metric.SetName("test_metric")
	dp := metric.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.Attributes().PutStr("app__component", "backend")

	t.Run("gate enabled preserves multiple underscores in metric labels", func(t *testing.T) {
		defer testutil.SetFeatureGateForTest(t, prometheustranslator.DropSanitizationGate, true)()

		c := newCollector(&Config{}, zap.NewNop())
		m, err := c.convertMetric(metric, pcommon.NewMap(), "scope", "v1", "", pcommon.NewMap())
		require.NoError(t, err)

		desc := m.Desc().String()
		require.Contains(t, desc, `app__component`)
	})

	t.Run("gate disabled collapses multiple underscores in metric labels", func(t *testing.T) {
		defer testutil.SetFeatureGateForTest(t, prometheustranslator.DropSanitizationGate, false)()

		c := newCollector(&Config{}, zap.NewNop())
		m, err := c.convertMetric(metric, pcommon.NewMap(), "scope", "v1", "", pcommon.NewMap())
		require.NoError(t, err)

		desc := m.Desc().String()
		require.Contains(t, desc, `app_component`)
		require.NotContains(t, desc, `app__component`)
	})
}
