// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snmptrapreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmptrapreceiver"

import (
	"context"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/gosnmp/gosnmp"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

const trapQueueCap = 1024

type snmpTrapReceiver struct {
	cfg    *Config
	next   consumer.Logs
	logger *zap.Logger

	listenMut sync.Mutex
	listener  *gosnmp.TrapListener
	tr        Translator

	queue chan plog.Logs
	wg    sync.WaitGroup
	stop  context.CancelFunc
}

func newReceiver(cfg *Config, set receiver.Settings, next consumer.Logs) (*snmpTrapReceiver, error) {
	return &snmpTrapReceiver{
		cfg:    cfg,
		next:   next,
		logger: set.Logger,
		queue:  make(chan plog.Logs, trapQueueCap),
	}, nil
}

func (r *snmpTrapReceiver) Start(ctx context.Context, _ component.Host) error {
	runCtx, cancel := context.WithCancel(context.Background())
	r.stop = cancel

	slogger := slog.Default()
	tr, err := NewGosmiTranslator(r.cfg.MIBPaths, slogger)
	if err != nil {
		r.logger.Warn("gosmi translator unavailable, using numeric OIDs", zap.Error(err))
		tr = NoopTranslator{}
	}
	r.tr = tr

	params, err := GoSNMPParams(r.listenConfig(), slogger)
	if err != nil {
		cancel()
		return err
	}

	tl := gosnmp.NewTrapListener()
	tl.Params = params
	tl.OnNewTrap = func(packet *gosnmp.SnmpPacket, addr *net.UDPAddr) {
		r.handleTrap(packet, addr)
	}

	errCh := make(chan error, 1)
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		errCh <- tl.Listen(r.cfg.ListenAddress)
	}()

	select {
	case <-tl.Listening():
		r.logger.Info("listening for SNMP traps", zap.String("listen_address", r.cfg.ListenAddress))
	case err := <-errCh:
		r.wg.Wait()
		cancel()
		if err == nil {
			return context.Canceled
		}
		return err
	case <-ctx.Done():
		tl.Close()
		r.wg.Wait()
		cancel()
		return ctx.Err()
	}

	r.listenMut.Lock()
	r.listener = tl
	r.listenMut.Unlock()

	r.wg.Add(1)
	go r.drain(runCtx)
	return nil
}

func (r *snmpTrapReceiver) Shutdown(_ context.Context) error {
	if r.stop != nil {
		r.stop()
	}
	r.listenMut.Lock()
	if r.listener != nil {
		r.listener.Close()
		r.listener = nil
	}
	r.listenMut.Unlock()
	r.wg.Wait()
	return nil
}

func (r *snmpTrapReceiver) drain(ctx context.Context) {
	defer r.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case ld := <-r.queue:
			if r.next == nil {
				continue
			}
			if err := r.next.ConsumeLogs(ctx, ld); err != nil {
				r.logger.Error("failed to consume trap logs", zap.Error(err))
			}
		}
	}
}

func (r *snmpTrapReceiver) handleTrap(packet *gosnmp.SnmpPacket, addr *net.UDPAddr) {
	if packet == nil {
		return
	}
	if !CommunityAllowed(r.cfg.Communities, packet.Community) {
		return
	}

	tr := r.tr
	if tr == nil {
		tr = NoopTranslator{}
	}
	rec := Decode(packet, addr, time.Now(), DecodeOptions{
		IncludeCommunity: r.cfg.IncludeCommunity,
		Translator:       tr,
	})
	if r.cfg.DropUndefined && !rec.Resolved() {
		return
	}

	ld := logsFromRecord(rec, r.cfg.Attributes)
	select {
	case r.queue <- ld:
	default:
		r.logger.Warn("dropping trap, outbound queue full")
	}
}

func (r *snmpTrapReceiver) listenConfig() ListenConfig {
	return ListenConfig{
		ListenAddress:    r.cfg.ListenAddress,
		Communities:      r.cfg.Communities,
		IncludeCommunity: r.cfg.IncludeCommunity,
		DropUndefined:    r.cfg.DropUndefined,
		MIBPaths:         r.cfg.MIBPaths,
		V3:               r.cfg.V3,
	}
}
