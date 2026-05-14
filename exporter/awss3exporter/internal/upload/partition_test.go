// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package upload

import (
	"testing"
	"text/template"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/config/configcompression"
)

func TestBuildKey_LegacyTemplate(t *testing.T) {
	t.Parallel()

	key := buildLegacyTemplateKey(
		"logs",
		template.Must(parseLegacyTemplate("{{.Prefix}}/{{.Date}}/{{.UUID}}.json.gz")),
		time.Date(2026, 4, 4, 12, 30, 0, 0, time.UTC),
	)

	assert.Regexp(t, `^logs/2026/04/04/.+\.json\.gz$`, key)
}

func TestPartitionKeyBuilderBuild_LegacyTemplateIncludesBasePrefix(t *testing.T) {
	t.Parallel()

	key := (&PartitionKeyBuilder{
		PartitionBasePrefix:       "base/path",
		PartitionPrefix:           "telemetry",
		LegacyS3KeyTemplate:       "{{.Prefix}}/{{.Date}}/{{.UUID}}.json.gz",
		LegacyS3KeyTemplateParsed: template.Must(parseLegacyTemplate("{{.Prefix}}/{{.Date}}/{{.UUID}}.json.gz")),
	}).Build(time.Date(2026, 4, 4, 12, 30, 0, 0, time.UTC), "")

	assert.Regexp(t, `^base/path/telemetry/2026/04/04/.+\.json\.gz$`, key)
}

