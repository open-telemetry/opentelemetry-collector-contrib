// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package client // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/internal/transport/client"

import (
	"fmt"
	"io"
	"net"
	"strings"
)

// StatsD defines the properties of a StatsD connection.
type StatsD struct {
	transport string
	address   string
	conn      io.Writer
}

// NewStatsD creates a new StatsD instance to support the need for testing
// the statsdreceiver package and is not intended/tested to be used in production.
func NewStatsD(transport string, address string) (*StatsD, error) {
	statsd := &StatsD{
		transport: transport,
		address:   address,
	}

	err := statsd.connect()
	if err != nil {
		return nil, err
	}

	return statsd, nil
}

// connect populates the StatsD.conn
func (s *StatsD) connect() error {
	switch s.transport {
	case "udp":
		udpAddr, err := net.ResolveUDPAddr(s.transport, s.address)
		if err != nil {
			return err
		}
		s.conn, err = net.DialUDP(s.transport, nil, udpAddr)
		if err != nil {
			return err
		}
	case "tcp":
		var err error
		s.conn, err = net.Dial(s.transport, s.address)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown/unsupported transport: %s", s.transport)
	}

	return nil
}

// Disconnect closes the StatsD.conn.
func (s *StatsD) Disconnect() error {
	var err error
	if cl, ok := s.conn.(io.Closer); ok {
		err = cl.Close()
	}
	s.conn = nil
	return err
}

// SendMetric sends the input metric to the StatsD connection.
func (s *StatsD) SendMetric(metric Metric) error {
	_, err := io.Copy(s.conn, strings.NewReader(metric.String()))
	if err != nil {
		return fmt.Errorf("send metric on test client: %w", err)
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
	return fmt.Sprintf("%s:%s|%s\n", m.Name, m.Value, m.Type)
}
