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

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

func TestTranslationSupportedVersion(t *testing.T) {
	t.Parallel()

	tn, err := newTranslatorFromReader(
		zaptest.NewLogger(t),
		"https://opentelemetry.io/schemas/1.9.0",
		LoadTranslationVersion(t, TranslationVersion190),
	)
	require.NoError(t, err, "Must not error when creating translator")

	tests := []struct {
		scenario  string
		version   *Version
		supported bool
	}{
		{
			scenario:  "Known supported version",
			version:   &Version{1, 0, 0},
			supported: true,
		},
		{
			scenario:  "Unsupported version",
			version:   &Version{1, 33, 7},
			supported: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.scenario, func(t *testing.T) {
			assert.Equal(
				t,
				tc.supported,
				tn.SupportedVersion(tc.version),
				"Must match the expected supported version",
			)
		})
	}
}

func TestTranslationIterator(t *testing.T) {
	t.Parallel()

	tests := []struct {
		scenario string
		target   string
		income   string
		status   int
	}{
		{
			scenario: "No update",
			target:   "https://opentelemetry.io/schemas/1.9.0",
			income:   "https://opentelemetry.io/schemas/1.9.0",
			status:   NoChange,
		},
		{
			scenario: "Update",
			target:   "https://opentelemetry.io/schemas/1.9.0",
			income:   "https://opentelemetry.io/schemas/1.6.1",
			status:   Update,
		},
		{
			scenario: "Revert",
			target:   "https://opentelemetry.io/schemas/1.6.1",
			income:   "https://opentelemetry.io/schemas/1.9.0",
			status:   Revert,
		},
		{
			scenario: "Unsupported / Unknown version",
			target:   "https://opentelemetry.io/schemas/1.6.1",
			income:   "https://opentelemetry.io/schemas/2.4.0",
			status:   NoChange,
		},
	}

	for _, tc := range tests {
		t.Run(tc.scenario, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)

			tn, err := newTranslatorFromReader(zaptest.NewLogger(t), tc.target, LoadTranslationVersion(t, TranslationVersion190))
			require.NoError(t, err, "Must have no error when creating translator")

			_, version, err := GetFamilyAndVersion(tc.income)
			require.NoError(t, err, "Must not error when parsing version from schemaURL")

			it, status := tn.iterator(ctx, version)
			assert.Equal(t, tc.status, status, "Must match the expected status")
			for rev, more := it(); more; rev, more = it() {
				switch status {
				case Update:
					version.LessThan(rev.Version())
				case Revert:
					version.GreaterThan(rev.Version())
				}
				version = rev.Version()
			}
		})
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	tn, err := newTranslatorFromReader(zaptest.NewLogger(t), "https://opentelemetry.io/schemas/1.9.0", LoadTranslationVersion(t, TranslationVersion190))
	require.NoError(t, err, "Must have no error when creating translator")

	ver := &Version{1, 0, 0}
	it, status := tn.iterator(ctx, ver)
	assert.Equal(t, Update, status, "Must provide an update status")

	count := 0
	for rev, more := it(); more; rev, more = it() {
		if count == 4 {
			cancel()
		}
		ver = rev.Version()
		count++
	}
	assert.EqualValues(t, &Version{1, 7, 0}, ver, "Must match the expected version number")
}


func TestTranslationSpanChanges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		scenario string
		target   Version
		income   Version
	}{
		{
			scenario: "No update",
			target:   Version{1, 1, 0},
			income:   Version{1, 1, 0},
		},
		{
			scenario: "Upgrade 1.0.0 -> 1.1.0",
			income:   Version{1, 0, 0},
			target:   Version{1, 1, 0},
		},
		{
			scenario: "Upgrade 1.0.0 -> 1.2.0",
			income:   Version{1, 0, 0},
			target:   Version{1, 2, 0},
		},
		{
			scenario: "Upgrade 1.0.0 -> 1.4.0",
			income:   Version{1, 0, 0},
			target:   Version{1, 4, 0},
		},
		{
			scenario: "Upgrade 1.0.0 -> 1.5.0",
			income:   Version{1, 0, 0},
			target:   Version{1, 5, 0},
		},
		{
			scenario: "Upgrade 1.0.0 -> 1.7.0",
			income:   Version{1, 0, 0},
			target:   Version{1, 7, 0},
		},
		{
			scenario: "Downgrade to original version",
			target:   Version{1, 0, 0},
			income:   Version{1, 7, 0},
		},
	}

	for _, tc := range tests {
		t.Run(tc.scenario, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)

			tn, err := newTranslatorFromReader(
				zaptest.NewLogger(t),
				joinSchemaFamilyAndVersion("https://example.com/", &tc.target),
				LoadTranslationVersion(t, "complex_changeset.yml"),
			)
			require.NoError(t, err, "Must not error creating translator")

			spans := NewExampleSpans(t, tc.income)
			for i := 0; i < spans.ResourceSpans().Len(); i++ {
				rSpan := spans.ResourceSpans().At(i)
				err := tn.ApplyAllResourceChanges(ctx, rSpan)
				require.NoError(t, err, "Must not error when applying resource changes")
				for j := 0; j < rSpan.ScopeSpans().Len(); j++ {
					span := rSpan.ScopeSpans().At(j)
					err = tn.ApplyScopeSpanChanges(ctx, span)
					require.NoError(t, err, "Must not error when applying scope span changes")
				}
			}
			expect := NewExampleSpans(t, tc.target)
			if diff := cmp.Diff(expect, spans, cmp.AllowUnexported(ptrace.Traces{})); diff != "" {
				t.Errorf("Span mismatch (-want +got):\n%s", diff)
			}
			assert.EqualValues(t, expect, spans, "Must match the expected values")
		})
	}
}

