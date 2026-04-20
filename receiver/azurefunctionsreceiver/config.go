// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azurefunctionsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azurefunctionsreceiver"

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
)

type Config struct {
	// HTTP defines the HTTP server settings for the Azure Functions invoke endpoints.
	HTTP *confighttp.ServerConfig `mapstructure:"http_config"`

	// Auth is the component.ID of the extension that provides Azure authentication
	Auth component.ID `mapstructure:"auth"`

	// EventHub configures the Event Hub trigger: log bindings and their encodings.
	EventHub *EventHubTriggerConfig `mapstructure:"event_hub"`
}

// EventHubTriggerConfig holds configuration for the Event Hub trigger.
type EventHubTriggerConfig struct {
	// Logs is the list of log bindings (e.g. name "logs" maps to path /logs). Each binding has its own encoding.
	Logs []LogsEncodingConfig `mapstructure:"logs"`
	// IncludeMetadata, when true, adds Azure Functions invoke metadata to resource attributes.
	IncludeMetadata bool `mapstructure:"include_metadata"`
}

// LogsEncodingConfig holds the binding name and encoding for a signal (e.g. one log binding).
// Name is the Azure Functions binding name and typically corresponds to the request path (e.g. /logs, /raw_logs).
type LogsEncodingConfig struct {
	Name     string       `mapstructure:"name"`
	Encoding component.ID `mapstructure:"encoding"`
}

// hasTriggerWithBindings returns true if any trigger is configured with at least one binding.
func (cfg *Config) hasTriggerWithBindings() bool {
	if cfg.EventHub != nil && len(cfg.EventHub.Logs) > 0 {
		return true
	}
	return false
}

// Validate checks if the receiver configuration is valid.
func (cfg *Config) Validate() error {
	var errs []error
	if cfg.HTTP == nil || cfg.HTTP.NetAddr.Endpoint == "" {
		errs = append(errs, errors.New("missing http server settings"))
	}

	if !cfg.hasTriggerWithBindings() {
		errs = append(errs, errors.New("at least one configured trigger with at least one binding is required"))
	}

	if cfg.EventHub != nil {
		seen := make(map[string]struct{}, len(cfg.EventHub.Logs))
		for i, log := range cfg.EventHub.Logs {
			if log.Name == "" {
				errs = append(errs, fmt.Errorf("event_hub.logs[%d].name must be set", i))
			} else if _, ok := seen[log.Name]; ok {
				errs = append(errs, fmt.Errorf("event_hub.logs: duplicate binding name %q", log.Name))
			} else {
				seen[log.Name] = struct{}{}
			}
			if log.Encoding.String() == "" {
				errs = append(errs, fmt.Errorf("event_hub.logs[%d].encoding must be set", i))
			}
		}
	}

	return errors.Join(errs...)
}
