// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pmetricassert // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetricassert"

import (
	"maps"
	"os"
	"testing"

	"go.opentelemetry.io/collector/pdata/pmetric"
	"gopkg.in/yaml.v3"
)

type writeOptions struct {
	includeValues                  bool
	includeHistogramExplicitBounds bool
	attributeExists                map[string]struct{}
	attributeRegex                 map[string]string
}

// WriteOption configures the snapshot generation.
type WriteOption interface {
	apply(*writeOptions)
}

type includeValuesOption struct{}

func (includeValuesOption) apply(o *writeOptions) { o.includeValues = true }

// IncludeValues opts into asserting the exact values of number datapoints
// (gauge and sum metrics). When enabled, generated snapshots will include
// the 'value' field.
func IncludeValues() WriteOption {
	return includeValuesOption{}
}

type includeHistogramExplicitBoundsOption struct{}

func (includeHistogramExplicitBoundsOption) apply(o *writeOptions) {
	o.includeHistogramExplicitBounds = true
}

// IncludeHistogramExplicitBounds opts into asserting each histogram datapoint's
// exact explicit bounds without other histogram values.
func IncludeHistogramExplicitBounds() WriteOption {
	return includeHistogramExplicitBoundsOption{}
}

type attributeExistsOption []string

func (o attributeExistsOption) apply(opts *writeOptions) {
	if opts.attributeExists == nil {
		opts.attributeExists = make(map[string]struct{}, len(o))
	}
	for _, key := range o {
		opts.attributeExists[key] = struct{}{}
	}
}

// WithAttributeExists generates /exists matchers for the selected resource and
// datapoint attribute keys.
func WithAttributeExists(keys ...string) WriteOption {
	return attributeExistsOption(append([]string(nil), keys...))
}

type attributeRegexOption map[string]string

func (o attributeRegexOption) apply(opts *writeOptions) {
	if opts.attributeRegex == nil {
		opts.attributeRegex = make(map[string]string, len(o))
	}
	maps.Copy(opts.attributeRegex, o)
}

// WithAttributeRegex generates /regex matchers for the selected resource and
// datapoint attribute keys. Each encountered value must be a string that fully
// matches its pattern.
func WithAttributeRegex(patterns map[string]string) WriteOption {
	return attributeRegexOption(maps.Clone(patterns))
}

// WriteAssertionFile regenerates the default-strict assertion snapshot at path
// from actual. It is intended to be called manually during test authoring,
// analogous to golden.WriteMetrics, and removed before committing.
//
// By default, emitted snapshots capture identity fields only: resource
// attributes, scope name/version, metric name/type/unit/temporality/monotonic,
// and the set of datapoint attribute permutations. Values, timestamps, and
// exemplars are omitted.
//
// The input metrics must be semantically valid. WriteAssertionFile normalizes
// valid metrics for assertion readability; it does not validate producer
// output.
func WriteAssertionFile(tb testing.TB, path string, actual pmetric.Metrics, opts ...WriteOption) error {
	tb.Helper()
	var o writeOptions
	for _, opt := range opts {
		opt.apply(&o)
	}
	snap := normalize(actual)
	project(snap, o)
	if err := applyWriteAttributeMatchers(snap, o); err != nil {
		return err
	}
	return writeSnapshot(path, snap)
}

// project drops the datapoint fields the write options do not opt into. A
// snapshot captures every value it can represent, but an assertion file should
// only pin the volatile ones a test explicitly asks for.
func project(snap *snapshot, opts writeOptions) {
	if opts.includeValues {
		return
	}
	forEachDatapoint(snap, func(dp *datapointSnapshot) {
		kept := datapointSnapshot{Attributes: dp.Attributes}
		if opts.includeHistogramExplicitBounds {
			kept.ExplicitBounds = dp.ExplicitBounds
		}
		*dp = kept
	})
}

func writeSnapshot(path string, snap *snapshot) error {
	compactShorthand(snap)
	b, err := yaml.Marshal(snap)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

// compactShorthand is the inverse of expandShorthand: it drops an explicit
// single empty-attribute datapoint so the emitted YAML reads as "metric with
// no dimensioning attributes" rather than "metric with one empty datapoint".
func compactShorthand(snap *snapshot) {
	forEachMetric(snap, func(m *metricSnapshot) {
		if len(m.Datapoints) == 1 && isEmptyDatapointSnapshot(m.Datapoints[0]) {
			m.Datapoints = nil
		}
	})
}

func isEmptyDatapointSnapshot(dp datapointSnapshot) bool {
	return len(dp.Attributes) == 0 &&
		dp.IntValue == nil &&
		dp.DoubleValue == nil &&
		dp.Count == nil &&
		dp.Sum == nil &&
		dp.ExplicitBounds == nil &&
		len(dp.BucketCounts) == 0 &&
		dp.Min == nil &&
		dp.Max == nil
}

func forEachMetric(snap *snapshot, fn func(*metricSnapshot)) {
	for i := range snap.Resources {
		for j := range snap.Resources[i].Scopes {
			for k := range snap.Resources[i].Scopes[j].Metrics {
				fn(&snap.Resources[i].Scopes[j].Metrics[k])
			}
		}
	}
}

func forEachDatapoint(snap *snapshot, fn func(*datapointSnapshot)) {
	forEachMetric(snap, func(m *metricSnapshot) {
		for i := range m.Datapoints {
			fn(&m.Datapoints[i])
		}
	})
}
