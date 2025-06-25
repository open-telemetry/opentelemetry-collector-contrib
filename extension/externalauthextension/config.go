package externalauthextension

import (
	"errors"
	"net/http"
	"net/url"
	"time"

	"go.opentelemetry.io/collector/component"
)

const (
	defaultRefreshInterval   = "1h"
	defaultHeader            = "Authorization"
	defaultExpectedCode      = 200
	defaultScheme            = "Bearer"
	defaultMethod            = "POST"
	defaultHttpClientTimeout = 10 * time.Second
	defaultTelemetryType     = "traces"
	defaultTokenFormat       = "raw"
)

type Config struct {
	// Endpoint specifies the endpoint to authenticate against. Required
	Endpoint string `mapstructure:"endpoint"`
	// HeaderEndpointMapping allows mapping a single header value to different endpoints
	// Format: { header: "header_name", values: {"header_value": "endpoint_url"} }
	// Example: { header: "Destination", values: {"stage": "https://stage-auth.example.com", "prod": "https://prod-auth.example.com"} }
	HeaderEndpointMapping *HeaderMapping `mapstructure:"header_endpoint_mapping,omitempty"`
	//RefreshInterval specifies the time that a newly checked token will be valid for. Defaults to "1h"
	RefreshInterval string `mapstructure:"refresh_interval,omitempty"`
	// Header specifies the header used in the request. Defaults to "Authorization"
	Header string `mapstructure:"header,omitempty"`
	// ExpectedCodes is a list of expected HTTP codes. Defaults to [200]
	ExpectedCodes []int `mapstructure:"expected_codes,omitempty"`
	// Scheme specifies the auth-scheme for the token. Defaults to "Bearer"
	Scheme string `mapstructure:"scheme,omitempty"`
	// Method specifies the HTTP method used in the request. Defaults to "POST"
	Method string `mapstructure:"method,omitempty"`
	// TokenFormat specifies the format of the token. Options: "raw", "basic_auth". Defaults to "raw"
	TokenFormat string `mapstructure:"token_format,omitempty"`
	// HTTPClientTimeout specifies the timeout for the HTTP client. Defaults to "10s"
	HTTPClientTimeout time.Duration `mapstructure:"http_client_timeout,omitempty"`
	// TelemetryType specifies the telemetry type for this endpoint. Options: "traces", "metrics", "logs". Defaults to "traces"
	TelemetryType string `mapstructure:"telemetry_type,omitempty"`
}

// HeaderMapping represents a single header-to-endpoint mapping with ordered values
type HeaderMapping struct {
	Header string            `mapstructure:"header"`
	Values map[string]string `mapstructure:"values"`
}

var (
	_                         component.Config = (*Config)(nil)
	errNoEndpointProvided                      = errors.New("no endpoint to authenticate against provided")
	errInvalidInterval                         = errors.New("invalid refresh interval")
	errInvalidEndpoint                         = errors.New("invalid remote endpoint")
	errInvalidHttpCode                         = errors.New("code provided is not a valid HTTP code")
	errInvalidTelemetryType                    = errors.New("telemetry_type must be one of: traces, metrics, logs")
	errInvalidTokenFormat                      = errors.New("token_format must be one of: raw, basic_auth")
	errInvalidEndpointMapping                  = errors.New("invalid endpoint mapping configuration")
)

// Validate checks if the extension configuration is valid
func (cfg *Config) Validate() error {
	// If endpoint mapping is provided, validate it
	if cfg.HeaderEndpointMapping != nil {
		if err := cfg.validateEndpointMapping(); err != nil {
			return err
		}
	} else {
		// Fall back to single endpoint validation
		if cfg.Endpoint == "" {
			return errNoEndpointProvided
		} else {
			_, err := url.ParseRequestURI(cfg.Endpoint)
			if err != nil {
				return errInvalidEndpoint
			}
		}
	}

	if cfg.RefreshInterval == "" {
		cfg.RefreshInterval = defaultRefreshInterval
	} else {
		_, err := time.ParseDuration(cfg.RefreshInterval)
		if err != nil {
			return errInvalidInterval
		}
	}
	if cfg.Header == "" {
		cfg.Header = defaultHeader
	}
	if cfg.HTTPClientTimeout == 0 {
		cfg.HTTPClientTimeout = defaultHttpClientTimeout
	}
	if cfg.ExpectedCodes == nil {
		cfg.ExpectedCodes = []int{defaultExpectedCode}
	} else {
		for _, code := range cfg.ExpectedCodes {
			if http.StatusText(code) == "" {
				return errInvalidHttpCode
			}
		}
	}
	if cfg.Scheme == "" {
		cfg.Scheme = defaultScheme
	}
	if cfg.Method == "" {
		cfg.Method = defaultMethod
	}
	if cfg.TelemetryType == "" {
		cfg.TelemetryType = defaultTelemetryType
	} else {
		validTypes := map[string]bool{
			"traces":  true,
			"metrics": true,
			"logs":    true,
		}
		if !validTypes[cfg.TelemetryType] {
			return errInvalidTelemetryType
		}
	}
	if cfg.TokenFormat == "" {
		cfg.TokenFormat = defaultTokenFormat
	} else {
		validFormats := map[string]bool{
			"raw":        true,
			"basic_auth": true,
		}
		if !validFormats[cfg.TokenFormat] {
			return errInvalidTokenFormat
		}
	}
	return nil
}

// validateEndpointMapping validates the endpoint mapping configuration
func (cfg *Config) validateEndpointMapping() error {
	if cfg.HeaderEndpointMapping == nil {
		return errInvalidEndpointMapping
	}

	if cfg.HeaderEndpointMapping.Header == "" {
		return errInvalidEndpointMapping
	}
	if len(cfg.HeaderEndpointMapping.Values) == 0 {
		return errInvalidEndpointMapping
	}
	for headerValue, endpoint := range cfg.HeaderEndpointMapping.Values {
		if headerValue == "" || endpoint == "" {
			return errInvalidEndpointMapping
		}
		_, err := url.ParseRequestURI(endpoint)
		if err != nil {
			return errInvalidEndpoint
		}
	}
	return nil
}
