package translator

// ====================
// ATTRIBUTE PREFIXES
// ====================
const (
	PREFIX_DATADOG      = "datadog"
	PREFIX_ERROR        = "error"
	PREFIX_USER         = "user"
	PREFIX_USR          = "usr"
	PREFIX_GEO          = "geo"
	PREFIX_DEVICE       = "device"
	PREFIX_MODEL        = "model"
	PREFIX_OS           = "os"
	PREFIX_SESSION      = "session"
	PREFIX_VIEW         = "view"
	PREFIX_INITIAL_VIEW = "initial_view"
	PREFIX_LAST_VIEW    = "last_view"
	PREFIX_RESOURCE     = "resource"
	PREFIX_ACTION       = "action"
	PREFIX_CLIENT       = "client"
	PREFIX_APPLICATION  = "application"
)

// ====================
// CORE ATTRIBUTES
// ====================
const (
	ATTR_ID           = "id"
	ATTR_NAME         = "name"
	ATTR_TYPE         = "type"
	ATTR_SERVICE      = "service"
	ATTR_DATE         = "date"
	ATTR_MESSAGE      = "message"
	ATTR_SOURCE       = "source"
	ATTR_STACK        = "stack"
	ATTR_DOMAIN       = "domain"
	ATTR_PROVIDER     = "provider"
	ATTR_IS_RUM       = "is_rum"
	ATTR_IS_ACTIVE    = "is_active"
	ATTR_REFERRER     = "referrer"
	ATTR_LOADING_TYPE = "loading_type"
)

// ====================
// NETWORK ATTRIBUTES
// ====================
const (
	ATTR_DURATION     = "duration"
	ATTR_LOADING_TIME = "loading_time"
	ATTR_TIME_SPENT   = "time_spent"
	ATTR_FIRST_BYTE   = "first_byte"
	ATTR_DOWNLOAD     = "download"
	ATTR_SSL          = "ssl"
	ATTR_DNS          = "dns"
	ATTR_REDIRECT     = "redirect"
	ATTR_SIZE         = "size"
	ATTR_STATUS_CODE  = "status_code"
	ATTR_METHOD       = "method"
)

// ==================
// URL ATTRIBUTES
// ==================
const (
	ATTR_URL            = "url"
	ATTR_URL_HOST       = "url_host"
	ATTR_URL_PATH       = "url_path"
	ATTR_URL_PATH_GROUP = "url_path_group"
	ATTR_URL_QUERY      = "url_query"
	ATTR_URL_SCHEME     = "url_scheme"
	ATTR_URL_HASH       = "url_hash"
)

// ==================
// UTM ATTRIBUTES
// ==================
const (
	ATTR_UTM_SOURCE   = "utm_source"
	ATTR_UTM_MEDIUM   = "utm_medium"
	ATTR_UTM_CAMPAIGN = "utm_campaign"
	ATTR_UTM_CONTENT  = "utm_content"
	ATTR_UTM_TERM     = "utm_term"
)

// =========================
// DEVICE & OS ATTRIBUTES
// =========================
const (
	ATTR_BRAND         = "brand"
	ATTR_MODEL         = "model"
	ATTR_VERSION       = "version"
	ATTR_VERSION_MAJOR = "version_major"
	ATTR_MANUFACTURER  = "manufacturer"
	ATTR_IDENTIFIER    = "identifier"
)

// =======================
// GEOGRAPHIC ATTRIBUTES
// =======================
const (
	ATTR_COUNTRY             = "country"
	ATTR_COUNTRY_SUBDIVISION = "country_subdivision"
	ATTR_CONTINENT           = "continent"
	ATTR_COUNTRY_ISO_CODE    = "country_iso_code"
	ATTR_ISO_CODE            = "iso_code"
	ATTR_CITY                = "city"
	ATTR_LOCALITY            = "locality"
	ATTR_CONTINENT_CODE      = "continent_code"
)

// ==================
// WEB ATTRIBUTES
// ==================
const (
	ATTR_LARGEST_CONTENTFUL_PAINT  = "largest_contentful_paint"
	ATTR_FIRST_INPUT_DELAY         = "first_input_delay"
	ATTR_INTERACTION_TO_NEXT_PAINT = "interaction_to_next_paint"
	ATTR_CUMULATIVE_LAYOUT_SHIFT   = "cumulative_layout_shift"
	ATTR_FIRST_CONTENTFUL_PAINT    = "first_contentful_paint"
	ATTR_DOM_INTERACTIVE           = "dom_interactive"
	ATTR_DOM_CONTENT_LOADED        = "dom_content_loaded"
	ATTR_DOM_COMPLETE              = "dom_complete"
	ATTR_LOAD_EVENT                = "load_event"
)

// ================================
// WEB TARGET SELECTOR ATTRIBUTES
// ================================
const (
	ATTR_LARGEST_CONTENTFUL_PAINT_TARGET_SELECTOR  = "largest_contentful_paint_target_selector"
	ATTR_FIRST_INPUT_DELAY_TARGET_SELECTOR         = "first_input_delay_target_selector"
	ATTR_INTERACTION_TO_NEXT_PAINT_TARGET_SELECTOR = "interaction_to_next_paint_target_selector"
	ATTR_CUMULATIVE_LAYOUT_SHIFT_TARGET_SELECTOR   = "cumulative_layout_shift_target_selector"
)

// =============================
// SESSION & VIEW ATTRIBUTES
// =============================
const (
	ATTR_COUNT       = "count"
	ATTR_LONG_TASK   = "long_task"
	ATTR_FRUSTRATION = "frustration"
)

// ==============================
// USER INTERACTION ATTRIBUTES
// ==============================
const (
	ATTR_TARGET      = "target"
	ATTR_EMAIL       = "email"
	ATTR_ADDRESS     = "address"
	ATTR_IP          = "ip"
	ATTR_DEAD_CLICK  = "dead_click"
	ATTR_RAGE_CLICK  = "rage_click"
	ATTR_ERROR_CLICK = "error_click"
)
