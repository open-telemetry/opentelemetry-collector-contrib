// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/stefexporter/internal"

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/jonboulle/clockwork"
	"go.uber.org/zap"
)

// ConnManager manages a pool of connections.
//
// It is responsible for creating connections, reconnecting or flushing connections
// in the background when needed, and ensuring each connection is used exclusively
// by one user at a time.
//
// In order to use the connections call Acquire, Release and DiscardAndClose methods.
type ConnManager struct {
	logger *zap.Logger

	// clock is used for mocking the clock for testing purposes.
	// Outside of tests it is a real clock.
	clock clockwork.Clock

	targetConnCount uint

	// Number of current connections, collectively in all pools or acquired.
	curConnCount atomic.Int64

	connCreator ConnCreator

	// Connection pools. curConnCount connections are either in one
	// of these pools, or are acquired temporarily,
	// i.e. curConnCount = len(idleConns) + len(recreateConns) + (acquired count).
	idleConns     chan *ManagedConn // Ready to be acquired.
	recreateConns chan *ManagedConn // Pending to be recreated.

	flushPeriod     time.Duration
	reconnectPeriod time.Duration

	// Flags to indicate if the goroutines are stopped.
	flusherStopped         bool
	durationLimiterStopped bool
	recreatorStopped       bool
	// stoppedCond is used to wait until all goroutines are stopped.
	stoppedCond *CancellableCond

	// stopSignal is used to signal all goroutines to stop.
	stopSignal chan struct{}
}

// ManagedConn wraps a Conn and keeps some tracking information that
// ConnManager needs.
type ManagedConn struct {
	// The underlying connection.
	conn Conn

	// The time when the connection was created.
	startTime time.Time

	// The time when the connection was last flushed. Set after conn.Flush() is called.
	lastFlush time.Time

	// needsFlush indicates if the connection needs to be flushed at the next periodic
	// background check that runs at the flushPeriod intervals.
	needsFlush bool

	// Keep track of the connection usage, whether it is acquired or no.
	isAcquired bool
}

// Conn returns the underlying connection.
func (c *ManagedConn) Conn() Conn {
	return c.conn
}

// ConnCreator allow creating connections.
type ConnCreator interface {
	// Create a new connection. May be called concurrently.
	// The attempt to create the connection should be cancelled if ctx is done.
	Create(ctx context.Context) (Conn, error)
}

// Conn represents a connection that can be closed or flushed.
type Conn interface {
	// Close the connection. The connection will be discarded
	// after this call returns.
	Close(ctx context.Context) error

	// Flush the connection. This is typically to send any buffered data.
	// Will be called periodically (see ConnManager flushPeriod) and
	// before ConnManager.Stop returns.
	Flush(context.Context) error
}

type ConnManagerSettings struct {
	Logger *zap.Logger

	// Creator helps create new connections.
	Creator ConnCreator

	// TargetConnCount is the number of connections desirable to maintain.
	// Must be >0.
	TargetConnCount uint

	// FlushPeriod is the approximate period to wait before flushing connections after
	// the user releases the connection. Setting this value to >0 avoids
	// unnecessary flushes when the connection is acquired and released frequently.
	// Setting this to 0 ensures flushing is done after every connection release.
	FlushPeriod time.Duration

	// ReconnectPeriod is the interval to reconnect connections. Each connection is
	// periodically reconnected approximately every reconnectPeriod.
	ReconnectPeriod time.Duration
}

func NewConnManager(set ConnManagerSettings) (*ConnManager, error) {
	if set.Logger == nil {
		set.Logger = zap.NewNop()
	}

	if set.TargetConnCount == 0 {
		return nil, errors.New("TargetConnCount must be >0")
	}

	if set.FlushPeriod <= 0 {
		return nil, errors.New("FlushPeriod must be >=0")
	}

	if set.ReconnectPeriod <= 0 {
		return nil, errors.New("ReconnectPeriod must be >0")
	}

	return &ConnManager{
		logger:          set.Logger,
		clock:           clockwork.NewRealClock(),
		connCreator:     set.Creator,
		targetConnCount: set.TargetConnCount,
		idleConns:       make(chan *ManagedConn, set.TargetConnCount),
		recreateConns:   make(chan *ManagedConn, set.TargetConnCount),
		flushPeriod:     set.FlushPeriod,
		reconnectPeriod: set.ReconnectPeriod,
		stoppedCond:     NewCancellableCond(),
		stopSignal:      make(chan struct{}),
	}, nil
}

