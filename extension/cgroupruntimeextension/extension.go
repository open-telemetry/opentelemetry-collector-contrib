// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cgroupruntimeextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/cgroupruntimeextension"

import (
	"context"
	"runtime"
	"runtime/debug"

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
	undoMemLimitFn undoFunc
}

func newCgroupRuntime(cfg *Config, logger *zap.Logger, maxProcsFn maxProcsFn, memLimitFn memLimitWithRatioFn) *cgroupRuntimeExtension {
	return &cgroupRuntimeExtension{
		config:              cfg,
		logger:              logger,
		maxProcsFn:          maxProcsFn,
		memLimitWithRatioFn: memLimitFn,
	}
}

func (c *cgroupRuntimeExtension) Start(_ context.Context, _ component.Host) error {
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
	}
	return nil
}

func (c *cgroupRuntimeExtension) Shutdown(_ context.Context) error {
	if c.undoMaxProcsFn != nil {
		c.undoMaxProcsFn()
	}
	if c.undoMemLimitFn != nil {
		c.undoMemLimitFn()
	}

	return nil
}
