// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"go.opentelemetry.io/collector/config/configtls"
)

// Supervisor is the Supervisor config file format.
type Supervisor struct {
	Server *OpAMPServer
	Agent  *Agent
}

type OpAMPServer struct {
	Endpoint   string
	TLSSetting configtls.TLSClientSetting `mapstructure:"tls,omitempty"`
}

type Agent struct {
	Executable string
}