// Start starts the connection manager. It will immediately start
// creating targetConnCount new connections by calling connCreator.Create().
// All successfully created connections will become available for acquisition.
func (c *ConnManager) Start() {
	// Put some dummy connections in recreateConns pool and let the recreator()
	// to replace them by proper connections in the background.
	for i := uint(0); i < c.targetConnCount; i++ {
		c.recreateConns <- &ManagedConn{}
		c.curConnCount.Add(1)
	}

	go c.flusher()
	go c.durationLimiter()
	go c.recreator()
}

// Stop stops the connection manager. It will wait until all acquired
// connections are returned. Then it will flush connections that
// are marked as needing to flush, and then will close all connections.
// Stop will be aborted if the context is done before all connections are closed.
func (c *ConnManager) Stop(ctx context.Context) error {
	// Signal goroutines to stop
	close(c.stopSignal)

	// Wait until all goroutines stop
	err := c.stoppedCond.Wait(
		ctx, func() bool {
			return c.flusherStopped && c.durationLimiterStopped && c.recreatorStopped
		},
	)
	if err != nil {
		c.logger.Error("Failed to stop connection manager", zap.Error(err))
		return err
	}

	return c.closeAll(ctx)
}

// Close all connections.
func (c *ConnManager) closeAll(ctx context.Context) error {
	// We must close exactly curConnCount connections in total.
	// All goroutines are stopped at this point, so they won't interfere.
	// All connections are either in the idleConns or recreateConns pools
	// or are acquired and will be returned to one of the pools soon.

	cnt := c.curConnCount.Load()
	c.logger.Debug("Closing connections", zap.Int64("count", cnt))

	var errs []error
	for i := int64(0); i < cnt; i++ {
		// Get a connection from one of the connection pools.
		var conn *ManagedConn
		var discarded bool
		select {
		case <-ctx.Done():
			return ctx.Err()
		case conn = <-c.recreateConns:
			// Connections in recreateConns are discarded and are candidates for recreation.
			// We don't need to flush them.
			discarded = true
		case conn = <-c.idleConns:
		}

		if conn.conn != nil {
			// Flush if needs a flush and is not discarded.
			if !discarded && conn.needsFlush {
				if err := conn.conn.Flush(ctx); err != nil {
					c.logger.Debug("Failed to flush connection", zap.Error(err))
					errs = append(errs, err)
					continue
				}
			}

			// And close the connection.
			if err := conn.conn.Close(ctx); err != nil {
				c.logger.Debug("Failed to close connection", zap.Error(err))
				errs = append(errs, err)
				continue
			}
		}
	}

	// Join all errors (if any)
	return errors.Join(errs...)
}

// Acquire an idle connection for exclusive use.
//
// Must call Release() or DiscardAndClose() when done.
// Returns an error if the connection is not available til ctx is done
// or if the manager is stopped.
func (c *ConnManager) Acquire(ctx context.Context) (*ManagedConn, error) {
	select {
	case conn := <-c.idleConns:
		if conn.isAcquired {
			panic("connection is not acquired")
		}
		conn.isAcquired = true
		return conn, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.stopSignal:
		return nil, errors.New("connection manager is stopped")
	}
}

// Release returns a previously acquired connection to the ConnManager
// and makes it available to be acquired again.
//
// If the connection was last flushed more than flushPeriod ago, it will
// be flushed otherwise it will be marked as needing to be flushed at the
// next opportunity.
func (c *ConnManager) Release(ctx context.Context, conn *ManagedConn) {
	if !conn.isAcquired {
		panic("connection is not acquired")
	}
	conn.isAcquired = false
	if c.clock.Since(conn.lastFlush) >= c.flushPeriod || c.flushPeriod == 0 {
		// Time to flush the connection.
		if err := conn.conn.Flush(ctx); err != nil {
			c.logger.Error("Failed to flush connection. Closing connection.", zap.Error(err))
			// Something went wrong, we need to recreate the connection since it
			// may no longer be usable.
			c.recreateConns <- conn
			return
		}
		conn.lastFlush = c.clock.Now()
	} else {
		// Remember that it needs to be flushed sometime in the future.
		conn.needsFlush = true
	}
	c.idleConns <- conn
}

// DiscardAndClose discards the acquired connection and closes it.
// This is normally used when the connection goes bad in some way
// and should not be reused.
// This will result in calling the Close method on the connection.
func (c *ConnManager) DiscardAndClose(conn *ManagedConn) {
	if !conn.isAcquired {
		panic("connection is not acquired")
	}
	conn.needsFlush = false
	c.recreateConns <- conn
}

