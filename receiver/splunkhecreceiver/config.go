// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkhecreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkhecreceiver"

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
	translator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/splunk"
)

type SplittingStrategy string

const (
	SplittingStrategyLine SplittingStrategy = "line"
	SplittingStrategyNone SplittingStrategy = "none"
)

// Config defines configuration for the Splunk HEC receiver.
type Config struct {
	confighttp.ServerConfig `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct

	splunk.AccessTokenPassthroughConfig `mapstructure:",squash"`

	Ack `mapstructure:"ack"`

	// RawPath for raw data collection, default is '/services/collector/raw'
	RawPath string `mapstructure:"raw_path"`
	// Splitting defines the splitting strategy used by the receiver when ingesting raw events. Can be set to "line" or "none". Default is "line".
	Splitting SplittingStrategy `mapstructure:"splitting"`
	// HealthPath for health API, default is '/services/collector/health'
	HealthPath string `mapstructure:"health_path"`
	// HecToOtelAttrs creates a mapping from HEC metadata to attributes.
	HecToOtelAttrs translator.HecToOtelAttrs `mapstructure:"hec_metadata_to_otel_attrs"`
}

// Ack defines configuration for the ACK functionality of the HEC receiver
type Ack struct {
	// Extension defines the extension to use for acking of events. Without specifying an extension, the ACK endpoint won't be exposed
	Extension *component.ID `mapstructure:"extension"`
	// Path for Ack API, default is '/services/collector/ack'. Ignored if Extension is not provided.
	Path string `mapstructure:"path"`
}
