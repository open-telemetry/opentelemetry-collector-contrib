// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package transport // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver/transport"

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"sync"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"go.opentelemetry.io/collector/consumer"

	internaldata "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus"
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
	var metrics []*metricspb.Metric
	buf := bytes.NewBuffer(data)
	for {
		bytes, err := buf.ReadBytes((byte)('\n'))
		if errors.Is(err, io.EOF) {
			if len(bytes) == 0 {
				// Completed without errors.
				break
			}
		}
		line := strings.TrimSpace(string(bytes))
		if line != "" {
			numReceivedMetricPoints++
			metric, err := p.Parse(line)
			if err != nil {
				u.reporter.OnTranslationError(ctx, err)
				continue
			}

			metrics = append(metrics, metric)
		}
	}

	err := nextConsumer.ConsumeMetrics(ctx, internaldata.OCToMetrics(nil, nil, metrics))
	u.reporter.OnMetricsProcessed(ctx, numReceivedMetricPoints, err)
}