func TestTranslationLogChanges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		scenario string
		target   Version
		income   Version
	}{
		{
			scenario: "No update",
			target:   Version{1, 1, 0},
			income:   Version{1, 1, 0},
		},
		{
			scenario: "Upgrade 1.0.0 -> 1.1.0",
			income:   Version{1, 0, 0},
			target:   Version{1, 1, 0},
		},
		{
			scenario: "Upgrade 1.0.0 -> 1.2.0",
			income:   Version{1, 0, 0},
			target:   Version{1, 2, 0},
		},
		{
			scenario: "Upgrade 1.0.0 -> 1.4.0",
			income:   Version{1, 0, 0},
			target:   Version{1, 4, 0},
		},
		{
			scenario: "Upgrade 1.0.0 -> 1.5.0",
			income:   Version{1, 0, 0},
			target:   Version{1, 5, 0},
		},
		{
			scenario: "Upgrade 1.0.0 -> 1.7.0",
			income:   Version{1, 0, 0},
			target:   Version{1, 7, 0},
		},
		{
			scenario: "Downgrade to original version",
			target:   Version{1, 0, 0},
			income:   Version{1, 7, 0},
		},
	}

	for _, tc := range tests {
		t.Run(tc.scenario, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)

			tn, err := newTranslatorFromReader(
				zaptest.NewLogger(t),
				joinSchemaFamilyAndVersion("https://example.com/", &tc.target),
				LoadTranslationVersion(t, "complex_changeset.yml"),
			)
			require.NoError(t, err, "Must not error creating translator")

			logs := NewExampleLogs(t, tc.income)
			for i := 0; i < logs.ResourceLogs().Len(); i++ {
				rLogs := logs.ResourceLogs().At(i)
				err = tn.ApplyAllResourceChanges(ctx, rLogs)
				require.NoError(t, err, "Must not error when applying resource changes")
				for j := 0; j < rLogs.ScopeLogs().Len(); j++ {
					log := rLogs.ScopeLogs().At(j)
					err = tn.ApplyScopeLogChanges(ctx, log)
					require.NoError(t, err, "Must not error when applying scope log changes")
				}
			}
			expect := NewExampleLogs(t, tc.target)
			assert.EqualValues(t, expect, logs, "Must match the expected values")
		})
	}
}

