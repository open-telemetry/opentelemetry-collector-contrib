// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fluentforwardreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/fluentforwardreceiver"

import (
	"context"
	"errors"
	"net"
	"syscall"

	"go.uber.org/zap"
)

// See https://github.com/fluent/fluentd/wiki/Forward-Protocol-Specification-v1#heartbeat-message
func respondToHeartbeats(ctx context.Context, udpSock net.PacketConn, logger *zap.Logger) {
	go func() {
		<-ctx.Done()
		udpSock.Close()
	}()

	buf := make([]byte, 1)
	for {
		n, addr, err := udpSock.ReadFrom(buf)
		if err != nil || n == 0 {
			if ctx.Err() != nil || errors.Is(err, syscall.EINVAL) {
				return
			}
			continue
		}
		// Technically the heartbeat should be a byte 0x00 but just echo back
		// whatever the client sent and move on.
		_, err = udpSock.WriteTo(buf, addr)
		if err != nil {
			logger.Debug("Failed to write back heartbeat packet", zap.String("addr", addr.String()), zap.Error(err))
		}
	}
}
