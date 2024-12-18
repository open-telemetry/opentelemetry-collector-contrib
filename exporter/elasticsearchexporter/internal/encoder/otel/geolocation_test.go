// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestMergeGeolocation(t *testing.T) {
	attributes := map[string]any{
		"geo.location.lon":          1.1,
		"geo.location.lat":          2.2,
		"foo.bar.geo.location.lon":  3.3,
		"foo.bar.geo.location.lat":  4.4,
		"a.geo.location.lon":        5.5,
		"b.geo.location.lat":        6.6,
		"unrelatedgeo.location.lon": 7.7,
		"unrelatedgeo.location.lat": 8.8,
		"d":                         9.9,
		"e.geo.location.lon":        "foo",
		"e.geo.location.lat":        "bar",
	}
	wantAttributes := map[string]any{
		"geo.location":              []any{1.1, 2.2},
		"foo.bar.geo.location":      []any{3.3, 4.4},
		"a.geo.location.lon":        5.5,
		"b.geo.location.lat":        6.6,
		"unrelatedgeo.location.lon": 7.7,
		"unrelatedgeo.location.lat": 8.8,
		"d":                         9.9,
		"e.geo.location.lon":        "foo",
		"e.geo.location.lat":        "bar",
	}
	input := pcommon.NewMap()
	err := input.FromRaw(attributes)
	require.NoError(t, err)
	mergeGeolocation(input)
	after := input.AsRaw()
	assert.Equal(t, wantAttributes, after)
}
