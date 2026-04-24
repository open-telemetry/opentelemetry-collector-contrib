// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alertsgenconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/alertsgenconnector"
import "go.opentelemetry.io/collector/component"

type Config struct{}

func createDefaultConfig() component.Config {
	return &Config{}
}
