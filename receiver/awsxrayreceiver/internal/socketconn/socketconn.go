// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package socketconn // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/socketconn"

// SocketConn is an interface for socket connection.
type SocketConn interface {
	// Reads a packet from the connection, copying the payload into b. It returns number of bytes copied.
	Read(b []byte) (int, error)

	// Closes the connection.
	Close() error
}
