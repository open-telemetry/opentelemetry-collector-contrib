// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transport // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/internal/transport"

// Transport is a set of constants of the transport supported by this receiver.
type Transport string

const (
	UDP  Transport = "udp"
	UDP4 Transport = "udp4"
	UDP6 Transport = "udp6"
)

// NewTransport creates a Transport based on the transport string or returns an empty Transport.
func NewTransport(ts string) Transport {
	trans := Transport(ts)
	switch trans {
	case UDP, UDP4, UDP6:
		return trans
	}
	return Transport("")
}

// String casts the transport to a String if the Transport is supported. Return an empty Transport overwise.
func (trans Transport) String() string {
	switch trans {
	case UDP, UDP4, UDP6:
		return string(trans)
	}
	return ""
}

// IsPacketTransport returns true if the transport is packet based.
func (trans Transport) IsPacketTransport() bool {
	switch trans {
	case UDP, UDP4, UDP6:
		return true
	}
	return false
}