func TestPartitionKeyInputsNewPartitionKey(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name           string
		inputs         *PartitionKeyBuilder
		expect         string
		overridePrefix string
	}{
		{
			name: "empty values",
			inputs: &PartitionKeyBuilder{
				UniqueKeyFunc: func() string {
					return "fixed"
				},
			},
			expect:         "_fixed",
			overridePrefix: "",
		},
		{
			name: "fixed key with prefix",
			inputs: &PartitionKeyBuilder{
				PartitionPrefix: "telemetry",
				UniqueKeyFunc: func() string {
					return "fixed"
				},
			},
			expect:         "telemetry/_fixed",
			overridePrefix: "",
		},
		{
			name: "empty partition",
			inputs: &PartitionKeyBuilder{
				PartitionPrefix: "telemetry/foo",
				PartitionFormat: "",
				FilePrefix:      "signal-output-",
				FileFormat:      "metrics",
				UniqueKeyFunc: func() string {
					return "fixed"
				},
			},
			expect:         "telemetry/foo/signal-output-_fixed.metrics",
			overridePrefix: "",
		},
		{
			name: "no compression set",
			inputs: &PartitionKeyBuilder{
				PartitionPrefix: "/telemetry",
				PartitionFormat: "year=%Y/month=%m/day=%d/hour=%H/minute=%M",
				FilePrefix:      "signal-output-",
				Metadata:        "service-01_pod2",
				FileFormat:      "metrics",
				UniqueKeyFunc: func() string {
					return "fixed"
				},
			},
			expect:         "/telemetry/service-01_pod2/year=2024/month=01/day=24/hour=06/minute=40/signal-output-service-01_pod2_fixed.metrics",
			overridePrefix: "",
		},
		{
			name: "gzip compression set",
			inputs: &PartitionKeyBuilder{
				PartitionPrefix: "/telemetry",
				PartitionFormat: "year=%Y/month=%m/day=%d/hour=%H/minute=%M",
				FilePrefix:      "signal-output-",
				Metadata:        "service-01_pod2",
				FileFormat:      "metrics",
				Compression:     configcompression.TypeGzip,
				UniqueKeyFunc: func() string {
					return "fixed"
				},
			},
			expect:         "/telemetry/service-01_pod2/year=2024/month=01/day=24/hour=06/minute=40/signal-output-service-01_pod2_fixed.metrics.gz",
			overridePrefix: "",
		},
		{
			name: "gzip compression set with overridePrefix",
			inputs: &PartitionKeyBuilder{
				PartitionPrefix: "/telemetry",
				PartitionFormat: "year=%Y/month=%m/day=%d/hour=%H/minute=%M",
				FilePrefix:      "signal-output-",
				Metadata:        "service-01_pod2",
				FileFormat:      "metrics",
				Compression:     configcompression.TypeGzip,
				UniqueKeyFunc: func() string {
					return "fixed"
				},
			},
			expect:         "/foo-prefix1/service-01_pod2/year=2024/month=01/day=24/hour=06/minute=40/signal-output-service-01_pod2_fixed.metrics.gz",
			overridePrefix: "/foo-prefix1",
		},
		{
			name: "base path only",
			inputs: &PartitionKeyBuilder{
				PartitionBasePrefix: "base/path",
				PartitionFormat:     "year=%Y/month=%m/day=%d/hour=%H/minute=%M",
				FilePrefix:          "signal-output-",
				Metadata:            "service-01_pod2",
				FileFormat:          "metrics",
				UniqueKeyFunc: func() string {
					return "fixed"
				},
			},
			expect:         "base/path/service-01_pod2/year=2024/month=01/day=24/hour=06/minute=40/signal-output-service-01_pod2_fixed.metrics",
			overridePrefix: "",
		},
		{
			name: "base path with prefix",
			inputs: &PartitionKeyBuilder{
				PartitionBasePrefix: "base/path",
				PartitionPrefix:     "telemetry",
				PartitionFormat:     "year=%Y/month=%m/day=%d/hour=%H/minute=%M",
				FilePrefix:          "signal-output-",
				Metadata:            "service-01_pod2",
				FileFormat:          "metrics",
				UniqueKeyFunc: func() string {
					return "fixed"
				},
			},
			expect:         "base/path/telemetry/service-01_pod2/year=2024/month=01/day=24/hour=06/minute=40/signal-output-service-01_pod2_fixed.metrics",
			overridePrefix: "",
		},
		{
			name: "base path with prefix and override",
			inputs: &PartitionKeyBuilder{
				PartitionBasePrefix: "base/path",
				PartitionPrefix:     "telemetry",
				PartitionFormat:     "year=%Y/month=%m/day=%d/hour=%H/minute=%M",
				FilePrefix:          "signal-output-",
				Metadata:            "service-01_pod2",
				FileFormat:          "metrics",
				UniqueKeyFunc: func() string {
					return "fixed"
				},
			},
			expect:         "base/path/override/service-01_pod2/year=2024/month=01/day=24/hour=06/minute=40/signal-output-service-01_pod2_fixed.metrics",
			overridePrefix: "override",
		},
		{
			name: "base path with empty prefix",
			inputs: &PartitionKeyBuilder{
				PartitionBasePrefix: "base/path",
				PartitionPrefix:     "",
				PartitionFormat:     "year=%Y/month=%m/day=%d/hour=%H/minute=%M",
				FilePrefix:          "signal-output-",
				Metadata:            "service-01_pod2",
				FileFormat:          "metrics",
				UniqueKeyFunc: func() string {
					return "fixed"
				},
			},
			expect:         "base/path/service-01_pod2/year=2024/month=01/day=24/hour=06/minute=40/signal-output-service-01_pod2_fixed.metrics",
			overridePrefix: "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ts := time.Date(2024, 0o1, 24, 6, 40, 20, 0, time.Local)

			assert.Equal(t, tc.expect, tc.inputs.Build(ts, tc.overridePrefix), "Must match the expected value")
		})
	}
}

