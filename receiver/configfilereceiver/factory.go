// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package configfilereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/configfilereceiver"

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver"
)

// NewFactory creates the configfile logs receiver factory.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		receiverTypeVal,
		createDefaultConfig,
		receiver.WithLogs(newLogsReceiver, component.StabilityLevelDevelopment),
	)
}
