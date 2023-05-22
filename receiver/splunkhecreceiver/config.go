// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkhecreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkhecreceiver"

import (
	"go.opentelemetry.io/collector/config/confighttp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

// Config defines configuration for the Splunk HEC receiver.
type Config struct {
	confighttp.HTTPServerSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct

	splunk.AccessTokenPassthroughConfig `mapstructure:",squash"`
	// RawPath for raw data collection, default is '/services/collector/raw'
	RawPath string `mapstructure:"raw_path"`
	// HealthPath for health API, default is '/services/collector/health'
	HealthPath string `mapstructure:"health_path"`
	// HecToOtelAttrs creates a mapping from HEC metadata to attributes.
	HecToOtelAttrs splunk.HecToOtelAttrs `mapstructure:"hec_metadata_to_otel_attrs"`
}
