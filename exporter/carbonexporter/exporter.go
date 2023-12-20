// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package carbonexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/carbonexporter"

import (
	"context"
	"net"
	"sync"
	"time"

	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry"
)

// newCarbonExporter returns a new Carbon exporter.
func newCarbonExporter(ctx context.Context, cfg *Config, set exporter.CreateSettings) (exporter.Metrics, error) {
	sender := carbonSender{
		writeTimeout: cfg.Timeout,
		conns:        newConnPool(cfg.TCPAddr, cfg.Timeout, cfg.MaxIdleConns),
	}

	exp, err := exporterhelper.NewMetricsExporter(
		ctx,
		set,
		cfg,
		sender.pushMetricsData,
		// We don't use exporterhelper.WithTimeout because the TCP connection does not accept writing with context.
		exporterhelper.WithQueue(cfg.QueueConfig),
		exporterhelper.WithRetry(cfg.RetryConfig),
		exporterhelper.WithShutdown(sender.Shutdown))
	if err != nil {
		return nil, err
	}

	return resourcetotelemetry.WrapMetricsExporter(cfg.ResourceToTelemetryConfig, exp), nil
}

// carbonSender is the struct tying the translation function and the TCP
// connections into an implementations of exporterhelper.PushMetricsData so
// the exporter can leverage the helper and get consistent observability.
type carbonSender struct {
	writeTimeout time.Duration
	conns        connPool
}

func (cs *carbonSender) pushMetricsData(_ context.Context, md pmetric.Metrics) error {
	lines := metricDataToPlaintext(md)

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
	conn, err := cs.conns.get()
	if err != nil {
		return err
	}

	if err = conn.SetWriteDeadline(time.Now().Add(cs.writeTimeout)); err != nil {
		// Do not re-enqueue the connection since it failed to set a deadline.
		return multierr.Append(err, conn.Close())
	}

	// If we did not write all bytes will get an error, so no need to check for that.
	_, err = conn.Write([]byte(lines))
	if err != nil {
		// Do not re-enqueue the connection since it failed to write.
		return multierr.Append(err, conn.Close())
	}

	// Even if we close the connection because of the max idle connections,
	cs.conns.put(conn)
	return nil
}

func (cs *carbonSender) Shutdown(context.Context) error {
	return cs.conns.close()
}

// connPool is a very simple implementation of a pool of net.Conn instances.
type connPool interface {
	get() (net.Conn, error)
	put(conn net.Conn)
	close() error
}

func newConnPool(
	tcpConfig confignet.TCPAddr,
	timeout time.Duration,
	maxIdleConns int,
) connPool {
	if maxIdleConns == 0 {
		return &nopConnPool{
			timeout:   timeout,
			tcpConfig: tcpConfig,
		}
	}
	return &connPoolWithIdle{
		timeout:      timeout,
		tcpConfig:    tcpConfig,
		maxIdleConns: maxIdleConns,
	}
}

// nopConnPool is a very simple implementation that does not cache any net.Conn.
type nopConnPool struct {
	timeout   time.Duration
	tcpConfig confignet.TCPAddr
}

func (cp *nopConnPool) get() (net.Conn, error) {
	return createTCPConn(cp.tcpConfig, cp.timeout)
}

func (cp *nopConnPool) put(conn net.Conn) {
	_ = conn.Close()
}

func (cp *nopConnPool) close() error {
	return nil
}

// connPool is a very simple implementation of a pool of net.Conn instances.
//
// It keeps at most maxIdleConns net.Conn and always "popping" the most
// recently returned to the pool. There is no accounting to terminating old
// unused connections.
type connPoolWithIdle struct {
	timeout      time.Duration
	maxIdleConns int
	mtx          sync.Mutex
	conns        []net.Conn
	tcpConfig    confignet.TCPAddr
}

func (cp *connPoolWithIdle) get() (net.Conn, error) {
	if conn := cp.getFromCache(); conn != nil {
		return conn, nil
	}

	return createTCPConn(cp.tcpConfig, cp.timeout)
}

func (cp *connPoolWithIdle) put(conn net.Conn) {
	cp.mtx.Lock()
	defer cp.mtx.Unlock()
	// Do not cache if above limit.
	if len(cp.conns) > cp.maxIdleConns {
		_ = conn.Close()
		return
	}
	cp.conns = append(cp.conns, conn)
}

func (cp *connPoolWithIdle) getFromCache() net.Conn {
	cp.mtx.Lock()
	defer cp.mtx.Unlock()
	lastIdx := len(cp.conns) - 1
	if lastIdx < 0 {
		return nil
	}
	conn := cp.conns[lastIdx]
	cp.conns = cp.conns[0:lastIdx]
	return conn
}

func (cp *connPoolWithIdle) close() error {
	cp.mtx.Lock()
	defer cp.mtx.Unlock()

	var errs error
	for _, conn := range cp.conns {
		errs = multierr.Append(errs, conn.Close())
	}
	cp.conns = nil
	return errs
}

func createTCPConn(tcpConfig confignet.TCPAddr, timeout time.Duration) (net.Conn, error) {
	c, err := net.DialTimeout("tcp", tcpConfig.Endpoint, timeout)
	if err != nil {
		return nil, err
	}
	return c, err
}
