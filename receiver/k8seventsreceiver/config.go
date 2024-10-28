// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8seventsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8seventsreceiver"

import (
	"fmt"

	k8s "k8s.io/client-go/kubernetes"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

type EventType string

const (
	EventTypeNormal  EventType = "Normal"
	EventTypeWarning EventType = "Warning"
)

// Config defines configuration for kubernetes events receiver.
type Config struct {
	k8sconfig.APIConfig `mapstructure:",squash"`

	// List of ‘namespaces’ to collect events from.
	Namespaces []string `mapstructure:"namespaces"`

	// List of ‘eventtypes’ to filter.
	EventTypes []EventType `mapstructure:"event_types,omitempty"`

	// Include only the specified involved objects. ObjectKind to List of Reasons.
	IncludeInvolvedObject map[string]InvolvedObjectProperties `mapstructure:"include_involved_objects,omitempty"`

	// For mocking
	makeClient func(apiConf k8sconfig.APIConfig) (k8s.Interface, error)
}

type InvolvedObjectProperties struct {
	// Include only the specified reasons. If its empty, list events of all reasons.
	IncludeReasons []string `mapstructure:"include_reasons,omitempty"`

	//Can be enhanced to take in object names with reg ex etc.
}

func (cfg *Config) Validate() error {

	for _, eventType := range cfg.EventTypes {
		switch eventType {
		case EventTypeNormal, EventTypeWarning:
		default:
			return fmt.Errorf("invalid event_type %s, must be one of %s or %s", eventType, EventTypeNormal, EventTypeWarning)
		}
	}

	return cfg.APIConfig.Validate()
}

func (cfg *Config) getK8sClient() (k8s.Interface, error) {
	if cfg.makeClient == nil {
		cfg.makeClient = k8sconfig.MakeClient
	}
	return cfg.makeClient(cfg.APIConfig)
}
