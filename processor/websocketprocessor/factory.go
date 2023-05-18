// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package websocketprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/websocketprocessor"

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/processor"
)

const (
	typeStr   = "websocket"
	stability = component.StabilityLevelDevelopment
)

func NewFactory() processor.Factory {
	return processor.NewFactory(
		typeStr,
		createDefaultConfig,
	)
}
