// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package communityidprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/communityidprocessor"

// Config holds the configuration for the CommunityId processor.
type Config struct{}

func (cfg *Config) Validate() error {
	return nil
}