func TestPartitionKeyInputsBucketPrefix(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name           string
		inputs         *PartitionKeyBuilder
		expect         string
		overridePrefix string
	}{
		{
			name:           "no values provided",
			inputs:         &PartitionKeyBuilder{},
			expect:         "",
			overridePrefix: "",
		},
		{
			name: "partition by minutes",
			inputs: &PartitionKeyBuilder{
				PartitionFormat: "year=%Y/month=%m/day=%d/hour=%H/minute=%M",
			},
			expect:         "year=2024/month=01/day=24/hour=06/minute=40",
			overridePrefix: "",
		},
		{
			name: "partition by hours",
			inputs: &PartitionKeyBuilder{
				PartitionFormat: "%Y/%m/%d/%H/%M",
			},
			expect:         "2024/01/24/06/40",
			overridePrefix: "",
		},
		{
			name:           "no values provided, overridePrefix is foo1",
			inputs:         &PartitionKeyBuilder{},
			expect:         "foo1/",
			overridePrefix: "foo1",
		},
		{
			name: "partition by minutes, overridePrefix is bar2",
			inputs: &PartitionKeyBuilder{
				PartitionFormat: "year=%Y/month=%m/day=%d/hour=%H/minute=%M",
			},
			expect:         "bar2/year=2024/month=01/day=24/hour=06/minute=40",
			overridePrefix: "bar2",
		},
		{
			name: "partition by hours, overridePrefix is foo3",
			inputs: &PartitionKeyBuilder{
				PartitionFormat: "%Y/%m/%d/%H/%M",
			},
			expect:         "foo3/2024/01/24/06/40",
			overridePrefix: "foo3",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ts := time.Date(2024, 0o1, 24, 6, 40, 20, 0, time.Local)

			assert.Equal(t, tc.expect, tc.inputs.bucketKeyPrefix(ts, tc.overridePrefix), "Must match the expected partition key")
		})
	}
}

func TestPartitionKeyInputsFilename(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		inputs *PartitionKeyBuilder
		expect string
	}{
		{
			name: "no values provided",
			inputs: &PartitionKeyBuilder{
				UniqueKeyFunc: func() string {
					return "buzz"
				},
			},
			expect: "_buzz",
		},
		{
			name: "no compression provided",
			inputs: &PartitionKeyBuilder{
				FilePrefix: "collector-capture-",
				FileFormat: "metrics",
				Metadata:   "service-01_pod1",
				UniqueKeyFunc: func() string {
					return "buzz"
				},
			},
			expect: "collector-capture-service-01_pod1_buzz.metrics",
		},
		{
			name: "valid compression set",
			inputs: &PartitionKeyBuilder{
				FilePrefix:  "collector-capture-",
				FileFormat:  "metrics",
				Metadata:    "service-01_pod1",
				Compression: configcompression.TypeGzip,
				UniqueKeyFunc: func() string {
					return "buzz"
				},
			},
			expect: "collector-capture-service-01_pod1_buzz.metrics.gz",
		},
		{
			name: "invalid compression set",
			inputs: &PartitionKeyBuilder{
				FilePrefix:  "collector-capture-",
				FileFormat:  "metrics",
				Metadata:    "service-01_pod1",
				Compression: configcompression.Type("foo"),
				UniqueKeyFunc: func() string {
					return "buzz"
				},
			},
			expect: "collector-capture-service-01_pod1_buzz.metrics",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.expect, tc.inputs.fileName(), "Must match the expected value")
		})
	}
}

func TestParseLegacyTemplate_DatadogKeyTemplate(t *testing.T) {
	t.Parallel()

	// Regression test: the datadog marshaler config generator emits an
	// s3_key_template using dateInZone, now, and randAlpha. The template
	// parser must support these functions or the collector will crash on
	// startup with "function dateInZone not defined".
	const datadogTemplate = `dt={{ dateInZone "20060102" (now) "UTC" }}/hour={{ dateInZone "15" (now) "UTC" }}/archive_{{ dateInZone "150405.0000" (now) "UTC" }}.{{ randAlpha 22 }}.json.gz`

	tmpl, err := parseLegacyTemplate(datadogTemplate)
	assert.NoError(t, err, "parseLegacyTemplate must accept dateInZone/now/randAlpha")
	assert.NotNil(t, tmpl)

	// Execute with real data to verify the functions produce valid output.
	key := buildLegacyTemplateKey("", tmpl, time.Date(2025, 9, 4, 18, 30, 45, 0, time.UTC))
	assert.Regexp(t, `^dt=\d{8}/hour=\d{2}/archive_\d{6}\.\d{4}\.[a-zA-Z]{22}\.json\.gz$`, key)
}