func (c *ConnManager) flusher() {
	defer func() {
		c.stoppedCond.Cond.L.Lock()
		c.flusherStopped = true
		c.stoppedCond.Cond.L.Unlock()
		c.stoppedCond.Cond.Broadcast()
	}()

	if c.flushPeriod == 0 {
		// flusher is not needed, we will always flush immediately in Release().
		return
	}

	ticker := c.clock.NewTicker(c.flushPeriod)

	// Context that cancels on stopSignal.
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer cancel()
		<-c.stopSignal
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.Chan():
		loop:
			// Flush all idle connections that need flushing.
			for i := 0; i < len(c.idleConns); i++ {
				select {
				// Get one idle conn (if any).
				case conn := <-c.idleConns:
					if conn.needsFlush {
						conn.needsFlush = false
						if err := conn.conn.Flush(ctx); err != nil {
							c.logger.Error("Failed to flush connection. Closing connection.", zap.Error(err))
							// Something went wrong, we need to recreate the connection since it
							// may no longer be usable.
							c.recreateConns <- conn
							continue loop
						}
						conn.lastFlush = c.clock.Now()
					}
					// Return to idle pool.
					c.idleConns <- conn

				default:
					// No more available idle connections.
					break loop
				}
			}
		}
	}
}

// durationLimiter periodically checks connections and reconnects them if they
// were connected for more than reconnectPeriod. It will stagger the reconnections
// to avoid all connections reconnecting at the same c.clock.
func (c *ConnManager) durationLimiter() {
	defer func() {
		c.stoppedCond.Cond.L.Lock()
		c.durationLimiterStopped = true
		c.stoppedCond.Cond.L.Unlock()
		c.stoppedCond.Cond.Broadcast()
	}()

	// Context that cancels on stopSignal.
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer cancel()
		<-c.stopSignal
	}()

	// Each connection will be reconnected at approximately reconnectPeriod interval.
	// We reconnect one per tick.
	ticker := c.clock.NewTicker(c.reconnectPeriod / time.Duration(c.targetConnCount))
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.Chan():
			// Find an idle connection
			var conn *ManagedConn
			select {
			case <-ctx.Done():
				return
			case conn = <-c.idleConns:
			}

			// Check if it is time to reconnect.
			if c.clock.Since(conn.startTime) >= c.reconnectPeriod {
				if conn.needsFlush {
					conn.needsFlush = false
					// Flush it first.
					if err := conn.conn.Flush(ctx); err != nil {
						c.logger.Error("Failed to flush connection. Closing connection.", zap.Error(err))
					}
				}

				// Send it for reconnection (regardless of whether it was flushed or not).
				c.recreateConns <- conn
			} else {
				// Put it back, too soon to reconnect.
				c.idleConns <- conn
			}
		}
	}
}

// recreator closes connections from recreateConns pool
// and replaces them by new connections.
func (c *ConnManager) recreator() {
	defer func() {
		// Indicate we are stopped on exit.
		c.stoppedCond.Cond.L.Lock()
		c.recreatorStopped = true
		c.stoppedCond.Cond.L.Unlock()
		c.stoppedCond.Cond.Broadcast()
	}()

	for {
		select {
		case <-c.stopSignal:
			return

		case conn := <-c.recreateConns:
			// Track the number of connections.
			if c.curConnCount.Add(-1) < 0 {
				panic("negative connection count")
			}

			if conn.conn != nil {
				ctx, cancel := contextFromStopSignal(c.stopSignal)
				// Close the connection.
				if err := conn.conn.Close(ctx); err != nil {
					c.logger.Error("Failed to close connection", zap.Error(err))
				}
				cancel()
			}
			c.createNewConn()
		}
	}
}

// contextFromStopSignal creates a context that is cancelled when the
// provided channel is closed.
func contextFromStopSignal(stopSignal <-chan struct{}) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-stopSignal
		cancel()
	}()
	return ctx, cancel
}

// createNewConn creates a new connection and adds it to the idleConns pool.
// It will keep trying with backoff until connection creation succeeds or
// until ConnManager is stopped.
func (c *ConnManager) createNewConn() {
	c.logger.Debug("Creating new connection")

	// Create a context that will be cancelled when the manager is stopped.
	ctx, cancel := contextFromStopSignal(c.stopSignal)
	defer cancel()

	// Try to connect, retrying until succeeding, with exponential backoff.
	bo := backoff.NewExponentialBackOff()
	ticker := backoff.NewTicker(bo)
	defer ticker.Stop()

	for {
		// Wait until the next retry or until the manager is stopped.
		select {
		case <-c.stopSignal:
			return
		case <-ticker.C:
		}

		c.logger.Debug("calling Create() connection")
		conn, err := c.connCreator.Create(ctx)
		if err != nil || conn == nil {
			c.logger.Info("Failed to create connection. Will retry.", zap.Error(err))
			continue
		}

		now := c.clock.Now()
		managedConn := &ManagedConn{
			conn:      conn,
			startTime: now,
			lastFlush: now,
		}
		c.curConnCount.Add(1)
		c.idleConns <- managedConn
		return
	}
}
