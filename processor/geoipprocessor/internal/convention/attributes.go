// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package conventions // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor/internal/convention"

import conventions "go.opentelemetry.io/otel/semconv/v1.38.0"

const (
	// AttributeGeoCityName represents the attribute name for the city name in geographical data.
	AttributeGeoCityName = "geo.city_name"

	// AttributeGeoPostalCode represents the attribute name for the city postal code.
	AttributeGeoPostalCode = string(conventions.GeoPostalCodeKey)

	// AttributeGeoCountryName represents the attribute name for the country name in geographical data.
	AttributeGeoCountryName = "geo.country_name"

	// AttributeGeoCountryIsoCode represents the attribute name for the Two-letter ISO Country Code.
	AttributeGeoCountryIsoCode = string(conventions.GeoCountryISOCodeKey)

	// AttributeGeoContinentName represents the attribute name for the continent name in geographical data.
	AttributeGeoContinentName = "geo.continent_name"

	// AttributeGeoContinentIsoCode represents the attribute name for the Two-letter Continent Code.
	AttributeGeoContinentCode = string(conventions.GeoContinentCodeKey)

	// AttributeGeoRegionName represents the attribute name for the region name in geographical data.
	AttributeGeoRegionName = "geo.region_name"

	// AttributeGeoRegionIsoCode represents the attribute name for the Two-letter ISO Region Code.
	AttributeGeoRegionIsoCode = string(conventions.GeoRegionISOCodeKey)

	// AttributeGeoTimezone represents the attribute name for the timezone.
	AttributeGeoTimezone = "geo.timezone"

	// AttributeGeoLocationLat represents the attribute name for the latitude.
	AttributeGeoLocationLat = string(conventions.GeoLocationLatKey)

	// AttributeGeoLocationLon represents the attribute name for the longitude.
	AttributeGeoLocationLon = string(conventions.GeoLocationLonKey)
)
