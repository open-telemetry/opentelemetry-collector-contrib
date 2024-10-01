// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package grpc // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension/internal/grpc"

import "go.opentelemetry.io/collector/config/configgrpc"

type Config struct {
	configgrpc.ServerConfig `mapstructure:",squash"`
}
