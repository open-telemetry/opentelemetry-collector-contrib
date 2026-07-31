// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sdnotifyextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/sdnotifyextension"

import (
	"context"
	"os"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensioncapabilities"
	"go.uber.org/zap"
)

type sdnotify struct {
	cfg    *Config
	logger *zap.Logger
	host   component.Host

	sigCh chan os.Signal
}

// Extension is the union of capability interfaces sdnotify implements.
type Extension interface {
	extension.Extension
	extensioncapabilities.PipelineWatcher
}

var _ Extension = (*sdnotify)(nil)

func newSDNotify(cfg *Config, logger *zap.Logger) *sdnotify {
	return &sdnotify{
		cfg:    cfg,
		logger: logger,
		sigCh:  make(chan os.Signal, 1),
	}
}

func (s *sdnotify) Start(_ context.Context, host component.Host) error {
	s.host = host
	return nil
}

func (*sdnotify) Shutdown(_ context.Context) error {
	return nil
}

func (*sdnotify) Ready() error {
	return nil
}

func (*sdnotify) NotReady() error {
	return nil
}
