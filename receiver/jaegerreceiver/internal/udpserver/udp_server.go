// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package udpserver

import (
	"context"
	"net"
	"net/http"
	"sync"

	"go.uber.org/zap"
)

type PacketHandler interface {
	Handle(ctx context.Context, packet []byte) error
}

type UDPServer struct {
	conn          net.PacketConn
	maxPacketSize int
	handler       PacketHandler
	logger        *zap.Logger
	wg            sync.WaitGroup
	stopCh        chan struct{}
}

func New(addr string, maxPacketSize int, handler PacketHandler, logger *zap.Logger) (*UDPServer, error) {
	conn, err := net.ListenPacket("udp", addr)
	if err != nil {
		return nil, err
	}

	return &UDPServer{
		conn:          conn,
		maxPacketSize: maxPacketSize,
		handler:       handler,
		logger:        logger,
		stopCh:        make(chan struct{}),
	}, nil
}

func (s *UDPServer) Start() error {
	s.wg.Add(1)
	go s.serve()
	return nil
}

func (s *UDPServer) Stop() {
	close(s.stopCh)
	if err := s.conn.Close(); err != nil {
		s.logger.Error("Error closing UDP connection", zap.Error(err))
	}
	s.wg.Wait()
}

func (s *UDPServer) serve() {
	defer s.wg.Done()

	buf := make([]byte, s.maxPacketSize)
	for {
		select {
		case <-s.stopCh:
			return
		default:
			n, _, err := s.conn.ReadFrom(buf)
			if err != nil {
				if err == http.ErrHandlerTimeout || err == context.DeadlineExceeded {
					s.logger.Warn("Timeout error reading from UDP", zap.Error(err))
					continue
				}
				if opErr, ok := err.(*net.OpError); ok {
					if opErr.Op == "read" && opErr.Net == "udp" {
						if opErr.Err.Error() == "use of closed network connection" {
							s.logger.Debug("UDP connection closed")
							return
						}
					}
				}

				select {
				case <-s.stopCh:
				default:
					s.logger.Error("Error reading from UDP", zap.Error(err))
				}
				return
			}

			if n > 0 {
				packet := make([]byte, n)
				copy(packet, buf[:n])

				if err := s.handler.Handle(context.Background(), packet); err != nil {
					s.logger.Error("Error handling UDP packet", zap.Error(err))
				}
			}
		}
	}
}

func (s *UDPServer) LocalAddr() net.Addr {
	return s.conn.LocalAddr()
}
