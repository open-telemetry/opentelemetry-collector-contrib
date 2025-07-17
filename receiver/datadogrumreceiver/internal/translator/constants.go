// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogrumreceiver/internal/translator"

// =========================
// GENERAL CONSTANTS
// =========================
const (
	InstrumentationScopeName = "datadog.rum-browser-sdk"
)

// ====================
// ATTRIBUTE PREFIXES
// ====================
const (
	PrefixDatadog     = "datadog"
	PrefixError       = "error"
	PrefixUser        = "user"
	PrefixUsr         = "usr"
	PrefixGeo         = "geo"
	PrefixDevice      = "device"
	PrefixModel       = "model"
	PrefixOS          = "os"
	PrefixSession     = "session"
	PrefixView        = "view"
	PrefixInitialView = "initial_view"
	PrefixLastView    = "last_view"
	PrefixResource    = "resource"
	PrefixAction      = "action"
	PrefixClient      = "client"
	PrefixApplication = "application"
	PrefixContainer   = "container"
)

// ====================
// CORE ATTRIBUTES
// ====================
const (
	AttrID          = "id"
	AttrName        = "name"
	AttrType        = "type"
	AttrService     = "service"
	AttrDate        = "date"
	AttrMessage     = "message"
	AttrSource      = "source"
	AttrStack       = "stack"
	AttrDomain      = "domain"
	AttrProvider    = "provider"
	AttrIsActive    = "is_active"
	AttrReferrer    = "referrer"
	AttrLoadingType = "loading_type"
)

// ====================
// NETWORK ATTRIBUTES
// ====================
const (
	AttrDuration    = "duration"
	AttrLoadingTime = "loading_time"
	AttrTimeSpent   = "time_spent"
	AttrFirstByte   = "first_byte"
	AttrDownload    = "download"
	AttrSSL         = "ssl"
	AttrDNS         = "dns"
	AttrRedirect    = "redirect"
	AttrSize        = "size"
	AttrStatusCode  = "status_code"
	AttrMethod      = "method"
)

// ==================
// URL ATTRIBUTES
// ==================
const (
	AttrURL          = "url"
	AttrURLHost      = "url_host"
	AttrURLPath      = "url_path"
	AttrURLPathGroup = "url_path_group"
	AttrURLQuery     = "url_query"
	AttrURLScheme    = "url_scheme"
	AttrURLHash      = "url_hash"
)

// ==================
// UTM ATTRIBUTES
// ==================
const (
	AttrUTMSource   = "utm_source"
	AttrUTMMedium   = "utm_medium"
	AttrUTMCampaign = "utm_campaign"
	AttrUTMContent  = "utm_content"
	AttrUTMTerm     = "utm_term"
)

// =========================
// DEVICE & OS ATTRIBUTES
// =========================
const (
	AttrBrand        = "brand"
	AttrModel        = "model"
	AttrVersion      = "version"
	AttrVersionMajor = "version_major"
	AttrManufacturer = "manufacturer"
	AttrIdentifier   = "identifier"
)

// =======================
// GEOGRAPHIC ATTRIBUTES
// =======================
const (
	AttrCountry            = "country"
	AttrCountrySubdivision = "country_subdivision"
	AttrContinent          = "continent"
	AttrCountryISOCode     = "country_iso_code"
	AttrISOCode            = "iso_code"
	AttrCity               = "city"
	AttrLocality           = "locality"
	AttrContinentCode      = "continent_code"
)

// ==================
// WEB ATTRIBUTES
// ==================
const (
	AttrLargestContentfulPaint = "largest_contentful_paint"
	AttrFirstInputDelay        = "first_input_delay"
	AttrInteractionToNextPaint = "interaction_to_next_paint"
	AttrCumulativeLayoutShift  = "cumulative_layout_shift"
	AttrFirstContentfulPaint   = "first_contentful_paint"
	AttrDOMInteractive         = "dom_interactive"
	AttrDOMContentLoaded       = "dom_content_loaded"
	AttrDOMComplete            = "dom_complete"
	AttrLoadEvent              = "load_event"
)

// ================================
// WEB TARGET SELECTOR ATTRIBUTES
// ================================
const (
	AttrLargestContentfulPaintTargetSelector = "largest_contentful_paint_target_selector"
	AttrFirstInputDelayTargetSelector        = "first_input_delay_target_selector"
	AttrInteractionToNextPaintTargetSelector = "interaction_to_next_paint_target_selector"
	AttrCumulativeLayoutShiftTargetSelector  = "cumulative_layout_shift_target_selector"
)

// =============================
// SESSION & VIEW ATTRIBUTES
// =============================
const (
	AttrCount       = "count"
	AttrLongTask    = "long_task"
	AttrFrustration = "frustration"
)

// ==============================
// USER INTERACTION ATTRIBUTES
// ==============================
const (
	AttrTarget     = "target"
	AttrEmail      = "email"
	AttrAddress    = "address"
	AttrIP         = "ip"
	AttrDeadClick  = "dead_click"
	AttrRageClick  = "rage_click"
	AttrErrorClick = "error_click"
)
