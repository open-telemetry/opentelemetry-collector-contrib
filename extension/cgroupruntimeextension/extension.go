// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cgroupruntimeextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/cgroupruntimeextension"

import (
	"context"

	"go.opentelemetry.io/collector/component"
)

type (
	undoFunc        func()
	runtimeModifier func() (undoFunc, error)
)

type cgroupRuntimeExtension struct {
	config *Config

	maxProcsFn     runtimeModifier
	undoMaxProcsFn undoFunc

	memLimitFn     runtimeModifier
	undoMemLimitFn undoFunc
}

func newCgroupRuntime(cfg *Config, maxProcsFn runtimeModifier, memLimitFn runtimeModifier) *cgroupRuntimeExtension {
	return &cgroupRuntimeExtension{
		config:     cfg,
		maxProcsFn: maxProcsFn,
		memLimitFn: memLimitFn,
	}
}

func (c *cgroupRuntimeExtension) Start(ctx context.Context, host component.Host) error {
	var err error
	if c.config.GoMaxProcs.Enable {
		c.undoMaxProcsFn, err = c.maxProcsFn()
		if err != nil {
			return err
		}
	}

	if c.config.GoMemLimit.Enable {
		c.undoMemLimitFn, err = c.memLimitFn()
		if err != nil {
			return err
		}
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
