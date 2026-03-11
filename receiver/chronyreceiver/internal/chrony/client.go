// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package chrony // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/chronyreceiver/internal/chrony"

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/facebook/time/ntp/chrony"
	"github.com/jonboulle/clockwork"
)

var errBadRequest = errors.New("bad request")

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
	localAddr   string
	timeout     time.Duration
	dialer      func(ctx context.Context, network, addr string) (net.Conn, error)
}

// WithLocalAddress sets a filesystem-based local socket address for unixgram
// connections. Required when the collector and chronyd run in separate network
// namespaces sharing a filesystem volume.
func WithLocalAddress(addr string) clientOption {
	return func(c *client) {
		c.localAddr = addr
	}
}

// New creates a client ready to use with chronyd
func New(addr string, timeout time.Duration, opts ...clientOption) (Client, error) {
	network, endpoint, err := SplitNetworkEndpoint(addr)
	if err != nil {
		return nil, err
	}

	c := &client{
		proto:   network,
		addr:    endpoint,
		timeout: timeout,
	}
	for _, opt := range opts {
		opt(c)
	}

	if c.dialer == nil {
		d := net.Dialer{}
		if c.localAddr != "" && c.proto == "unixgram" {
			d.LocalAddr = &net.UnixAddr{Name: c.localAddr, Net: "unixgram"}
		}
		c.dialer = d.DialContext
	}

	return c, nil
}

// GetTrackingData is not safe for concurrent use when localAddr is set;
// the scraper framework serializes calls so this is not an issue in practice.
func (c *client) GetTrackingData(ctx context.Context) (*Tracking, error) {
	ctx, cancel := c.getContext(ctx)
	defer cancel()

	if c.localAddr != "" && c.proto == "unixgram" {
		if err := removeIfSocket(c.localAddr); err != nil {
			return nil, err
		}
		defer func() { _ = removeIfSocket(c.localAddr) }()
	}

	sock, err := c.dialer(ctx, c.proto, c.addr)
	if err != nil {
		return nil, err
	}

	if deadline, ok := ctx.Deadline(); ok {
		err = sock.SetDeadline(deadline)
		if err != nil {
			return nil, err
		}
	}

	packet := chrony.NewTrackingPacket()
	packet.SetSequence(uint32(clockwork.FromContext(ctx).Now().UnixNano()))

	if err := binary.Write(sock, binary.BigEndian, packet); err != nil {
		return nil, errors.Join(err, sock.Close())
	}
	data := make([]uint8, 1024)
	if _, err := sock.Read(data); err != nil {
		return nil, errors.Join(err, sock.Close())
	}

	if err := sock.Close(); err != nil {
		return nil, err
	}

	return newTrackingData(data)
}

func (c *client) getContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if c.timeout == 0 {
		return context.WithCancel(ctx)
	}

	return context.WithTimeout(ctx, c.timeout)
}

// removeIfSocket removes the file at path only if it is a Unix socket.
// Returns nil if the file does not exist. Returns an error if the path
// exists but is not a socket, preventing accidental deletion of regular files.
func removeIfSocket(path string) error {
	fi, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if fi.Mode().Type()&os.ModeSocket == 0 {
		return fmt.Errorf("local_endpoint path %q exists but is not a socket", path)
	}
	return os.Remove(path)
}
