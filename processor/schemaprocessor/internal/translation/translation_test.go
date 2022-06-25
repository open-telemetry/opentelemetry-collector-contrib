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
	_ "embed"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

func TestTranslationSupportedVersion(t *testing.T) {
	t.Parallel()

	tn, err := newTranslater(zaptest.NewLogger(t), "https://opentelemetry.io/schemas/1.9.0")
	require.NoError(t, err, "Must not error when creating translator")
	require.NoError(t, tn.merge(LoadTranslationVersion(t, TranslationVersion190)), "Must not have issue converting ast to translation")

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

			tn, err := newTranslater(zaptest.NewLogger(t), tc.target)
			require.NoError(t, err, "Must have no error when creating translator")
			require.NoError(t, tn.merge(LoadTranslationVersion(t, TranslationVersion190)), "Must not error when loading translation definition")

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

	tn, err := newTranslater(zaptest.NewLogger(t), "https://opentelemetry.io/schemas/1.9.0")
	require.NoError(t, err, "Must have no error when creating translator")
	require.NoError(t, tn.merge(LoadTranslationVersion(t, TranslationVersion190)), "Must not error when loading translation definition")

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

			tn, err := newTranslater(zaptest.NewLogger(t), fmt.Sprint("https://example.com/", tc.target.String()))
			require.NoError(t, err, "Must not error creating translator")
			require.NoError(t, tn.merge(LoadTranslationVersion(t, "complex_changeset.yml")), "Must not error when trying to load translation definition")

			spans := NewExampleSpans(t, tc.income)
			for i := 0; i < spans.ResourceSpans().Len(); i++ {
				rSpan := spans.ResourceSpans().At(i)
				tn.ApplyAllResourceChanges(ctx, rSpan)
				for j := 0; j < rSpan.ScopeSpans().Len(); j++ {
					span := rSpan.ScopeSpans().At(j)
					tn.ApplyScopeSpanChanges(ctx, span)
				}
			}
			expect := NewExampleSpans(t, tc.target)
			assert.EqualValues(t, expect, spans, "Must match the expected values")
			// expect, err := ptrace.NewJSONMarshaler().MarshalTraces(expect)
			// require.NoError(t, err, "Must not error when marshaling traces")
			// t.Log("Expect: ", string(expect))

			// actual, err := ptrace.NewJSONMarshaler().MarshalTraces(spans)
			// require.NoError(t, err, "Must not error when marshaling traces")
			// t.Log("Actual: ", string(actual))

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

			tn, err := newTranslater(zaptest.NewLogger(t), fmt.Sprint("https://example.com/", tc.target.String()))
			require.NoError(t, err, "Must not error creating translator")
			require.NoError(t, tn.merge(LoadTranslationVersion(t, "complex_changeset.yml")), "Must not error when trying to load translation definition")

			logs := NewExampleLogs(t, tc.income)
			for i := 0; i < logs.ResourceLogs().Len(); i++ {
				rLogs := logs.ResourceLogs().At(i)
				tn.ApplyAllResourceChanges(ctx, rLogs)
				for j := 0; j < rLogs.ScopeLogs().Len(); j++ {
					log := rLogs.ScopeLogs().At(j)
					tn.ApplyScopeLogChanges(ctx, log)
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

			tn, err := newTranslater(zaptest.NewLogger(t), fmt.Sprint("https://example.com/", tc.target.String()))
			require.NoError(t, err, "Must not error creating translator")
			require.NoError(t, tn.merge(LoadTranslationVersion(t, "complex_changeset.yml")), "Must not error when trying to load translation definition")

			logs := NewExampleMetrics(t, tc.income)
			for i := 0; i < logs.ResourceMetrics().Len(); i++ {
				rMetrics := logs.ResourceMetrics().At(i)
				tn.ApplyAllResourceChanges(ctx, rMetrics)
				for j := 0; j < rMetrics.ScopeMetrics().Len(); j++ {
					metric := rMetrics.ScopeMetrics().At(j)
					tn.ApplyScopeMetricChanges(ctx, metric)
				}
			}
			expect := NewExampleMetrics(t, tc.target)
			assert.EqualValues(t, expect, logs, "Must match the expected values")
		})
	}
}

func BenchmarkTranslationColdStartTranslation(b *testing.B) {
	log := zap.NewNop()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		tn, err := newTranslater(log, "https://opentelemetry.io/schemas/1.9.0")
		assert.NoError(b, err, "Must not error when creating translator")

		b.StartTimer()
		assert.NoError(b, tn.merge(LoadTranslationVersion(b, TranslationVersion190)), "Must not error when merging data")
	}
}

func BenchmarkTranslationPartialUpdateTranslation(b *testing.B) {
	log := zap.NewNop()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		tn, err := newTranslater(log, "https://opentelemetry.io/schemas/1.6.1")
		assert.NoError(b, err, "Must not error when creating translator")
		assert.NoError(b, tn.merge(LoadTranslationVersion(b, TranslationVersion161)), "Must not error when merging base")

		b.StartTimer()
		assert.NoError(b, tn.merge(LoadTranslationVersion(b, TranslationVersion190)), "Must not error when merging update")
	}
}
