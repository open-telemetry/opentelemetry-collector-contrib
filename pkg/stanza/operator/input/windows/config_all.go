// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package windows // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/windows"

import (
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

const operatorType = "windows_eventlog_input"

// EventDataFormat controls the structure of the event_data field in the log body.
type EventDataFormat string

const (
	// EventDataFormatMap emits event_data as a flat map with named Data elements
	// as direct keys and anonymous Data elements numbered as param1, param2, etc.
	EventDataFormatMap EventDataFormat = "map"
	// EventDataFormatArray emits event_data with a nested "data" array of
	// single-key maps, preserving the original format.
	EventDataFormatArray EventDataFormat = "array"
)

// NewConfig will return an event log config with default values.
func NewConfig() *Config {
	return NewConfigWithID(operatorType)
}

// NewConfig will return an event log config with default values.
func NewConfigWithID(operatorID string) *Config {
	return &Config{
		InputConfig:         helper.NewInputConfig(operatorID, operatorType),
		MaxReads:            100,
		StartAt:             "end",
		PollInterval:        1 * time.Second,
		WaitTimeout:         5 * time.Second,
		IgnoreChannelErrors: false,
		EventDataFormat:     EventDataFormatMap,
	}
}

// Config is the configuration of a windows event log operator.
type Config struct {
	helper.InputConfig  `mapstructure:",squash"`
	Channel             string        `mapstructure:"channel"`
	IgnoreChannelErrors bool          `mapstructure:"ignore_channel_errors,omitempty"`
	MaxReads            int           `mapstructure:"max_reads,omitempty"`
	StartAt             string        `mapstructure:"start_at,omitempty"`
	PollInterval        time.Duration `mapstructure:"poll_interval,omitempty"`
	MaxEventsPerPoll    int           `mapstructure:"max_events_per_poll,omitempty"`
	// WaitTimeout is the maximum duration to wait for new events before performing a
	// safety-net poll in event-driven mode (see stanza.windows.eventDrivenScraping
	// feature gate). Under normal conditions the subscription signal fires immediately, so
	// this value is rarely reached. Defaults to 5s. Ignored when the feature gate is not enabled.
	WaitTimeout              time.Duration   `mapstructure:"wait_timeout,omitempty"`
	Raw                      bool            `mapstructure:"raw,omitempty"`
	EventDataFormat          EventDataFormat `mapstructure:"event_data_format,omitempty"`
	IncludeLogRecordOriginal bool            `mapstructure:"include_log_record_original,omitempty"`
	SuppressRenderingInfo    bool            `mapstructure:"suppress_rendering_info,omitempty"`
	ExcludeProviders         []string        `mapstructure:"exclude_providers,omitempty"`
	Remote                   RemoteConfig    `mapstructure:"remote,omitempty"`
	Query                    *string         `mapstructure:"query,omitempty"`
}

// RemoteConfig is the configuration for a remote server.
type RemoteConfig struct {
	Server   string `mapstructure:"server"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	Domain   string `mapstructure:"domain,omitempty"`
}
