// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sdnotifyextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/sdnotifyextension"

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/coreos/go-systemd/daemon"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensioncapabilities"
	"go.uber.org/zap"
)

type sdnotify struct {
	cfg    *Config
	logger *zap.Logger
	host   component.Host

	shutdownOnce sync.Once
	done         chan struct{}
	termCh       chan os.Signal
	sighupCh     chan os.Signal
}

// Extension is the union of capability interfaces sdnotify implements.
type Extension interface {
	extension.Extension
	extensioncapabilities.PipelineWatcher
}

var _ Extension = (*sdnotify)(nil)

func newSDNotify(cfg *Config, logger *zap.Logger) *sdnotify {
	return &sdnotify{
		cfg:      cfg,
		logger:   logger,
		done:     make(chan struct{}),
		termCh:   make(chan os.Signal, 1),
		sighupCh: make(chan os.Signal, 1),
	}
}

func (s *sdnotify) Start(_ context.Context, host component.Host) error {
	s.host = host

	// If NOTIFY_SOCKET environment variable is unset, then the sd_notify
	// protocol is no-op.
	//
	// See the link below for the relevant man page:
	// https://www.man7.org/linux/man-pages/man3/sd_notify.3.html
	if os.Getenv("NOTIFY_SOCKET") == "" {
		s.logger.Warn("NOTIFY_SOCKET is not set; sd_notify support is disabled")

		return nil
	}

	// STOPPING=1 must be sent only on genuine termination (SIGINT/SIGTERM).
	//
	// Note that this implementation does not guarantee ordering — for example,
	// it does not guarantee that the Collector's own signal handler won't run
	// before this extension's. The only way to truly guarantee that would be
	// for the OpenTelemetry Collector API to let components register custom
	// code to run on the controller's control loop when specific signal occur.
	//
	// See the issue below for discussion of this suggestion:
	// https://github.com/open-telemetry/opentelemetry-collector/issues/15732
	signal.Notify(s.termCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-s.done:
			return
		case <-s.termCh:
			sent, err := daemon.SdNotify(false, daemon.SdNotifyStopping)
			if err != nil {
				s.logger.Warn("sdnotify STOPPING=1 failed", zap.Error(err))
			} else if sent {
				s.logger.Info("sdnotify: sent STOPPING=1 to systemd")
			}
		}
	}()

	// RELOADING=1\nMONOTONIC_USEC=X is send only for Type=notify-reload services.
	// Systemd signals the process with SIGHUP when a reload is requested.
	// The process is responsible for reloading its configuration and informing
	// systemd when the reload has completed.
	//
	// Note that this approach has the same ordering limitation as our SIGINT/SIGTERM
	// handling above — there's no guarantee it runs before or after the
	// OpenTelemetry Collector's own signal handler.
	monotonicEpoch := time.Now()
	signal.Notify(s.sighupCh, syscall.SIGHUP)
	go func() {
		for {
			select {
			case <-s.done:
				return

			// This extension should not restart the process, because the collector handles it by itself.
			case <-s.sighupCh:
				// Per sd_notify(3): MONOTONIC_USEC must be CLOCK_MONOTONIC in microseconds,
				// formatted as a decimal string, in the same datagram as RELOADING=1.
				monotonicUSec := uint64(max(time.Since(monotonicEpoch), 0) / time.Microsecond)
				msg := fmt.Sprintf(
					"%s\nMONOTONIC_USEC=%d",
					daemon.SdNotifyReloading,
					monotonicUSec,
				)

				sent, err := daemon.SdNotify(false, msg)
				if err != nil {
					s.logger.Warn("sdnotify RELOADING=1 failed", zap.Error(err))
				} else if sent {
					s.logger.Info(
						"sdnotify: SIGHUP received, sent RELOADING=1 to systemd",
						zap.Uint64("monotonic_usec", monotonicUSec),
					)
				}
			}
		}
	}()

	// WATCHDOG=1 is the keep-alive ping that services need to issue in regular
	// intervals if WatchdogSec= is enabled for it.
	duration, err := daemon.SdWatchdogEnabled(false)
	switch {
	case err != nil:
		s.logger.Debug("sdnotify: SdWatchdogEnabled returned error; watchdog disabled",
			zap.Error(err))
	case duration == 0:
		s.logger.Debug("sdnotify: WATCHDOG_USEC not set; watchdog disabled")
	default:
		go func() {
			// Per sd_watchdog_enabled(3): It is recommended that a daemon sends a keep-alive
			// notification message to the service manager every half of the time returned here.
			ticker := time.NewTicker(duration / 2)
			defer ticker.Stop()
			for {
				select {
				case <-s.done:
					return
				case <-ticker.C:
					if _, err := daemon.SdNotify(false, daemon.SdNotifyWatchdog); err != nil {
						s.logger.Debug("sdnotify WATCHDOG=1 failed", zap.Error(err))
					}
				}
			}
		}()
	}

	return nil
}

func (s *sdnotify) Shutdown(_ context.Context) error {
	s.shutdownOnce.Do(func() {
		signal.Stop(s.termCh)
		signal.Stop(s.sighupCh)
		close(s.done)
	})

	return nil
}

func (s *sdnotify) Ready() error {
	// READY=1 informs systemd that the collector is fully ready to receive traffic.
	sent, err := daemon.SdNotify(false, daemon.SdNotifyReady)
	switch {
	case err != nil:
		return fmt.Errorf("sdnotify READY=1: %w", err)
	case sent:
		s.logger.Info("sdnotify: sent READY=1 to systemd")
	default:
		s.logger.Info("sdnotify: NOTIFY_SOCKET not set; READY=1 was a no-op")
	}

	return nil
}

func (*sdnotify) NotReady() error {
	return nil
}
