// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package client // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver/internal/client"

import (
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

// Graphite is a struct that defines the relevant properties of a graphite
// connection.
// This code was initially taken from
// https://github.com/census-ecosystem/opencensus-go-exporter-graphite/tree/master/internal/client
// and modified for the needs of testing the Carbon receiver package and is not
// intended/tested to be used in production.
type Graphite struct {
	Endpoint string
	Timeout  time.Duration
	Conn     io.Writer
}

// Transport is used as an enum to select the type of transport to be used.
type Transport int

// Available transport options: TCP and UDP.
const (
	TCP Transport = iota
	UDP
)

const defaultTimeout = 5

// NewGraphite is a method that's used to create a new Graphite instance.
// This code was initially taken from
// https://github.com/census-ecosystem/opencensus-go-exporter-graphite/tree/master/internal/client
// and modified for the needs of testing the Carbon receiver package and is not
// intended/tested to be used in production.
func NewGraphite(transport Transport, endpoint string) (*Graphite, error) {
	graphite := &Graphite{Endpoint: endpoint}
	err := graphite.connect(transport)
	if err != nil {
		return nil, err
	}

	return graphite, nil
}

// connect populates the Graphite.conn.
func (g *Graphite) connect(transport Transport) error {
	if cl, ok := g.Conn.(io.Closer); ok {
		cl.Close()
	}

	if g.Timeout == 0 {
		g.Timeout = defaultTimeout * time.Second
	}

	var err error
	switch transport {
	case TCP:
		g.Conn, err = net.DialTimeout("tcp", g.Endpoint, g.Timeout)
	case UDP:
		var udpAddr *net.UDPAddr
		udpAddr, err = net.ResolveUDPAddr("udp", g.Endpoint)
		if err != nil {
			return err
		}
		g.Conn, err = net.DialUDP("udp", nil, udpAddr)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown transport %d", transport)
	}

	return err
}

// Disconnect closes the Graphite.conn field
func (g *Graphite) Disconnect() (err error) {
	if cl, ok := g.Conn.(io.Closer); ok {
		err = cl.Close()
	}
	g.Conn = nil
	return err
}

// SendMetric method can be used to just pass a metric name and value and
// have it be sent to the Graphite host
func (g *Graphite) SendMetric(metric Metric) error {
	_, err := fmt.Fprint(g.Conn, metric.String())
	if err != nil {
		return err
	}
	return nil
}

// SputterThenSendMetric method sends a bad partial metric, then the whole metric across.
func (g *Graphite) SputterThenSendMetric(metric Metric) error {
	str := metric.String()
	for i := 0; i < 5; i++ {
		if _, err := fmt.Fprint(g.Conn, ""); err != nil {
			return err
		}
		if err := g.Disconnect(); err != nil {
			return err
		}
		if err := g.connect(TCP); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprint(g.Conn, str); err != nil {
		return err
	}

	return nil
}

// SendMetrics method can be used to pass a set of metrics and
// have it be sent to the Graphite host
func (g *Graphite) SendMetrics(metrics []Metric) error {
	sb := strings.Builder{}
	for i, metric := range metrics {
		if _, err := sb.WriteString(metric.String()); err != nil {
			return err
		}
		if i == len(metrics)-1 {
			break
		}
		if err := sb.WriteByte('\n'); err != nil {
			return err
		}
	}
	_, err := fmt.Fprint(g.Conn, sb.String())
	if err != nil {
		return err
	}
	return nil
}

// Metric contains the metric fields expected by Graphite.
type Metric struct {
	Name      string
	Value     float64
	Timestamp time.Time
}

// String formats a Metric to the format expected bt Graphite.
func (m Metric) String() string {
	return fmt.Sprintf(
		"%s %s %d\n",
		m.Name,
		strconv.FormatFloat(m.Value, 'f', -1, 64),
		m.Timestamp.Unix(),
	)
}
