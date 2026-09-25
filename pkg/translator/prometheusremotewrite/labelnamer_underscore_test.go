// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewrite

import (
	"fmt"
	"testing"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
	prometheustranslator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"
)

// When the permissive-label-sanitization feature gate is enabled, label names
// with consecutive underscores must be preserved (a__b stays a__b) rather than
// collapsed to a single underscore. When the gate is disabled, the default
// sanitization still collapses them.
func TestNewPrometheusConverterLabelNamerMultipleUnderscores(t *testing.T) {
	t.Run("gate enabled preserves multiple underscores", func(t *testing.T) {
		defer testutil.SetFeatureGateForTest(t, prometheustranslator.DropSanitizationGate, true)()

		converter := newPrometheusConverter(Settings{})
		got, err := converter.labelNamer.Build("a__b")
		require.NoError(t, err)
		require.Equal(t, "a__b", got)
	})

	t.Run("gate disabled collapses multiple underscores", func(t *testing.T) {
		defer testutil.SetFeatureGateForTest(t, prometheustranslator.DropSanitizationGate, false)()

		converter := newPrometheusConverter(Settings{})
		got, err := converter.labelNamer.Build("a__b")
		require.NoError(t, err)
		require.Equal(t, "a_b", got)
	})

	for _, gateEnabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("reserved double-underscore label is preserved with gate=%t", gateEnabled), func(t *testing.T) {
			defer testutil.SetFeatureGateForTest(t, prometheustranslator.DropSanitizationGate, gateEnabled)()

			converter := newPrometheusConverter(Settings{})
			got, err := converter.labelNamer.Build("__name__")
			require.NoError(t, err)
			require.Equal(t, "__name__", got)
		})
	}
}

// The remote-write v2 converter builds its own LabelNamer, so it needs the same
// treatment — otherwise FromMetricsV2 keeps collapsing underscores with the gate on.
func TestNewPrometheusConverterV2LabelNamerMultipleUnderscores(t *testing.T) {
	t.Run("gate enabled preserves multiple underscores", func(t *testing.T) {
		defer testutil.SetFeatureGateForTest(t, prometheustranslator.DropSanitizationGate, true)()

		converter := newPrometheusConverterV2(Settings{})
		got, err := converter.labelNamer.Build("a__b")
		require.NoError(t, err)
		require.Equal(t, "a__b", got)
	})

	t.Run("gate disabled collapses multiple underscores", func(t *testing.T) {
		defer testutil.SetFeatureGateForTest(t, prometheustranslator.DropSanitizationGate, false)()

		converter := newPrometheusConverterV2(Settings{})
		got, err := converter.labelNamer.Build("a__b")
		require.NoError(t, err)
		require.Equal(t, "a_b", got)
	})
}

func TestAddResourceTargetInfoMultipleUnderscores(t *testing.T) {
	resource := pcommon.NewResource()
	resource.Attributes().PutStr("service.name", "my_service")
	resource.Attributes().PutStr("app__component", "backend")

	t.Run("gate enabled preserves multiple underscores in target_info", func(t *testing.T) {
		defer testutil.SetFeatureGateForTest(t, prometheustranslator.DropSanitizationGate, true)()

		converter := newPrometheusConverter(Settings{})
		err := converter.addResourceTargetInfo(resource, Settings{}, testdata.TestMetricStartTimestamp)
		require.NoError(t, err)

		wantLabels := []prompb.Label{
			{Name: model.MetricNameLabel, Value: "target_info"},
			{Name: "app__component", Value: "backend"},
			{Name: model.JobLabel, Value: "my_service"},
		}
		require.Len(t, converter.unique, 1)
		for _, ts := range converter.unique {
			require.Equal(t, wantLabels, ts.Labels)
		}
	})

	t.Run("gate disabled collapses multiple underscores in target_info", func(t *testing.T) {
		defer testutil.SetFeatureGateForTest(t, prometheustranslator.DropSanitizationGate, false)()

		converter := newPrometheusConverter(Settings{})
		err := converter.addResourceTargetInfo(resource, Settings{}, testdata.TestMetricStartTimestamp)
		require.NoError(t, err)

		wantLabels := []prompb.Label{
			{Name: model.MetricNameLabel, Value: "target_info"},
			{Name: "app_component", Value: "backend"},
			{Name: model.JobLabel, Value: "my_service"},
		}
		require.Len(t, converter.unique, 1)
		for _, ts := range converter.unique {
			require.Equal(t, wantLabels, ts.Labels)
		}
	})
}