func TestTranslationMetricChanges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		scenario string
		target   Version
		income   Version
	}{
		{
			scenario: "No update",
			target:   Version{1, 1, 0},
			income:   Version{1, 1, 0},
		},
		{
			scenario: "Upgrade 1.0.0 -> 1.1.0",
			income:   Version{1, 0, 0},
			target:   Version{1, 1, 0},
		},
		{
			scenario: "Upgrade 1.0.0 -> 1.2.0",
			income:   Version{1, 0, 0},
			target:   Version{1, 2, 0},
		},
		{
			scenario: "Upgrade 1.0.0 -> 1.4.0",
			income:   Version{1, 0, 0},
			target:   Version{1, 4, 0},
		},
		{
			scenario: "Upgrade 1.0.0 -> 1.5.0",
			income:   Version{1, 0, 0},
			target:   Version{1, 5, 0},
		},
		{
			scenario: "Upgrade 1.0.0 -> 1.7.0",
			income:   Version{1, 0, 0},
			target:   Version{1, 7, 0},
		},
		{
			scenario: "Downgrade to original version",
			target:   Version{1, 0, 0},
			income:   Version{1, 7, 0},
		},
	}

	for _, tc := range tests {
		t.Run(tc.scenario, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)

			tn, err := newTranslatorFromReader(
				zaptest.NewLogger(t),
				joinSchemaFamilyAndVersion("https://example.com/", &tc.target),
				LoadTranslationVersion(t, "complex_changeset.yml"),
			)
			require.NoError(t, err, "Must not error creating translator")

			metrics := NewExampleMetrics(t, tc.income)
			for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
				rMetrics := metrics.ResourceMetrics().At(i)
				err = tn.ApplyAllResourceChanges(ctx, rMetrics)
				require.NoError(t, err, "Must not error when applying resource changes")
				for j := 0; j < rMetrics.ScopeMetrics().Len(); j++ {
					metric := rMetrics.ScopeMetrics().At(j)
					err := tn.ApplyScopeMetricChanges(ctx, metric)
					require.NoError(t, err, "Must not error when applying scope metric changes")
				}
			}
			expect := NewExampleMetrics(t, tc.target)
			assert.EqualValues(t, expect, metrics, "Must match the expected values")
		})
	}
}

func TestTranslationEquvialance_Logs(t *testing.T) {
	t.Parallel()

	ctx, done := context.WithCancel(context.Background())
	t.Cleanup(done)

	a, b := NewExampleLogs(t, Version{1, 0, 0}), NewExampleLogs(t, Version{1, 7, 0})

	tn, err := newTranslatorFromReader(
		zaptest.NewLogger(t),
		"https://example.com/1.4.0",
		LoadTranslationVersion(t, "complex_changeset.yml"),
	)
	require.NoError(t, err, "Must not error creating translator")

	for _, logs := range []plog.Logs{a, b} {
		for i := 0; i < logs.ResourceLogs().Len(); i++ {
			rLogs := logs.ResourceLogs().At(i)
			err = tn.ApplyAllResourceChanges(ctx, rLogs)
			require.NoError(t, err, "Must not error when applying resource changes")
			for j := 0; j < rLogs.ScopeLogs().Len(); j++ {
				log := rLogs.ScopeLogs().At(j)
				err = tn.ApplyScopeLogChanges(ctx, log)
				require.NoError(t, err, "Must not error when applying scope log changes")
			}
		}
	}
	expect := NewExampleLogs(t, Version{1, 4, 0})
	assert.EqualValues(t, expect, a, "Must match the expected value when upgrading versions")
	assert.EqualValues(t, expect, b, "Must match the expected value when reverting versions")
}

func TestTranslationEquvialance_Metrics(t *testing.T) {
	t.Parallel()

	ctx, done := context.WithCancel(context.Background())
	t.Cleanup(done)

	a, b := NewExampleMetrics(t, Version{1, 0, 0}), NewExampleMetrics(t, Version{1, 7, 0})

	tn, err := newTranslatorFromReader(
		zaptest.NewLogger(t),
		"https://example.com/1.4.0",
		LoadTranslationVersion(t, "complex_changeset.yml"),
	)
	require.NoError(t, err, "Must not error creating translator")

	for _, metrics := range []pmetric.Metrics{a, b} {
		for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
			rMetrics := metrics.ResourceMetrics().At(i)
			err = tn.ApplyAllResourceChanges(ctx, rMetrics)
			require.NoError(t, err, "Must not error when applying resource changes")
			for j := 0; j < rMetrics.ScopeMetrics().Len(); j++ {
				metric := rMetrics.ScopeMetrics().At(j)
				err = tn.ApplyScopeMetricChanges(ctx, metric)
				require.NoError(t, err, "Must not error when applying scope metric changes")
			}
		}
	}
	expect := NewExampleMetrics(t, Version{1, 4, 0})
	assert.EqualValues(t, expect, a, "Must match the expected value when upgrading versions")
	assert.EqualValues(t, expect, b, "Must match the expected value when reverting versions")
}

