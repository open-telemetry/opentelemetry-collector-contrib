// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package grpc // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2/internal/grpc"

import "go.opentelemetry.io/collector/config/configgrpc"

type Settings struct {
	configgrpc.GRPCServerSettings `mapstructure:",squash"`
}
