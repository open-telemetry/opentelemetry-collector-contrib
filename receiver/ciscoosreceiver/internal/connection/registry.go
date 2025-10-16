// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package connection // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/connection"

// SharedConnectionRegistry holds shared SSH connections for coordination between scrapers
var SharedConnectionRegistry = make(map[string]*SharedConnection)

// SharedConnection represents a shared SSH connection and RPC client
type SharedConnection struct {
	SSHClient *Client
	RPCClient *RPCClient
	Target    string
	OSType    string
	Connected bool
}
