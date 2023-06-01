// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config

// Supervisor is the Supervisor config file format.
type Supervisor struct {
	Server *OpAMPServer
	Agent  *Agent
}

type OpAMPServer struct {
	Endpoint string
}

type Agent struct {
	Executable string
}
