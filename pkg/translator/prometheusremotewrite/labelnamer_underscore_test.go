// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewrite

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
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

	t.Run("reserved double-underscore label is preserved regardless of gate", func(t *testing.T) {
		defer testutil.SetFeatureGateForTest(t, prometheustranslator.DropSanitizationGate, false)()

		converter := newPrometheusConverter(Settings{})
		got, err := converter.labelNamer.Build("__name__")
		require.NoError(t, err)
		require.Equal(t, "__name__", got)
	})
}
