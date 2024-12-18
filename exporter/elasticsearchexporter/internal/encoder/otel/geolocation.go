// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otel // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/otel"

import (
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

// mergeGeolocation mutates attributes map to merge all `geo.location.{lon,lat}`,
// and namespaced `*.geo.location.{lon,lat}` to unnamespaced and namespaced `geo.location`.
// This is to match the geo_point type in Elasticsearch.
func mergeGeolocation(attributes pcommon.Map) {
	const (
		lonKey    = "geo.location.lon"
		latKey    = "geo.location.lat"
		mergedKey = "geo.location"
	)
	// Prefix is the attribute name without lonKey or latKey suffix
	// e.g. prefix of "foo.bar.geo.location.lon" is "foo.bar.", prefix of "geo.location.lon" is "".
	prefixToGeo := make(map[string]struct {
		lon, lat       float64
		lonSet, latSet bool
	})
	setLon := func(prefix string, v float64) {
		g := prefixToGeo[prefix]
		g.lon = v
		g.lonSet = true
		prefixToGeo[prefix] = g
	}
	setLat := func(prefix string, v float64) {
		g := prefixToGeo[prefix]
		g.lat = v
		g.latSet = true
		prefixToGeo[prefix] = g
	}
	attributes.RemoveIf(func(key string, val pcommon.Value) bool {
		if val.Type() != pcommon.ValueTypeDouble {
			return false
		}

		if key == lonKey {
			setLon("", val.Double())
			return true
		} else if key == latKey {
			setLat("", val.Double())
			return true
		} else if namespace, found := strings.CutSuffix(key, "."+lonKey); found {
			prefix := namespace + "."
			setLon(prefix, val.Double())
			return true
		} else if namespace, found := strings.CutSuffix(key, "."+latKey); found {
			prefix := namespace + "."
			setLat(prefix, val.Double())
			return true
		}
		return false
	})

	for prefix, geo := range prefixToGeo {
		if geo.lonSet && geo.latSet {
			key := prefix + mergedKey
			// Geopoint expressed as an array with the format: [lon, lat]
			s := attributes.PutEmptySlice(key)
			s.EnsureCapacity(2)
			s.AppendEmpty().SetDouble(geo.lon)
			s.AppendEmpty().SetDouble(geo.lat)
			continue
		}

		// Place the attributes back if lon and lat are not present together
		if geo.lonSet {
			key := prefix + lonKey
			attributes.PutDouble(key, geo.lon)
		}
		if geo.latSet {
			key := prefix + latKey
			attributes.PutDouble(key, geo.lat)
		}
	}
}
