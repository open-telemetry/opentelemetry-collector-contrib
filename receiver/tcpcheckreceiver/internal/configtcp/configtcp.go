// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package configtcp // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver/internal/configtcp"

import (
	"context"
	"net"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
)

type TCPClientSettings struct {
	// Endpoint is always required
	Endpoint string        `mapstructure:"endpoint"`
	Timeout  time.Duration `mapstructure:"timeout"`
}

type Client struct {
	net.Conn
	TCPAddrConfig confignet.TCPAddrConfig
}

// Dial starts a TCP session.
func (c *Client) Dial() (err error) {
	c.Conn, err = c.TCPAddrConfig.Dial(context.Background())
	if err != nil {
		return err
	}
	return nil
}

func (tcs *TCPClientSettings) ToClient(_ component.Host, _ component.TelemetrySettings) (*Client, error) {
	return &Client{
		TCPAddrConfig: confignet.TCPAddrConfig{
			Endpoint: tcs.Endpoint,
			DialerConfig: confignet.DialerConfig{
				Timeout: tcs.Timeout,
			},
		},
	}, nil
}
