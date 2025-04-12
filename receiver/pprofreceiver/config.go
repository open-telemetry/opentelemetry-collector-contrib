// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pprofreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pprofreceiver"

import "go.opentelemetry.io/collector/config/confighttp"

type Config struct {
	confighttp.ClientConfig `mapstructure:",squash"`
}
