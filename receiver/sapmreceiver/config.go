// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sapmreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sapmreceiver"

import (
	"go.opentelemetry.io/collector/config/confighttp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

// Config defines configuration for SAPM receiver.
type Config struct {
	confighttp.ServerConfig `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct

	// Deprecated: `access_token_passthrough` is deprecated.
	// Please enable include_metadata in the receiver and add the following config to the batch processor:
	// batch:
	// 	 metadata_keys: [X-Sf-Token]
	splunk.AccessTokenPassthroughConfig `mapstructure:",squash"`
}
