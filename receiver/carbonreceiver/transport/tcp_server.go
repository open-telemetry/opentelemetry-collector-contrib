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
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"go.opencensus.io/trace"
	"go.opentelemetry.io/collector/consumer"

	internaldata "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver/protocol"
)

const (
	// TCPIdleTimeoutDefault is the default timeout for idle TCP connections.
	TCPIdleTimeoutDefault = 30 * time.Second
)

type tcpServer struct {
	ln          net.Listener
	wg          sync.WaitGroup
	idleTimeout time.Duration
	reporter    Reporter
}

var _ Server = (*tcpServer)(nil)

// NewTCPServer creates a transport.Server using TCP as its transport.
func NewTCPServer(
	addr string,
	idleTimeout time.Duration,
) (Server, error) {
	if idleTimeout < 0 {
		return nil, fmt.Errorf("invalid idle timeout: %v", idleTimeout)
	}

	if idleTimeout == 0 {
		idleTimeout = TCPIdleTimeoutDefault
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	t := tcpServer{
		ln:          ln,
		idleTimeout: idleTimeout,
	}
	return &t, nil
}

func (t *tcpServer) ListenAndServe(
	parser protocol.Parser,
	nextConsumer consumer.Metrics,
	reporter Reporter,
) error {
	if parser == nil || nextConsumer == nil || reporter == nil {
		return errNilListenAndServeParameters
	}

	acceptedConnMap := make(map[net.Conn]struct{})
	connMapMtx := &sync.Mutex{}

	t.reporter = reporter
	var err error
	for {
		conn, acceptErr := t.ln.Accept()
		if acceptErr == nil {
			connMapMtx.Lock()
			acceptedConnMap[conn] = struct{}{}
			connMapMtx.Unlock()
			t.wg.Add(1)
			go func(c net.Conn) {
				t.handleConnection(parser, nextConsumer, c)
				connMapMtx.Lock()
				delete(acceptedConnMap, c)
				connMapMtx.Unlock()
				t.wg.Done()
			}(conn)
			continue
		}

		var netErr net.Error
		if errors.As(acceptErr, &netErr) {
			t.reporter.OnDebugf(
				"TCP Transport (%s) - Accept (temporary=%v) net.Error: %v",
				t.ln.Addr().String(),
				netErr.Timeout(),
				netErr)
			if netErr.Timeout() {
				continue
			}
		}

		err = acceptErr
		break
	}

	t.reporter.OnDebugf(
		"TCP Transport (%s) exiting Accept loop error: %v",
		t.ln.Addr().String(),
		err)

	// Close any lingering connection
	connMapMtx.Lock()
	for conn := range acceptedConnMap {
		conn.Close()
	}
	connMapMtx.Unlock()

	return err
}

func (t *tcpServer) Close() error {
	err := t.ln.Close()
	t.wg.Wait()
	return err
}

func (t *tcpServer) handleConnection(
	p protocol.Parser,
	nextConsumer consumer.Metrics,
	conn net.Conn,
) {
	defer conn.Close()
	var span *trace.Span
	reader := bufio.NewReader(conn)
	for {
		if span != nil {
			span.End()
			span = nil
		}

		if err := conn.SetDeadline(time.Now().Add(t.idleTimeout)); err != nil {
			t.reporter.OnDebugf(
				"TCP Transport (%s) - conn.SetDeadLine error: %v",
				t.ln.Addr(),
				err)
			return
		}

		// reader.ReadBytes call below will block until either:
		//
		// * a '\n' char is read
		// * the connection is closed (either by client or server)
		// * an idle timeout happens (see call to conn.SetDeadline above)
		//
		// Notice that it is possible for the function to return with error at
		// the same time that it returns data (typically the error is io.EOF in
		// this case).
		bytes, err := reader.ReadBytes((byte)('\n'))

		// It is possible to have new data in bytes and err to be io.EOF
		ctx := t.reporter.OnDataReceived(context.Background())
		var numReceivedMetricPoints int
		line := strings.TrimSpace(string(bytes))
		if line != "" {
			numReceivedMetricPoints++
			var metric *metricspb.Metric
			metric, err = p.Parse(line)
			if err != nil {
				t.reporter.OnTranslationError(ctx, err)
				continue
			}

			err = nextConsumer.ConsumeMetrics(ctx, internaldata.OCToMetrics(nil, nil, []*metricspb.Metric{metric}))
			t.reporter.OnMetricsProcessed(ctx, numReceivedMetricPoints, err)
			if err != nil {
				// The protocol doesn't account for returning errors.
				// Since this is a TCP connection it seems reasonable to close the
				// connection as a way to report "error" back to client and minimize
				// the effect of a client constantly submitting bad data.
				span.End()
				return
			}
		}

		netErr := &net.OpError{}
		if errors.As(err, &netErr) {
			t.reporter.OnDebugf("TCP Transport (%s) - net.OpError: %v", t.ln.Addr(), netErr)
			if netErr.Timeout() {
				// We want to end on timeout so idle connections are purged.
				span.End()
				return
			}
		}

		if errors.Is(err, io.EOF) {
			t.reporter.OnDebugf(
				"TCP Transport (%s) - error: %v",
				t.ln.Addr(),
				err)
			span.End()
			return
		}
	}
}
