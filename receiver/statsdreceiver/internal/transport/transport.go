package transport // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/internal/transport"

import "errors"

// Transport is a set of constants of the transport supported by this receiver.
type Transport string

var (
	ErrUnsupportedTransport       = errors.New("unsupported transport")
	ErrUnsupportedPacketTransport = errors.New("unsupported Packet transport")
	ErrUnsupportedStreamTransport = errors.New("unsupported Stream transport")
)

const (
	UDP  Transport = "udp"
	UDP4 Transport = "udp4"
	UDP6 Transport = "udp6"
	TCP  Transport = "tcp"
	TCP4 Transport = "tcp4"
	TCP6 Transport = "tcp6"
)

// Create a Transport based on the transport string or return error if it is not supported.
func NewTransport(ts string) (Transport, error) {
	trans := Transport(ts)
	switch trans {
	case UDP, UDP4, UDP6:
		return trans, nil
	case TCP, TCP4, TCP6:
		return trans, nil
	}
	return Transport(""), ErrUnsupportedTransport
}

// Returns the string of this transport.
func (trans Transport) String() string {
	switch trans {
	case UDP, UDP4, UDP6, TCP, TCP4, TCP6:
		return string(trans)
	}
	return ""
}

// Returns true if the transport is packet based.
func (trans Transport) IsPacketTransport() bool {
	switch trans {
	case UDP, UDP4, UDP6:
		return true
	}
	return false
}

// Returns true if the transport is stream based.
func (trans Transport) IsStreamTransport() bool {
	switch trans {
	case TCP, TCP4, TCP6:
		return true
	}
	return false
}
