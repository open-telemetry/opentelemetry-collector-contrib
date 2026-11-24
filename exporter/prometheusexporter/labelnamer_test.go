// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusexporter

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

func TestConfigureLabelNamer(t *testing.T) {
	for _, strategy := range []translationStrategy{"", underscoreEscapingWithSuffixes, underscoreEscapingWithoutSuffixes, noUTF8EscapingWithSuffixes, noTranslation} {
		t.Run(string(strategy), func(t *testing.T) {
			namer := configureLabelNamer(&Config{TranslationStrategy: strategy})
			for _, label := range []string{"_foo", "a__b", "__name__"} {
				t.Run(label, func(t *testing.T) {
					got, err := namer.Build(label)
					require.NoError(t, err)
					require.Equal(t, label, got)
				})
			}
		})
	}
}

func TestCollectorTargetInfoLabelSanitization(t *testing.T) {
	resourceAttrs := pcommon.NewMap()
	resourceAttrs.PutStr("service.name", "my_service")
	resourceAttrs.PutStr("service.instance.id", "my_instance")
	resourceAttrs.PutStr("app__component", "backend")

	t.Run("preserves multiple underscores in target_info", func(t *testing.T) {
		c := newCollector(&Config{}, zap.NewNop())
		metrics, err := c.createTargetInfoMetrics([]pcommon.Map{resourceAttrs})
		require.NoError(t, err)
		require.Len(t, metrics, 1)

		desc := metrics[0].Desc().String()
		require.Contains(t, desc, `app__component`)
	})
}

func TestCollectorMetricLabelSanitization(t *testing.T) {
	metric := pmetric.NewMetric()
	metric.SetName("test_metric")
	dp := metric.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.Attributes().PutStr("app__component", "backend")

	t.Run("preserves multiple underscores in metric labels", func(t *testing.T) {
		c := newCollector(&Config{}, zap.NewNop())
		m, err := c.convertMetric(metric, pcommon.NewMap(), "scope", "v1", "", pcommon.NewMap())
		require.NoError(t, err)

		desc := m.Desc().String()
		require.Contains(t, desc, `app__component`)
	})
}
