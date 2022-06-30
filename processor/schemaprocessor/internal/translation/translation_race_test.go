// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package translation

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/fixture"
)

func TestRaceTranslationSpanChanges(t *testing.T) {
	t.Parallel()

	tn, err := newTranslater(
		zap.NewNop(),
		"https://example.com/1.7.0",
		LoadTranslationVersion(t, "complex_changeset.yml"),
	)
	require.NoError(t, err, "Must not error when creating translator")

	ctx, done := context.WithCancel(context.Background())
	t.Cleanup(done)

	fixture.ParallelRaceCompute(t, 10, func() error {
		for i := 0; i < 10; i++ {
			v := &Version{1, 0, 0}
			spans := NewExampleSpans(t, *v)
			for i := 0; i < spans.ResourceSpans().Len(); i++ {
				rSpan := spans.ResourceSpans().At(i)
				tn.ApplyAllResourceChanges(ctx, rSpan)
				for j := 0; j < rSpan.ScopeSpans().Len(); j++ {
					span := rSpan.ScopeSpans().At(j)
					tn.ApplyScopeSpanChanges(ctx, span)
				}
			}
		}
		return nil
	})
}

func TestRaceTranslationMetricChanges(t *testing.T) {
	t.Parallel()

	tn, err := newTranslater(
		zap.NewNop(),
		"https://example.com/1.7.0",
		LoadTranslationVersion(t, "complex_changeset.yml"),
	)
	require.NoError(t, err, "Must not error when creating translator")

	ctx, done := context.WithCancel(context.Background())
	t.Cleanup(done)

	fixture.ParallelRaceCompute(t, 10, func() error {
		for i := 0; i < 10; i++ {
			spans := NewExampleSpans(t, Version{1, 0, 0})
			for i := 0; i < spans.ResourceSpans().Len(); i++ {
				rSpan := spans.ResourceSpans().At(i)
				tn.ApplyAllResourceChanges(ctx, rSpan)
				for j := 0; j < rSpan.ScopeSpans().Len(); j++ {
					span := rSpan.ScopeSpans().At(j)
					tn.ApplyScopeSpanChanges(ctx, span)
				}
			}
		}
		return nil
	})
}

func TestRaceTranslationLogChanges(t *testing.T) {
	t.Parallel()

	tn, err := newTranslater(
		zap.NewNop(),
		"https://example.com/1.7.0",
		LoadTranslationVersion(t, "complex_changeset.yml"),
	)
	require.NoError(t, err, "Must not error when creating translator")

	ctx, done := context.WithCancel(context.Background())
	t.Cleanup(done)

	fixture.ParallelRaceCompute(t, 10, func() error {
		for i := 0; i < 10; i++ {
			metrics := NewExampleMetrics(t, Version{1, 0, 0})
			for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
				rMetrics := metrics.ResourceMetrics().At(i)
				tn.ApplyAllResourceChanges(ctx, rMetrics)
				for j := 0; j < rMetrics.ScopeMetrics().Len(); j++ {
					metric := rMetrics.ScopeMetrics().At(j)
					tn.ApplyScopeMetricChanges(ctx, metric)
				}
			}
		}
		return nil
	})
}
