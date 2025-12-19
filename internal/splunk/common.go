// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunk // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"

// Constants for Splunk components.
const (
	SFxAccessTokenHeader       = "X-Sf-Token"                       // #nosec
	SFxAccessTokenLabel        = "com.splunk.signalfx.access_token" // #nosec
	SFxEventCategoryKey        = "com.splunk.signalfx.event_category"
	SFxEventPropertiesKey      = "com.splunk.signalfx.event_properties"
	SFxEventType               = "com.splunk.signalfx.event_type"
	DefaultSourceTypeLabel     = "com.splunk.sourcetype"
	DefaultSourceLabel         = "com.splunk.source"
	DefaultIndexLabel          = "com.splunk.index"
	DefaultNameLabel           = "otel.log.name"
	DefaultSeverityTextLabel   = "otel.log.severity.text"
	DefaultSeverityNumberLabel = "otel.log.severity.number"
	HECTokenHeader             = "Splunk"
	HTTPSplunkChannelHeader    = "X-Splunk-Request-Channel"

	HecTokenLabel     = "com.splunk.hec.access_token" // #nosec
	DefaultRawPath    = "/services/collector/raw"
	DefaultHealthPath = "/services/collector/health"
	DefaultAckPath    = "/services/collector/ack"
)

// AccessTokenPassthroughConfig configures passing through access tokens.
type AccessTokenPassthroughConfig struct {
	// AccessTokenPassthrough indicates whether to associate datapoints with an organization access token received in request.
	AccessTokenPassthrough bool `mapstructure:"access_token_passthrough"`
}

type AckRequest struct {
	Acks []uint64 `json:"acks"`
}