func TestParseLegacyTemplate_SprigFunctionsAvailable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		template string
		pattern  string
	}{
		{
			name:     "dateInZone formats time in UTC",
			template: `{{ dateInZone "2006-01-02" (now) "UTC" }}`,
			pattern:  `^\d{4}-\d{2}-\d{2}$`,
		},
		{
			name:     "randAlpha generates alphabetic string",
			template: `{{ randAlpha 10 }}`,
			pattern:  `^[a-zA-Z]{10}$`,
		},
		{
			name:     "now returns current time",
			template: `{{ dateInZone "15" (now) "UTC" }}`,
			pattern:  `^\d{2}$`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tmpl, err := parseLegacyTemplate(tc.template)
			assert.NoError(t, err)

			key := buildLegacyTemplateKey("", tmpl, time.Now())
			assert.Regexp(t, tc.pattern, key)
		})
	}
}

func TestValidateLegacyTemplate_DatadogKeyTemplate(t *testing.T) {
	t.Parallel()

	// Regression: config validation must not reject the datadog template.
	const datadogTemplate = `dt={{ dateInZone "20060102" (now) "UTC" }}/hour={{ dateInZone "15" (now) "UTC" }}/archive_{{ dateInZone "150405.0000" (now) "UTC" }}.{{ randAlpha 22 }}.json.gz`

	tmpl, err := ParseLegacyTemplateForValidation(datadogTemplate)
	assert.NoError(t, err)
	assert.NoError(t, ValidateLegacyTemplateForValidation(tmpl))
}

// SAW-7554: the legacy-template mode must honor the configured s3_prefix as
// a runtime invariant, symmetric with default mode (which always joins the
// prefix via bucketKeyPrefix). Before this fix, a template that omitted
// {{.Prefix}} would silently drop the prefix on the floor, causing every
// caller (e.g. the collectors-service generator) to bear responsibility for
// remembering to reference .Prefix — see the bug report for the production
// impact (~486 active collectors writing to bucket root).

func TestBuildKey_LegacyTemplate_AutoPrependsPrefix_WhenTemplateOmitsPrefix(t *testing.T) {
	t.Parallel()

	// The exact "datadog archive" template emitted by collectors-service:
	// authored before {{.Prefix}} was a thing, never referenced it.
	const tmpl = `dt={{ dateInZone "20060102" (now) "UTC" }}/hour={{ dateInZone "15" (now) "UTC" }}/archive_{{ dateInZone "150405.0000" (now) "UTC" }}.{{ randAlpha 22 }}.json.gz`

	parsed, err := parseLegacyTemplate(tmpl)
	assert.NoError(t, err)

	key := buildLegacyTemplateKey(
		"/datadog/hosted/logs",
		parsed,
		time.Date(2026, 5, 14, 12, 30, 0, 0, time.UTC),
	)

	// Prefix must be present at the start. The template uses `(now)`, not
	// the ts arg, so the date/hour/seconds reflect wall-clock — assert
	// only the invariant pieces.
	assert.Regexp(t, `^/datadog/hosted/logs/dt=\d{8}/hour=\d{2}/archive_\d+\.\d+\.[A-Za-z]{22}\.json\.gz$`, key)
}

