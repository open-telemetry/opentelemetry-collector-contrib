// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package chrony // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/chronyreceiver/internal/chrony"

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"syscall"
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

	// Close closes the underlying connection and cleans up any resources.
	Close() error
}

// ClientOption configures the chrony client.
type ClientOption func(c *client)

// client is a partial rewrite of the client provided by
// github.com/facebook/time/ntp/chrony
//
// The reason for the partial rewrite is that the original
// client uses logrus' global instance within the main code path.
type client struct {
	proto, addr   string
	fileMountPath string
	localAddr     string
	timeout       time.Duration
	dialer        func(ctx context.Context, network, addr string) (net.Conn, error)
	newLocalAddr  func(dir string) (string, error)

	conn net.Conn
}

// WithFileMountPath sets a filesystem-based directory for unixgram
// connections to bind a random local socket. Required when the collector
// and chronyd run in separate network namespaces sharing a filesystem volume.
func WithFileMountPath(dir string) ClientOption {
	return func(c *client) {
		c.fileMountPath = dir
	}
}

// New creates a client ready to use with chronyd
func New(addr string, timeout time.Duration, opts ...ClientOption) (Client, error) {
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

	if c.fileMountPath != "" {
		if c.newLocalAddr == nil {
			c.newLocalAddr = generateLocalAddr
		}
		c.localAddr, err = c.newLocalAddr(c.fileMountPath)
		if err != nil {
			return nil, err
		}
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

// GetTrackingData is not safe for concurrent use.
// The scraper framework serializes calls so this is not an issue in practice.
func (c *client) GetTrackingData(ctx context.Context) (_ *Tracking, err error) {
	ctx, cancel := c.getContext(ctx)
	defer cancel()

	sock, err := c.dialer(ctx, c.proto, c.addr)
	if err != nil {
		return nil, errors.Join(err, c.Close())
	}
	c.conn = sock
	defer func() {
		err = errors.Join(err, c.Close())
	}()

	if deadline, ok := ctx.Deadline(); ok {
		if setDeadlineErr := c.conn.SetDeadline(deadline); setDeadlineErr != nil {
			return nil, setDeadlineErr
		}
	}

	packet := chrony.NewTrackingPacket()
	packet.SetSequence(uint32(clockwork.FromContext(ctx).Now().UnixNano()))

	if writeErr := binary.Write(c.conn, binary.BigEndian, packet); writeErr != nil {
		return nil, writeErr
	}
	data := make([]uint8, 1024)
	if _, err = c.conn.Read(data); err != nil {
		return nil, err
	}

	return newTrackingData(data)
}

func (c *client) Close() error {
	var err error
	if c.conn != nil {
		err = c.conn.Close()
		c.conn = nil
	}
	if c.localAddr != "" && c.proto == "unixgram" {
		if rmErr := os.Remove(c.localAddr); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
			err = errors.Join(err, rmErr)
		}
	}
	return err
}

func (c *client) getContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if c.timeout == 0 {
		return context.WithCancel(ctx)
	}

	return context.WithTimeout(ctx, c.timeout)
}

func generateLocalAddr(dir string) (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate local socket name: %w", err)
	}
	localAddr := filepath.Join(dir, fmt.Sprintf("otel-chrony-%x.sock", b))
	if len(localAddr) >= len(syscall.RawSockaddrUnix{}.Path) {
		return "", fmt.Errorf("file_mount_path %q produces a unix socket path %q that exceeds the platform limit of %d bytes", dir, localAddr, len(syscall.RawSockaddrUnix{}.Path)-1)
	}
	return localAddr, nil
}

func (c *client) setLocalAddrGenerator(fn func(string) (string, error)) {
	c.newLocalAddr = fn
}
