// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package configtcp // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver/internal/configtcp"

import (
	"net"
	"time"

	"go.opentelemetry.io/collector/component"
)

type TCPClientSettings struct {
	// Endpoint is always required
	Endpoint string        `mapstructure:"endpoint"`
	Timeout  time.Duration `mapstructure:"timeout"`
}

type Client struct {
	net.Conn
	Address  string
	Timeout  time.Duration
	DialFunc func(network, address string, timeout time.Duration) (net.Conn, error)
}

// Dial starts a TCP session.
func (c *Client) Dial(endpoint string, timeout time.Duration) (err error) {
	c.Conn, err = c.DialFunc("tcp", endpoint, timeout)
	if err != nil {
		return err
	}
	return nil
}

func (tcs *TCPClientSettings) ToClient(_ component.Host, _ component.TelemetrySettings) (*Client, error) {
	return &Client{
		Timeout:  tcs.Timeout,
		Address:  tcs.Endpoint,
		DialFunc: net.DialTimeout,
	}, nil
}
