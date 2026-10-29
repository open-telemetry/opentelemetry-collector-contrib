// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cgroupruntimeextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/cgroupruntimeextension"

import (
	"context"
	"runtime"
	"runtime/debug"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

type (
	undoFunc            func()
	maxProcsFn          func() (undoFunc, error)
	memLimitWithRatioFn func(float64) (undoFunc, error)
)

type cgroupRuntimeExtension struct {
	config *Config
	logger *zap.Logger

	// runtime modifiers
	maxProcsFn
	undoMaxProcsFn undoFunc

	memLimitWithRatioFn
	undoMemLimitFn        undoFunc
	memLimitRefreshCancel context.CancelFunc
	memLimitRefreshDone   chan struct{}
}

func newCgroupRuntime(cfg *Config, logger *zap.Logger, maxProcsFn maxProcsFn, memLimitFn memLimitWithRatioFn) *cgroupRuntimeExtension {
	return &cgroupRuntimeExtension{
		config:              cfg,
		logger:              logger,
		maxProcsFn:          maxProcsFn,
		memLimitWithRatioFn: memLimitFn,
	}
}

func (c *cgroupRuntimeExtension) Start(ctx context.Context, _ component.Host) error {
	var err error
	if c.config.GoMaxProcs.Enabled {
		c.undoMaxProcsFn, err = c.maxProcsFn()
		if err != nil {
			return err
		}

		c.logger.Info("GOMAXPROCS has been set",
			zap.Int("GOMAXPROCS", runtime.GOMAXPROCS(-1)),
		)
	}

	if c.config.GoMemLimit.Enabled {
		c.undoMemLimitFn, err = c.memLimitWithRatioFn(c.config.GoMemLimit.Ratio)
		if err != nil {
			return err
		}

		c.logger.Info("GOMEMLIMIT has been set",
			zap.Int64("GOMEMLIMIT", debug.SetMemoryLimit(-1)),
		)

		if c.config.GoMemLimit.RefreshInterval > 0 {
			// automemlimit.WithRefreshInterval currently starts an unmanaged goroutine.
			// Keep refresh scheduling in this extension so it can be cleanly stopped in Shutdown.
			// See: https://github.com/KimMachineGun/automemlimit/issues/29
			refreshCtx, cancel := context.WithCancel(ctx)
			c.memLimitRefreshCancel = cancel
			c.memLimitRefreshDone = make(chan struct{})
			go c.refreshGoMemLimit(refreshCtx)
		}
	}
	return nil
}

func (c *cgroupRuntimeExtension) refreshGoMemLimit(ctx context.Context) {
	defer close(c.memLimitRefreshDone)

	ticker := time.NewTicker(c.config.GoMemLimit.RefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := c.memLimitWithRatioFn(c.config.GoMemLimit.Ratio); err != nil {
				c.logger.Warn("GOMEMLIMIT refresh failed", zap.Error(err))
			}
		}
	}
}

func (c *cgroupRuntimeExtension) Shutdown(_ context.Context) error {
	if c.memLimitRefreshCancel != nil {
		c.memLimitRefreshCancel()
		<-c.memLimitRefreshDone
	}

	if c.undoMaxProcsFn != nil {
		c.undoMaxProcsFn()
	}
	if c.undoMemLimitFn != nil {
		c.undoMemLimitFn()
	}

	return nil
}
