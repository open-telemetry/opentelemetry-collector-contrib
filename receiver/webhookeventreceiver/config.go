// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package webhookeventreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/webhookeventreceiver"

import (
	"errors"
	"regexp"
	"time"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.uber.org/multierr"
)

var (
	errMissingEndpointFromConfig   = errors.New("missing receiver server endpoint from config")
	errReadTimeoutExceedsMaxValue  = errors.New("the duration specified for read_timeout exceeds the maximum allowed value of 10s")
	errWriteTimeoutExceedsMaxValue = errors.New("the duration specified for write_timeout exceeds the maximum allowed value of 10s")
	errRequiredHeader              = errors.New("both key and value are required to assign a required_header")
	errHeaderAttributeRegexCompile = errors.New("regex for header_attribute_regex failed to compile")
	errHMACMissingSecret           = errors.New("hmac_signature.secret is required when hmac_signature is configured")
	errHMACMissingHeader           = errors.New("hmac_signature.header is required when hmac_signature is configured")
	errHMACMissingPrefix           = errors.New("hmac_signature.prefix is required when hmac_signature is configured")
)

// Config defines configuration for the Generic Webhook receiver.
type Config struct {
	ServerConfig               confighttp.ServerConfig `mapstructure:",squash"`                       // squash ensures fields are correctly decoded in embedded struct
	ReadTimeout                string                  `mapstructure:"read_timeout"`                  // wait time for reading request headers in ms. Default is 500ms.
	WriteTimeout               string                  `mapstructure:"write_timeout"`                 // wait time for writing request response in ms. Default is 500ms.
	Path                       string                  `mapstructure:"path"`                          // path for data collection. Default is /events
	HealthPath                 string                  `mapstructure:"health_path"`                   // path for health check api. Default is /health_check
	RequiredHeader             RequiredHeader          `mapstructure:"required_header"`               // optional setting to set a required header for all requests to have
	SplitLogsAtNewLine         bool                    `mapstructure:"split_logs_at_newline"`         // optional setting to split logs into multiple log records
	SplitLogsAtJSONBoundary    bool                    `mapstructure:"split_logs_at_json_boundary"`   // optional setting to split logs at JSON object boundaries
	ConvertHeadersToAttributes bool                    `mapstructure:"convert_headers_to_attributes"` // optional to convert all headers to attributes
	HeaderAttributeRegex       string                  `mapstructure:"header_attribute_regex"`        // optional to convert headers matching a regex to log attributes
	HMACSignature              HMACSignature           `mapstructure:"hmac_signature"`                // optional HMAC hex digest signature verification
}

type RequiredHeader struct {
	Key   string `mapstructure:"key"`
	Value string `mapstructure:"value"`
}

// HMACSignature defines configuration for HMAC hex digest signature verification.
// This is compatible with webhook signature schemes used by GitHub (X-Hub-Signature-256: sha256=<hex>)
// and Fingerprint (fpjs-event-signature: v1=<hex>).
type HMACSignature struct {
	// Secret is the shared secret used to compute the HMAC.
	Secret configopaque.String `mapstructure:"secret"`
	// Header is the HTTP header name containing the signature (e.g. "X-Hub-Signature-256").
	Header string `mapstructure:"header"`
	// Prefix is the prefix before the hex digest in the header value (e.g. "sha256=" or "v1=").
	Prefix string `mapstructure:"prefix"`
}

func (cfg *Config) Validate() error {
	var errs error

	maxReadWriteTimeout, _ := time.ParseDuration("10s")

	if cfg.ServerConfig.NetAddr.Endpoint == "" {
		errs = multierr.Append(errs, errMissingEndpointFromConfig)
	}

	// If a user defines a custom read/write timeout there is a maximum value
	// of 10s imposed here.
	if cfg.ReadTimeout != "" {
		readTimeout, err := time.ParseDuration(cfg.ReadTimeout)
		if err != nil {
			errs = multierr.Append(errs, err)
		}

		if readTimeout > maxReadWriteTimeout {
			errs = multierr.Append(errs, errReadTimeoutExceedsMaxValue)
		}
	}

	if cfg.WriteTimeout != "" {
		writeTimeout, err := time.ParseDuration(cfg.WriteTimeout)
		if err != nil {
			errs = multierr.Append(errs, err)
		}

		if writeTimeout > maxReadWriteTimeout {
			errs = multierr.Append(errs, errWriteTimeoutExceedsMaxValue)
		}
	}

	// Set default MaxRequestBodySize if not configured
	if cfg.ServerConfig.MaxRequestBodySize == 0 {
		cfg.ServerConfig.MaxRequestBodySize = int64(20 * 1024 * 1024) // 20MiB
		// to match default value http://github.com/open-telemetry/opentelemetry-collector/blob/release/v0.139.x/config/confighttp/server.go#L31
	}

	if (cfg.RequiredHeader.Key != "" && cfg.RequiredHeader.Value == "") || (cfg.RequiredHeader.Value != "" && cfg.RequiredHeader.Key == "") {
		errs = multierr.Append(errs, errRequiredHeader)
	}

	if cfg.SplitLogsAtNewLine && cfg.SplitLogsAtJSONBoundary {
		errs = multierr.Append(errs, errors.New("split_logs_at_new_line and split_logs_at_json_boundary cannot be enabled at the same time"))
	}

	if cfg.HMACSignature.Secret != "" || cfg.HMACSignature.Header != "" || cfg.HMACSignature.Prefix != "" {
		if cfg.HMACSignature.Secret == "" {
			errs = multierr.Append(errs, errHMACMissingSecret)
		}
		if cfg.HMACSignature.Header == "" {
			errs = multierr.Append(errs, errHMACMissingHeader)
		}
		if cfg.HMACSignature.Prefix == "" {
			errs = multierr.Append(errs, errHMACMissingPrefix)
		}
	}

	if cfg.HeaderAttributeRegex != "" {
		_, err := regexp.Compile(cfg.HeaderAttributeRegex)
		if err != nil {
			errs = multierr.Append(errs, errHeaderAttributeRegexCompile)
			errs = multierr.Append(errs, err)
		}
	}

	return errs
}

func (cfg *Config) ShouldSplitLogsAtJSONBoundary() bool {
	return cfg.SplitLogsAtJSONBoundary
}