func TestBuildKey_LegacyTemplate_PreservesExistingPrefixReference(t *testing.T) {
	t.Parallel()

	// A template that already references {{.Prefix}} must NOT have the
	// prefix prepended a second time — back-compat for callers that
	// learned the original rule.
	parsed, err := parseLegacyTemplate(`{{.Prefix}}/{{.Date}}/{{.UUID}}.json.gz`)
	assert.NoError(t, err)

	key := buildLegacyTemplateKey(
		"/datadog/hosted/logs",
		parsed,
		time.Date(2026, 5, 14, 12, 30, 0, 0, time.UTC),
	)

	assert.Regexp(t, `^/datadog/hosted/logs/2026/05/14/.+\.json\.gz$`, key)
}

func TestBuildKey_LegacyTemplate_EmptyPrefix_NoLeadingSlash(t *testing.T) {
	t.Parallel()

	// With no prefix configured, the rendered key must NOT gain a leading
	// slash — preserves today's behavior for prefix-less destinations
	// (which currently render starting with "dt=...").
	parsed, err := parseLegacyTemplate(`dt=20260514/archive.json.gz`)
	assert.NoError(t, err)

	key := buildLegacyTemplateKey("", parsed, time.Date(2026, 5, 14, 12, 30, 0, 0, time.UTC))
	assert.Equal(t, "dt=20260514/archive.json.gz", key)
}

func TestTemplateReferencesPrefix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		tmpl string
		want bool
	}{
		{"plain prefix ref", `{{.Prefix}}/foo`, true},
		{"spaced prefix ref", `{{ .Prefix }}/foo`, true},
		{"prefix inside if", `{{if .Prefix}}{{.Prefix}}/{{end}}foo`, true},
		{"prefix inside with", `{{with .Prefix}}{{.}}/{{end}}foo`, true},
		{"no prefix ref", `dt=20260514/archive.gz`, false},
		{"date ref but no prefix ref", `{{.Date}}/{{.UUID}}.gz`, false},
		{
			"datadog template (the buggy one)",
			`dt={{ dateInZone "20060102" (now) "UTC" }}/hour={{ dateInZone "15" (now) "UTC" }}/archive_{{ dateInZone "150405.0000" (now) "UTC" }}.{{ randAlpha 22 }}.json.gz`,
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			parsed, err := parseLegacyTemplate(tc.tmpl)
			assert.NoError(t, err)
			assert.Equal(t, tc.want, templateReferencesPrefix(parsed))
		})
	}
}

func TestPartitionKeyInputsUniqueKey(t *testing.T) {
	t.Parallel()

	// This test to is to help validate that a unique key
	// is not repeated

	seen := make(map[string]struct{})
	for range 500 {
		uv := (&PartitionKeyBuilder{}).uniqueKey()
		_, ok := seen[uv]
		assert.False(t, ok, "Must not have repeated partition key %q", uv)
		seen[uv] = struct{}{}
	}

	// This test is to validate that the UUIDv7 unique key
	// is generated correctly, is unique, and is ordered by time.
	seen = make(map[string]struct{})
	lastKey := ""
	for range 500 {
		uv := (&PartitionKeyBuilder{UniqueKeyFunc: GenerateUUIDv7}).uniqueKey()
		_, ok := seen[uv]
		assert.False(t, ok, "Must not have repeated partition key %q", uv)
		seen[uv] = struct{}{}

		assert.Greater(t, uv, lastKey, "Must be greater than the last key %q", lastKey)
		lastKey = uv
	}

	for _, tc := range []struct {
		name   string
		inputs *PartitionKeyBuilder
		match  string
	}{
		{
			name: "default unique key",
			inputs: &PartitionKeyBuilder{
				FilePrefix: "collector-capture-",
				FileFormat: "metrics",
			},
			match: "collector-capture-_[0-9]+.metrics",
		},
		{
			name: "uuidv7 key",
			inputs: &PartitionKeyBuilder{
				FilePrefix:    "collector-capture-",
				FileFormat:    "metrics",
				UniqueKeyFunc: GenerateUUIDv7,
			},
			match: "collector-capture-_[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}.metrics",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Regexp(t, tc.match, tc.inputs.fileName(), "Must match the expected regex pattern")
		})
	}
}
