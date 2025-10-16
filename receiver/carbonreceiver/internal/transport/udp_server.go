// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transport // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver/internal/transport"

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"sync"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver/protocol"
)

type udpServer struct {
	wg         sync.WaitGroup
	packetConn net.PacketConn
	reporter   Reporter
}

var _ Server = (*udpServer)(nil)

// NewUDPServer creates a transport.Server using UDP as its transport.
func NewUDPServer(addr string) (Server, error) {
	packetConn, err := net.ListenPacket("udp", addr)
	if err != nil {
		return nil, err
	}

	u := udpServer{
		packetConn: packetConn,
	}
	return &u, nil
}

func (u *udpServer) ListenAndServe(
	parser protocol.Parser,
	nextConsumer consumer.Metrics,
	reporter Reporter,
) error {
	if parser == nil || nextConsumer == nil || reporter == nil {
		return errNilListenAndServeParameters
	}

	u.reporter = reporter

	buf := make([]byte, 65527) // max size for udp packet body (assuming ipv6)
	for {
		n, _, err := u.packetConn.ReadFrom(buf)
		if n > 0 {
			u.wg.Add(1)
			bufCopy := make([]byte, n)
			copy(bufCopy, buf)
			go func() {
				u.handlePacket(parser, nextConsumer, bufCopy)
				u.wg.Done()
			}()
		}
		if err != nil {
			u.reporter.OnDebugf(
				"UDP Transport (%s) - ReadFrom error: %v",
				u.packetConn.LocalAddr(),
				err)
			var netErr net.Error
			if errors.As(err, &netErr) {
				if netErr.Timeout() {
					continue
				}
			}
			return err
		}
	}
}

func (u *udpServer) Close() error {
	err := u.packetConn.Close()
	u.wg.Wait()
	return err
}

func (u *udpServer) handlePacket(
	p protocol.Parser,
	nextConsumer consumer.Metrics,
	data []byte,
) {
	ctx := u.reporter.OnDataReceived(context.Background())
	var numReceivedMetricPoints int
	metrics := pmetric.NewMetrics()
	sm := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()

	buf := bytes.NewBuffer(data)
	for {
		data, err := buf.ReadBytes(byte('\n'))
		if errors.Is(err, io.EOF) {
			if len(data) == 0 {
				// Completed without errors.
				break
			}
		}
		trimmedData := bytes.TrimSpace(data)
		if len(trimmedData) != 0 {
			numReceivedMetricPoints++
			metric, err := p.Parse(trimmedData)
			if err != nil {
				u.reporter.OnTranslationError(ctx, err)
				continue
			}
			newMetric := sm.Metrics().AppendEmpty()
			metric.MoveTo(newMetric)
		}
	}

	err := nextConsumer.ConsumeMetrics(ctx, metrics)
	u.reporter.OnMetricsProcessed(ctx, numReceivedMetricPoints, err)
}
