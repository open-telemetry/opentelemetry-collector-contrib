// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package chrony // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/chronyreceiver/internal/chrony"

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"time"

	"github.com/facebook/time/ntp/chrony"
	"github.com/tilinna/clock"
	"go.uber.org/multierr"
)

var (
	errBadRequest = errors.New("bad request")
)

type Client interface {
	// GetTrackingData will connection the configured chronyd endpoint
	// and will read that instance tracking information relatively to the configured
	// upstream NTP server(s).
	GetTrackingData(ctx context.Context) (*Tracking, error)
}

type clientOption func(c *client)

// client is a partial rewrite of the client provided by
// github.com/facebook/time/ntp/chrony
//
// The reason for the partial rewrite is that the original
// client uses logrus' global instance within the main code path.
type client struct {
	proto, addr string
	timeout     time.Duration
	dialer      func(ctx context.Context, network, addr string) (net.Conn, error)
}

// New creates a client ready to use with chronyd
func New(addr string, timeout time.Duration, opts ...clientOption) (Client, error) {
	if timeout < 1 {
		return nil, errors.New("timeout must be positive")
	}

	network, endpoint, err := SplitNetworkEndpoint(addr)
	if err != nil {
		return nil, err
	}

	var d net.Dialer

	c := &client{
		proto:   network,
		addr:    endpoint,
		timeout: timeout,
		dialer:  d.DialContext,
	}
	for _, opt := range opts {
		opt(c)
	}

	return c, nil
}

func (c *client) GetTrackingData(ctx context.Context) (*Tracking, error) {
	clk := clock.FromContext(ctx)

	ctx, cancel := clk.TimeoutContext(ctx, c.timeout)
	defer cancel()

	sock, err := c.dialer(ctx, c.proto, c.addr)
	if err != nil {
		return nil, err
	}

	deadline, ok := ctx.Deadline()
	if !ok {
		return nil, errors.New("no deadline set")
	}

	if err = sock.SetDeadline(deadline); err != nil {
		return nil, err
	}

	packet := chrony.NewTrackingPacket()
	packet.SetSequence(uint32(clk.Now().UnixNano()))

	if err := binary.Write(sock, binary.BigEndian, packet); err != nil {
		return nil, multierr.Combine(err, sock.Close())
	}
	data := make([]uint8, 1024)
	if _, err := sock.Read(data); err != nil {
		return nil, multierr.Combine(err, sock.Close())
	}

	if err := sock.Close(); err != nil {
		return nil, err
	}

	return newTrackingData(data)
}
