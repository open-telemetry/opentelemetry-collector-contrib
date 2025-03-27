// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package stefreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/stefreceiver"

// Config defines configuration for STEF receiver.
type Config struct{}

func (c *Config) Validate() error {
	return nil
}
