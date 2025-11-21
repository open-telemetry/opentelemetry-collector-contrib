// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fluentforwardreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/fluentforwardreceiver"

import "time" // Config defines configuration for the fluentforward receiver.
type Config struct {
	// The address to listen on for incoming Fluent Forward events.  Should be
	// of the form `<ip addr>:<port>` (TCP) or `unix://<socket_path>` (Unix
	// domain socket).
	ListenAddress string `mapstructure:"endpoint"`
	// The duration to wait after a shutdown signal has been received to
	// process incoming events.
	ShutdownDelay time.Duration `mapstructure:"duration"`

	// prevent unkeyed literal initialization
	_ struct{}
}
