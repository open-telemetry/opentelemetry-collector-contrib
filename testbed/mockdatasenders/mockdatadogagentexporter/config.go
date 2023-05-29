// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mockdatadogagentexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/mockdatasenders/mockdatadogagentexporter"

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
)

// Config defines configuration for datadog receiver.
type Config struct {
	component.Config
	// client to send to the agent
	confighttp.HTTPClientSettings `mapstructure:",squash"`
}
