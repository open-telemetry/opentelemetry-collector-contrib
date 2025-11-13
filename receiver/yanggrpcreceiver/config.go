// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package yanggrpcreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/yanggrpcreceiver"

// Config defines configuration for yanggrpc receiver.
type Config struct{}

// Validate checks the receiver configuration is valid
func (*Config) Validate() error {
	return nil
}
