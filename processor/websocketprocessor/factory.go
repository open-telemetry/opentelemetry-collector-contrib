// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package websocketprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/websocketprocessor"

import (
	"go.opentelemetry.io/collector/processor"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/websocketprocessor/internal/metadata"
)

func NewFactory() processor.Factory {
	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
	)
}
