// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package targetallocatorextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/targetallocatorextension"
import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

type taExtension struct{}

func (t taExtension) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (t taExtension) Shutdown(_ context.Context) error {
	return nil
}

func newTargetAllocatorExtension(_ *Config, _ *zap.Logger) taExtension {
	return taExtension{}
}
