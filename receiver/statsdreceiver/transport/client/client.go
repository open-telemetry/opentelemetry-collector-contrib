// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package client // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/transport/client"

import (
	"fmt"
	"io"
	"net"
)

// StatsD defines the properties of a StatsD connection.
type StatsD struct {
	Host string
	Port int
	Conn io.Writer
}

// Transport is an enum to select the type of transport.
type Transport int

const (
	// TCP Transport
	TCP Transport = iota
	// UDP Transport
	UDP
)

// NewStatsD creates a new StatsD instance to support the need for testing
// the statsdreceiver package and is not intended/tested to be used in production.
func NewStatsD(transport Transport, host string, port int) (*StatsD, error) {
	statsd := &StatsD{
		Host: host,
		Port: port,
	}
	err := statsd.connect(transport)
	if err != nil {
		return nil, err
	}

	return statsd, nil
}

// connect populates the StatsD.Conn
func (s *StatsD) connect(transport Transport) error {
	if cl, ok := s.Conn.(io.Closer); ok {
		cl.Close()
	}

	address := fmt.Sprintf("%s:%d", s.Host, s.Port)

	var err error
	switch transport {
	case TCP:
		// TODO: implement TCP support
		return fmt.Errorf("TCP unsupported")
	case UDP:
		var udpAddr *net.UDPAddr
		udpAddr, err = net.ResolveUDPAddr("udp", address)
		if err != nil {
			return err
		}
		s.Conn, err = net.DialUDP("udp", nil, udpAddr)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown transport: %d", transport)
	}

	return err
}

// Disconnect closes the StatsD.Conn.
func (s *StatsD) Disconnect() error {
	var err error
	if cl, ok := s.Conn.(io.Closer); ok {
		err = cl.Close()
	}
	s.Conn = nil
	return err
}

// SendMetric sends the input metric to the StatsD connection.
func (s *StatsD) SendMetric(metric Metric) error {
	_, err := fmt.Fprint(s.Conn, metric.String())
	if err != nil {
		return err
	}
	return nil
}

// Metric contains the metric fields for a StatsD message.
type Metric struct {
	Name  string
	Value string
	Type  string
}

// String formats a Metric into a StatsD message.
func (m Metric) String() string {
	return fmt.Sprintf("%s:%s|%s", m.Name, m.Value, m.Type)
}
