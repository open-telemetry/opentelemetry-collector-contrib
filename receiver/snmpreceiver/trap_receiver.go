// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snmpreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver"

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/gosnmp/gosnmp"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"
)

const trapDataFormat = "snmp"

var errTrapQueueFull = errors.New("SNMP notification queue is full")

type queuedTrap struct {
	logs   plog.Logs
	obsCtx context.Context
}

type trapReceiver struct {
	cfg    *TrapsConfig
	next   consumer.Logs
	logger *zap.Logger
	obs    *receiverhelper.ObsReport
	parser *trapParser

	mu       sync.Mutex
	conn     net.PacketConn
	started  bool
	stopping bool
	cancel   context.CancelFunc

	queue       chan queuedTrap
	receiveDone chan struct{}
	consumeDone chan struct{}
	engines     map[string]trapEngineTime
}

func newTrapReceiver(cfg *TrapsConfig, settings receiver.Settings, next consumer.Logs) (receiver.Logs, error) {
	if cfg == nil {
		return nil, errors.New("traps configuration is required for a logs pipeline")
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	if next == nil {
		return nil, errors.New("SNMP notification logs consumer is required")
	}
	parser, err := newTrapParser(cfg)
	if err != nil {
		return nil, err
	}
	obs, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             settings.ID,
		Transport:              "udp",
		ReceiverCreateSettings: settings,
	})
	if err != nil {
		return nil, err
	}
	return &trapReceiver{
		cfg:         cfg,
		next:        next,
		logger:      settings.Logger,
		obs:         obs,
		parser:      parser,
		queue:       make(chan queuedTrap, cfg.QueueSize),
		receiveDone: make(chan struct{}),
		consumeDone: make(chan struct{}),
		engines:     make(map[string]trapEngineTime),
	}, nil
}

func (r *trapReceiver) Start(ctx context.Context, host component.Host) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.started || r.stopping {
		return errors.New("SNMP trap receiver cannot be started more than once")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	var lc net.ListenConfig
	conn, err := lc.ListenPacket(ctx, "udp", r.cfg.ListenAddress)
	if err != nil {
		return fmt.Errorf("listen for SNMP notifications: %w", err)
	}
	if err := ctx.Err(); err != nil {
		_ = conn.Close()
		return err
	}
	consumeCtx, cancel := context.WithCancel(context.Background())
	r.conn = conn
	r.cancel = cancel
	r.started = true
	go r.consume(consumeCtx)
	go r.receive(consumeCtx, host, conn)
	return nil
}

func (r *trapReceiver) Shutdown(ctx context.Context) error {
	r.mu.Lock()
	if !r.started {
		r.stopping = true
		r.mu.Unlock()
		return nil
	}
	conn, cancel := r.conn, r.cancel
	closeConn := !r.stopping
	r.stopping = true
	r.mu.Unlock()
	if closeConn {
		if err := conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			r.logger.Warn("Failed to close SNMP notification listener", zap.Error(err))
		}
	}
	// The receive loop owns closing the queue, so no producer can enqueue
	// during the drain. Do not cancel delivery until the deadline expires.
	select {
	case <-r.receiveDone:
	case <-ctx.Done():
		cancel()
		return ctx.Err()
	}
	select {
	case <-r.consumeDone:
		cancel()
		return nil
	case <-ctx.Done():
		cancel()
		return ctx.Err()
	}
}

func (r *trapReceiver) receive(ctx context.Context, host component.Host, conn net.PacketConn) {
	defer close(r.receiveDone)
	defer close(r.queue)
	// Cover a complete UDP datagram, including notifications larger than
	// gosnmp.TrapListener's default 4096-byte buffer.
	buffer := make([]byte, 65535)
	for {
		n, remote, err := conn.ReadFrom(buffer)
		if err != nil {
			if !errors.Is(err, net.ErrClosed) {
				componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(fmt.Errorf("read SNMP notification: %w", err)))
			}
			return
		}
		received := time.Now()
		obsCtx := r.obs.StartLogsOp(ctx)
		// Validate the message envelope before decoding and authenticating it.
		// Admission independently enforces the configured version, username,
		// minimum security level, and authenticated engine time window.
		if envelopeErr := checkTrapEnvelope(buffer[:n]); envelopeErr != nil {
			r.obs.EndLogsOp(obsCtx, trapDataFormat, 1, envelopeErr)
			continue
		}
		packet, err := safeUnmarshalTrap(r.parser, buffer[:n])
		if err == nil {
			err = admitTrap(r.cfg, packet, r.engines, received)
		}
		if err != nil {
			r.obs.EndLogsOp(obsCtx, trapDataFormat, 1, err)
			continue
		}
		sender, ok := remote.(*net.UDPAddr)
		if !ok {
			r.obs.EndLogsOp(obsCtx, trapDataFormat, 1, errors.New("SNMP notification sender is not a UDP address"))
			continue
		}
		logs, err := decodeTrap(packet, sender, received, r.cfg.IncludeCommunity)
		if err != nil {
			r.obs.EndLogsOp(obsCtx, trapDataFormat, 1, err)
			continue
		}
		for _, rl := range logs.ResourceLogs().All() {
			for _, sl := range rl.ScopeLogs().All() {
				for _, lr := range sl.LogRecords().All() {
					for key, value := range r.cfg.Attributes {
						lr.Attributes().PutStr(key, value)
					}
				}
			}
		}
		select {
		case r.queue <- queuedTrap{logs: logs, obsCtx: obsCtx}:
			if packet.PDUType == gosnmp.InformRequest {
				if err := acknowledgeTrapInform(conn, buffer[:n], sender); err != nil {
					r.logger.Warn("Failed to acknowledge SNMP inform", zap.Error(err))
				}
			}
		default:
			r.obs.EndLogsOp(obsCtx, trapDataFormat, logs.LogRecordCount(), errTrapQueueFull)
		}
	}
}

func (r *trapReceiver) consume(ctx context.Context) {
	defer close(r.consumeDone)
	for {
		select {
		case <-ctx.Done():
			// Shutdown can expire while reception is finishing. Continue
			// accounting for pending entries until the producer closes queue.
			for item := range r.queue {
				r.obs.EndLogsOp(item.obsCtx, trapDataFormat, item.logs.LogRecordCount(), ctx.Err())
			}
			return
		case item, ok := <-r.queue:
			if !ok {
				return
			}
			// Consumption transfers ownership to downstream components, which
			// may mutate or release the logs before returning.
			count := item.logs.LogRecordCount()
			err := r.next.ConsumeLogs(item.obsCtx, item.logs)
			r.obs.EndLogsOp(item.obsCtx, trapDataFormat, count, err)
			if err != nil {
				r.logger.Error("Failed to deliver SNMP notification logs", zap.Error(err))
			}
		}
	}
}
