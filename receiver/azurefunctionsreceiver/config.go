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
	HTTP *confighttp.ServerConfig `mapstructure:"http"`

	// Auth is the component.ID of the extension that provides Azure authentication
	Auth component.ID `mapstructure:"auth"`

	// Triggers holds configuration for Azure Functions triggers (e.g. Event Hub)
	Triggers *TriggersConfig `mapstructure:"triggers"`
}

// TriggersConfig groups all supported trigger types for this receiver.
type TriggersConfig struct {
	// EventHub configures the Event Hub trigger: log bindings and their encodings.
	EventHub *EventHubTriggerConfig `mapstructure:"event_hub"`

	_ struct{} // prevent unkeyed literal initialization
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

// hasAnyBinding reports whether at least one trigger has at least one binding.
func (t *TriggersConfig) hasAnyBinding() bool {
	return t.EventHub != nil && len(t.EventHub.Logs) > 0
}

// Validate checks if the receiver configuration is valid.
func (cfg *Config) Validate() error {
	var errs []error
	if cfg.HTTP == nil || cfg.HTTP.NetAddr.Endpoint == "" {
		errs = append(errs, errors.New("missing http server settings"))
	}

	if cfg.Triggers == nil {
		errs = append(errs, errors.New("missing triggers configuration"))
	} else if !cfg.Triggers.hasAnyBinding() {
		errs = append(errs, errors.New("at least one configured trigger with at least one binding is required"))
	}

	if cfg.Triggers != nil && cfg.Triggers.EventHub != nil {
		eh := cfg.Triggers.EventHub
		seen := make(map[string]struct{}, len(eh.Logs))
		for i, log := range eh.Logs {
			if log.Name == "" {
				errs = append(errs, fmt.Errorf("triggers.event_hub.logs[%d].name must be set", i))
			} else if _, ok := seen[log.Name]; ok {
				errs = append(errs, fmt.Errorf("triggers.event_hub.logs: duplicate binding name %q", log.Name))
			} else {
				seen[log.Name] = struct{}{}
			}
			if log.Encoding.String() == "" {
				errs = append(errs, fmt.Errorf("triggers.event_hub.logs[%d].encoding must be set", i))
			}
		}
	}

	return errors.Join(errs...)
}
