// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package netflowreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/netflowreceiver"

import "fmt"

// Config represents the receiver config settings within the collector's config.yaml
type Config struct {
	Listeners []ListenerConfig `mapstructure:"listeners"`
}

type ListenerConfig struct {

	// The scheme defines the type of flow data that the listener will receive
	// The scheme must be one of sflow, netflow, or flow
	Scheme string `mapstructure:"scheme"`

	// The hostname or IP address that the listener will bind to
	Hostname string `mapstructure:"hostname"`

	// The port that the listener will bind to
	Port int `mapstructure:"port"`

	// The number of sockets that the listener will use
	Sockets int `mapstructure:"sockets"`

	// The number of workers that the listener will use to decode incoming flow messages
	// By default it will be two times the number of sockets
	// Ideally set this to the number of CPU cores
	Workers int `mapstructure:"workers"`

	// The size of the queue that the listener will use
	// This is a buffer that will hold flow messages before they are processed by a worker
	QueueSize int `mapstructure:"queue_size"`
}

// Validate checks if the receiver configuration is valid
func (cfg *Config) Validate() error {
	validSchemes := [3]string{"sflow", "netflow", "flow"}

	for _, listener := range cfg.Listeners {

		validScheme := false
		for _, scheme := range validSchemes {
			if listener.Scheme == scheme {
				validScheme = true
				break
			}
		}
		if !validScheme {
			return fmt.Errorf("scheme must be one of sflow, netflow, or flow")
		}

		if listener.Sockets <= 0 {
			return fmt.Errorf("sockets must be greater than 0")
		}

		if listener.Workers <= 0 {
			return fmt.Errorf("workers must be greater than 0")
		}

		if listener.QueueSize <= 0 {
			listener.QueueSize = defaultQueueSize
		}

		if listener.Port <= 0 {
			return fmt.Errorf("port must be greater than 0")
		}
	}

	return nil
}