func TestTranslationEquvialance_Traces(t *testing.T) {
	t.Parallel()

	ctx, done := context.WithCancel(context.Background())
	t.Cleanup(done)

	a, b := NewExampleSpans(t, Version{1, 0, 0}), NewExampleSpans(t, Version{1, 7, 0})

	tn, err := newTranslatorFromReader(
		zaptest.NewLogger(t),
		"https://example.com/1.4.0",
		LoadTranslationVersion(t, "complex_changeset.yml"),
	)
	require.NoError(t, err, "Must not error creating translator")

	for _, traces := range []ptrace.Traces{a, b} {
		for i := 0; i < traces.ResourceSpans().Len(); i++ {
			rSpans := traces.ResourceSpans().At(i)
			err = tn.ApplyAllResourceChanges(ctx, rSpans)
			require.NoError(t, err, "Must not error when applying resource changes")
			for j := 0; j < rSpans.ScopeSpans().Len(); j++ {
				spans := rSpans.ScopeSpans().At(j)
				err = tn.ApplyScopeSpanChanges(ctx, spans)
				require.NoError(t, err, "Must not error when applying scope span changes")
			}
		}
	}
	expect := NewExampleSpans(t, Version{1, 4, 0})
	assert.EqualValues(t, expect, a, "Must match the expected value when upgrading versions")
	assert.EqualValues(t, expect, b, "Must match the expected value when reverting versions")
}

func BenchmarkCreatingTranslation(b *testing.B) {
	log := zap.NewNop()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		tn, err := newTranslatorFromReader(
			log,
			"https://opentelemetry.io/schemas/1.9.0",
			LoadTranslationVersion(b, TranslationVersion190),
		)
		assert.NoError(b, err, "Must not error when creating translator")
		assert.NotNil(b, tn)
	}
}

func BenchmarkUpgradingMetrics(b *testing.B) {
	ctx := context.Background()

	tn, err := newTranslatorFromReader(
		zap.NewNop(),
		"https://example.com/1.7.0",
		LoadTranslationVersion(b, "complex_changeset.yml"),
	)
	require.NoError(b, err, "Must not error creating translator")

	metrics := NewExampleMetrics(b, Version{1, 0, 0})

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		m := pmetric.NewMetrics()
		metrics.CopyTo(m)
		b.StartTimer()
		for i := 0; i < m.ResourceMetrics().Len(); i++ {
			rMetrics := m.ResourceMetrics().At(i)
			err = tn.ApplyAllResourceChanges(ctx, rMetrics)
			require.NoError(b, err, "Must not error when applying resource changes")
			for j := 0; j < rMetrics.ScopeMetrics().Len(); j++ {
				metric := rMetrics.ScopeMetrics().At(j)
				err = tn.ApplyScopeMetricChanges(ctx, metric)
				require.NoError(b, err, "Must not error when applying scope metric changes")
			}
		}
	}
}

func BenchmarkUpgradingTraces(b *testing.B) {
	ctx := context.Background()

	tn, err := newTranslatorFromReader(
		zap.NewNop(),
		"https://example.com/1.7.0",
		LoadTranslationVersion(b, "complex_changeset.yml"),
	)
	require.NoError(b, err, "Must not error creating translator")

	traces := NewExampleSpans(b, Version{1, 0, 0})

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		t := ptrace.NewTraces()
		traces.CopyTo(t)
		b.StartTimer()
		for i := 0; i < t.ResourceSpans().Len(); i++ {
			rSpans := t.ResourceSpans().At(i)
			err = tn.ApplyAllResourceChanges(ctx, rSpans)
			require.NoError(b, err, "Must not error when applying resource changes")
			for j := 0; j < rSpans.ScopeSpans().Len(); j++ {
				spans := rSpans.ScopeSpans().At(j)
				err = tn.ApplyScopeSpanChanges(ctx, spans)
				require.NoError(b, err, "Must not error when applying scope span changes")
			}
		}
	}
}

func BenchmarkUpgradingLogs(b *testing.B) {
	ctx := context.Background()

	tn, err := newTranslatorFromReader(
		zap.NewNop(),
		"https://example.com/1.7.0",
		LoadTranslationVersion(b, "complex_changeset.yml"),
	)
	require.NoError(b, err, "Must not error creating translator")

	logs := NewExampleLogs(b, Version{1, 0, 0})

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		l := plog.NewLogs()
		logs.CopyTo(l)
		b.StartTimer()
		for i := 0; i < l.ResourceLogs().Len(); i++ {
			rLogs := l.ResourceLogs().At(i)
			err = tn.ApplyAllResourceChanges(ctx, rLogs)
			require.NoError(b, err, "Must not error when applying resource changes")
			for j := 0; j < rLogs.ScopeLogs().Len(); j++ {
				log := rLogs.ScopeLogs().At(j)
				err = tn.ApplyScopeLogChanges(ctx, log)
				require.NoError(b, err, "Must not error when applying scope log changes")
			}
		}
	}
}
