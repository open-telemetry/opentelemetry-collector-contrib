// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pmetricassert // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetricassert"

import (
	"maps"
	"testing"

	"go.opentelemetry.io/collector/pdata/pmetric"
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
	doc := normalize(actual, o)
	if err := applyWriteAttributeMatchers(doc, o); err != nil {
		return err
	}
	return writeDocument(path, doc)
}
