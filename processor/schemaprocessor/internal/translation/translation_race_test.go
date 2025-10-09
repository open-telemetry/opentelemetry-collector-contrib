// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translation

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/fixture"
)

func TestRaceTranslationSpanChanges(t *testing.T) {
	t.Parallel()

	tn, err := newTranslator(
		zap.NewNop(),
		"https://example.com/1.7.0",
		LoadTranslationVersion(t, "complex_changeset.yml"),
	)
	require.NoError(t, err, "Must not error when creating translator")

	fixture.ParallelRaceCompute(t, 10, func() error {
		for range 10 {
			v := &Version{1, 0, 0}
			spans := NewExampleSpans(t, *v)
			for i := 0; i < spans.ResourceSpans().Len(); i++ {
				rSpan := spans.ResourceSpans().At(i)
				if err := tn.ApplyAllResourceChanges(rSpan, rSpan.SchemaUrl()); err != nil {
					return err
				}
				for j := 0; j < rSpan.ScopeSpans().Len(); j++ {
					span := rSpan.ScopeSpans().At(j)
					if err := tn.ApplyScopeSpanChanges(span, span.SchemaUrl()); err != nil {
						return err
					}
				}
			}
		}
		return nil
	})
}

func TestRaceTranslationMetricChanges(t *testing.T) {
	t.Parallel()

	tn, err := newTranslator(
		zap.NewNop(),
		"https://example.com/1.7.0",
		LoadTranslationVersion(t, "complex_changeset.yml"),
	)
	require.NoError(t, err, "Must not error when creating translator")

	fixture.ParallelRaceCompute(t, 10, func() error {
		for range 10 {
			spans := NewExampleSpans(t, Version{1, 0, 0})
			for i := 0; i < spans.ResourceSpans().Len(); i++ {
				rSpan := spans.ResourceSpans().At(i)
				err := tn.ApplyAllResourceChanges(rSpan, rSpan.SchemaUrl())
				if err != nil {
					return err
				}
				for j := 0; j < rSpan.ScopeSpans().Len(); j++ {
					span := rSpan.ScopeSpans().At(j)
					err := tn.ApplyScopeSpanChanges(span, span.SchemaUrl())
					if err != nil {
						return err
					}
				}
			}
		}
		return nil
	})
}

func TestRaceTranslationLogChanges(t *testing.T) {
	t.Parallel()

	tn, err := newTranslator(
		zap.NewNop(),
		"https://example.com/1.7.0",
		LoadTranslationVersion(t, "complex_changeset.yml"),
	)
	require.NoError(t, err, "Must not error when creating translator")

	fixture.ParallelRaceCompute(t, 10, func() error {
		for range 10 {
			metrics := NewExampleMetrics(t, Version{1, 0, 0})
			for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
				rMetrics := metrics.ResourceMetrics().At(i)
				err := tn.ApplyAllResourceChanges(rMetrics, rMetrics.SchemaUrl())
				if err != nil {
					return err
				}
				for j := 0; j < rMetrics.ScopeMetrics().Len(); j++ {
					metric := rMetrics.ScopeMetrics().At(j)
					err := tn.ApplyScopeMetricChanges(metric, metric.SchemaUrl())
					if err != nil {
						return err
					}
				}
			}
		}
		return nil
	})
}
