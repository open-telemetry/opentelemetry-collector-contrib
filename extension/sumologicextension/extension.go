// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumologicextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/sumologicextension"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

type SumologicExtension struct {
}

const DefaultHeartbeatInterval = 15 * time.Second

func init() {
}

func newSumologicExtension(_ *Config, _ *zap.Logger, _ component.ID, _ string) (*SumologicExtension, error) {
	return &SumologicExtension{}, nil
}

func (se *SumologicExtension) Start(_ context.Context, _ component.Host) error {
	return nil
}

// Shutdown is invoked during service shutdown.
func (se *SumologicExtension) Shutdown(_ context.Context) error {
	return nil
}
