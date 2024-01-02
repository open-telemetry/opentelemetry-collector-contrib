// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fluentforwardreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/fluentforwardreceiver"

// Config defines configuration for the fluentforward receiver.
type Config struct {

	// The address to listen on for incoming Fluent Forward events.  Should be
	// of the form `<ip addr>:<port>` (TCP) or `unix://<socket_path>` (Unix
	// domain socket).
	ListenAddress string `mapstructure:"endpoint"`
}
