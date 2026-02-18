// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tcp // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/tcp"

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/jpillora/backoff"
	"go.uber.org/zap"
	"golang.org/x/text/encoding"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/textutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

// Input is an operator that listens for log entries over tcp.
type Input struct {
	helper.InputOperator
	address         string
	MaxLogSize      int
	addAttributes   bool
	OneLogPerPacket bool

	listener net.Listener
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	tls      *tls.Config
	backoff  backoff.Backoff

	encoding  encoding.Encoding
	splitFunc bufio.SplitFunc
	resolver  *helper.IPResolver
}

// Start will start listening for log entries over tcp.
func (i *Input) Start(_ operator.Persister) error {
	if err := i.configureListener(); err != nil {
		return fmt.Errorf("failed to listen on interface: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	i.cancel = cancel
	i.goListen(ctx)
	return nil
}

func (i *Input) configureListener() error {
	if i.tls == nil {
		listener, err := net.Listen("tcp", i.address)
		if err != nil {
			return fmt.Errorf("failed to configure tcp listener: %w", err)
		}
		i.listener = listener
		return nil
	}

	i.tls.Time = time.Now
	i.tls.Rand = rand.Reader

	listener, err := tls.Listen("tcp", i.address, i.tls)
	if err != nil {
		return fmt.Errorf("failed to configure tls listener: %w", err)
	}

	i.listener = listener
	return nil
}

// goListenn will listen for tcp connections.
func (i *Input) goListen(ctx context.Context) {
	i.wg.Go(func() {
		for {
			conn, err := i.listener.Accept()
			if err != nil {
				select {
				case <-ctx.Done():
					return
				default:
					i.Logger().Debug("Listener accept error", zap.Error(err))
					time.Sleep(i.backoff.Duration())
					continue
				}
			}
			i.backoff.Reset()

			i.Logger().Debug("Received connection", zap.String("address", conn.RemoteAddr().String()))
			subctx, cancel := context.WithCancel(ctx)
			i.goHandleClose(subctx, conn)
			i.goHandleMessages(subctx, conn, cancel)
		}
	})
}

// goHandleClose will wait for the context to finish before closing a connection.
func (i *Input) goHandleClose(ctx context.Context, conn net.Conn) {
	i.wg.Go(func() {
		<-ctx.Done()
		i.Logger().Debug("Closing connection", zap.String("address", conn.RemoteAddr().String()))
		if err := conn.Close(); err != nil {
			i.Logger().Error("Failed to close connection", zap.Error(err))
		}
	})
}

// goHandleMessages will handles messages from a tcp connection.
func (i *Input) goHandleMessages(ctx context.Context, conn net.Conn, cancel context.CancelFunc) {
	i.wg.Go(func() {
		defer cancel()

		dec := i.encoding.NewDecoder()
		if i.OneLogPerPacket {
			var buf bytes.Buffer
			_, err := io.Copy(&buf, conn)
			if err != nil {
				i.Logger().Error("IO copy net connection buffer error", zap.Error(err))
			}
			log := truncateMaxLog(buf.Bytes(), i.MaxLogSize)
			i.handleMessage(ctx, conn, dec, log)
			return
		}

		buf := make([]byte, 0, i.MaxLogSize)

		scanner := bufio.NewScanner(conn)
		scanner.Buffer(buf, i.MaxLogSize)

		scanner.Split(i.splitFunc)

		for scanner.Scan() {
			i.handleMessage(ctx, conn, dec, scanner.Bytes())
		}

		if err := scanner.Err(); err != nil {
			i.Logger().Error("Scanner error", zap.Error(err))
		}
	})
}

func (i *Input) handleMessage(ctx context.Context, conn net.Conn, dec *encoding.Decoder, log []byte) {
	decoded, err := textutils.DecodeAsString(dec, log)
	if err != nil {
		i.Logger().Error("Failed to decode data", zap.Error(err))
		return
	}

	entry, err := i.NewEntry(decoded)
	if err != nil {
		i.Logger().Error("Failed to create entry", zap.Error(err))
		return
	}

	if i.addAttributes {
		entry.AddAttribute("net.transport", "IP.TCP")
		if addr, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
			ip := addr.IP.String()
			entry.AddAttribute("net.peer.ip", ip)
			entry.AddAttribute("net.peer.port", strconv.FormatInt(int64(addr.Port), 10))
			entry.AddAttribute("net.peer.name", i.resolver.GetHostFromIP(ip))
		}

		if addr, ok := conn.LocalAddr().(*net.TCPAddr); ok {
			ip := addr.IP.String()
			entry.AddAttribute("net.host.ip", addr.IP.String())
			entry.AddAttribute("net.host.port", strconv.FormatInt(int64(addr.Port), 10))
			entry.AddAttribute("net.host.name", i.resolver.GetHostFromIP(ip))
		}
	}

	err = i.Write(ctx, entry)
	if err != nil {
		i.Logger().Error("Failed to write entry", zap.Error(err))
	}
}

func truncateMaxLog(data []byte, maxLogSize int) (token []byte) {
	if len(data) >= maxLogSize {
		return data[:maxLogSize]
	}

	if len(data) == 0 {
		return nil
	}

	return data
}

// Stop will stop listening for log entries over TCP.
func (i *Input) Stop() error {
	if i.cancel == nil {
		return nil
	}
	i.cancel()

	if i.listener != nil {
		if err := i.listener.Close(); err != nil {
			i.Logger().Error("failed to close TCP connection", zap.Error(err))
		}
	}

	i.wg.Wait()
	if i.resolver != nil {
		i.resolver.Stop()
	}
	return nil
}
