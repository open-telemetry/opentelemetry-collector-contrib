// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fluentforwardreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/fluentforwardreceiver"

import (
	"errors"
	"net"
	"syscall"

	"go.uber.org/zap"
)

// See https://github.com/fluent/fluentd/wiki/Forward-Protocol-Specification-v1#heartbeat-message
func respondToHeartbeats(udpSock net.PacketConn, logger *zap.Logger) {
	buf := make([]byte, 1)
	for {
		n, addr, err := udpSock.ReadFrom(buf)
		if err != nil || n == 0 {
			if errors.Is(err, syscall.EINVAL) || errors.Is(err, net.ErrClosed) {
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
