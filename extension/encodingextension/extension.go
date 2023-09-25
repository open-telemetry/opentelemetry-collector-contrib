// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package encodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encodingextension"

import (
	"context"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/codec"
)

// Extension defines an extension registered marshalers and unmarshalers that can be used
// throughout the collector.
type Extension struct {
	config *Config
}

func (e *Extension) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (e *Extension) Shutdown(_ context.Context) error {
	return nil
}

// GetLogsCodec returns a registered log codec associated with the `codecType` identity.
func (e *Extension) GetLogsCodec(codecType string) codec.Log {
	return e.config.LogCodecs[codecType]
}
