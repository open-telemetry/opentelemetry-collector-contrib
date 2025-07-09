// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transport // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver/internal/transport"

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver/protocol"
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
	reader := bufio.NewReader(conn)
	reporterActive := false
	var ctx context.Context
	for {
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

		var numReceivedMetricPoints int
		line := strings.TrimSpace(string(bytes))
		if line != "" {
			if !reporterActive {
				ctx = t.reporter.OnDataReceived(context.Background())
				reporterActive = true
			}
			numReceivedMetricPoints++
			var metric pmetric.Metric
			metric, err = p.Parse(line)
			if err != nil {
				t.reporter.OnTranslationError(ctx, err)
				continue
			}
			metrics := pmetric.NewMetrics()
			newMetric := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
			metric.MoveTo(newMetric)
			err = nextConsumer.ConsumeMetrics(ctx, metrics)
			t.reporter.OnMetricsProcessed(ctx, numReceivedMetricPoints, err)
			reporterActive = false
			if err != nil {
				// The protocol doesn't account for returning errors.
				// Since this is a TCP connection it seems reasonable to close the
				// connection as a way to report "error" back to client and minimize
				// the effect of a client constantly submitting bad data.
				return
			}
		}

		netErr := &net.OpError{}
		if errors.As(err, &netErr) {
			t.reporter.OnDebugf("TCP Transport (%s) - net.OpError: %v", t.ln.Addr(), netErr)
			if netErr.Timeout() {
				// We want to end on timeout so idle connections are purged.
				if reporterActive {
					t.reporter.OnMetricsProcessed(ctx, 0, err)
				}
				return
			}
		}

		if errors.Is(err, io.EOF) {
			t.reporter.OnDebugf(
				"TCP Transport (%s) - error: %v",
				t.ln.Addr(),
				err)

			if reporterActive {
				t.reporter.OnMetricsProcessed(ctx, 0, err)
			}
			return
		}
	}
}
