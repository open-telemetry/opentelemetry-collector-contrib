// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cgroupruntimeextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/cgroupruntimeextension"

import (
	"context"
	"fmt"

	"github.com/KimMachineGun/automemlimit/memlimit"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/automaxprocs/maxprocs"
	"go.uber.org/zap"
)

type cgroupRuntimeExtension struct {
	config *Config
	logger *zap.Logger

	maxProcsUndoFn func()
}

func newCgroupRuntime(cfg *Config, set component.TelemetrySettings) *cgroupRuntimeExtension {
	return &cgroupRuntimeExtension{
		config: cfg,
		logger: set.Logger,
	}
}

func (c *cgroupRuntimeExtension) Start(ctx context.Context, host component.Host) error {
	if c.config.GoMaxProcs.Enable {
		undo, err := maxprocs.Set(maxprocs.Logger(func(str string, params ...interface{}) {
			c.logger.Debug(fmt.Sprintf(str, params))
		}))
		if err != nil {
			return err
		}
		c.maxProcsUndoFn = undo
	}

	if c.config.GoMemLimit.Enable {
		// TODO: set logger bridge
		fmt.Println("setting go mem limit")
		_, err := memlimit.SetGoMemLimitWithOpts(memlimit.WithRatio(c.config.GoMemLimit.Ratio))
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *cgroupRuntimeExtension) Shutdown(_ context.Context) error {
	if c.maxProcsUndoFn != nil {
		c.maxProcsUndoFn()
	}

	return nil
}
