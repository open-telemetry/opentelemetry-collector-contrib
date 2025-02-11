// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package conventions // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor/internal/convention"

// TODO: replace for semconv once https://github.com/open-telemetry/semantic-conventions/issues/1033 is closed.
const (
	// AttributeGeoCityName represents the attribute name for the city name in geographical data.
	AttributeGeoCityName = "geo.city_name"

	// AttributeGeoPostalCode represents the attribute name for the city postal code.
	AttributeGeoPostalCode = "geo.postal_code"

	// AttributeGeoCountryName represents the attribute name for the country name in geographical data.
	AttributeGeoCountryName = "geo.country_name"

	// AttributeGeoCountryIsoCode represents the attribute name for the Two-letter ISO Country Code.
	AttributeGeoCountryIsoCode = "geo.country_iso_code"

	// AttributeGeoContinentName represents the attribute name for the continent name in geographical data.
	AttributeGeoContinentName = "geo.continent_name"

	// AttributeGeoContinentIsoCode represents the attribute name for the Two-letter Continent Code.
	AttributeGeoContinentCode = "geo.continent_code"

	// AttributeGeoRegionName represents the attribute name for the region name in geographical data.
	AttributeGeoRegionName = "geo.region_name"

	// AttributeGeoRegionIsoCode represents the attribute name for the Two-letter ISO Region Code.
	AttributeGeoRegionIsoCode = "geo.region_iso_code"

	// AttributeGeoTimezone represents the attribute name for the timezone.
	AttributeGeoTimezone = "geo.timezone"

	// AttributeGeoLocationLat represents the attribute name for the latitude.
	AttributeGeoLocationLat = "geo.location.lat"

	// AttributeGeoLocationLon represents the attribute name for the longitude.
	AttributeGeoLocationLon = "geo.location.lon"
)
