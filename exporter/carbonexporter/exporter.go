// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package carbonexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/carbonexporter"

import (
	"context"
	"net"
	"sync"
	"time"

	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// newCarbonExporter returns a new Carbon exporter.
func newCarbonExporter(cfg *Config, set exporter.CreateSettings) (exporter.Metrics, error) {
	sender := carbonSender{
		connPool: newTCPConnPool(cfg.Endpoint, cfg.Timeout),
	}

	return exporterhelper.NewMetricsExporter(
		context.TODO(),
		set,
		cfg,
		sender.pushMetricsData,
		exporterhelper.WithShutdown(sender.Shutdown))
}

// carbonSender is the struct tying the translation function and the TCP
// connections into an implementations of exporterhelper.PushMetricsData so
// the exporter can leverage the helper and get consistent observability.
type carbonSender struct {
	connPool *connPool
}

func (cs *carbonSender) pushMetricsData(_ context.Context, md pmetric.Metrics) error {
	lines := metricDataToPlaintext(md)

	if _, err := cs.connPool.Write([]byte(lines)); err != nil {
		// Use the sum of converted and dropped since the write failed for all.
		return err
	}

	return nil
}

func (cs *carbonSender) Shutdown(context.Context) error {
	cs.connPool.Close()
	return nil
}

// connPool is a very simple implementation of a pool of net.TCPConn instances.
// The implementation hides the pool and exposes a Write and Close methods.
// It leverages the prior art from SignalFx Gateway (see
// https://github.com/signalfx/gateway/blob/master/protocol/carbon/conn_pool.go
// but not its implementation).
//
// It keeps a unbounded "stack" of TCPConn instances always "popping" the most
// recently returned to the pool. There is no accounting to terminating old
// unused connections as that was the case on the prior art mentioned above.
type connPool struct {
	mtx      sync.Mutex
	conns    []*net.TCPConn
	endpoint string
	timeout  time.Duration
}

func newTCPConnPool(
	endpoint string,
	timeout time.Duration,
) *connPool {
	return &connPool{
		endpoint: endpoint,
		timeout:  timeout,
	}
}

func (cp *connPool) Write(bytes []byte) (int, error) {
	var conn *net.TCPConn
	var err error

	// The deferred function below is what puts back connections on the pool.
	defer func() {
		if err == nil {
			cp.mtx.Lock()
			cp.conns = append(cp.conns, conn)
			cp.mtx.Unlock()
		} else if conn != nil {
			conn.Close()
		}
	}()

	start := time.Now()
	cp.mtx.Lock()
	lastIdx := len(cp.conns) - 1
	if lastIdx >= 0 {
		conn = cp.conns[lastIdx]
		cp.conns = cp.conns[0:lastIdx]
	}
	cp.mtx.Unlock()
	if conn == nil {
		if conn, err = cp.createTCPConn(); err != nil {
			return 0, err
		}
	}

	// There is no way to do a call equivalent to recvfrom with an empty buffer
	// to check if the connection was terminated (if the size of the buffer is
	// 0 the Read call doesn't call lower level). So due to buffer sizes it is
	// possible that a write will succeed on a connection that was already
	// closed by the server.
	//
	// At least on Darwin it is possible to work around this by configuring the
	// buffer on each call, ie.:
	//
	// if err = conn.SetWriteBuffer(len(bytes)-1); err != nil {
	//    return 0, err
	// }
	//
	// However, this causes a performance penalty of ~10% cpu and it is not
	// present in various implementations of Carbon clients. Considering these
	// facts this "workaround" is not being added at this moment. If it is
	// needed in some scenarios the workaround should be validated on other
	// platforms and offered as a configuration setting.

	if err = conn.SetWriteDeadline(start.Add(cp.timeout)); err != nil {
		return 0, err
	}

	var n int
	n, err = conn.Write(bytes)
	return n, err
}

func (cp *connPool) Close() {
	cp.mtx.Lock()
	defer cp.mtx.Unlock()

	for _, conn := range cp.conns {
		conn.Close()
	}
	cp.conns = nil
}

func (cp *connPool) createTCPConn() (*net.TCPConn, error) {
	c, err := net.DialTimeout("tcp", cp.endpoint, cp.timeout)
	if err != nil {
		return nil, err
	}
	return c.(*net.TCPConn), err
}
