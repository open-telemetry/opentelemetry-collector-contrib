// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package journaldreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/journaldreceiver"

import (
	"go.opentelemetry.io/collector/receiver"
)

// NewFactory creates a factory for journald receiver
func NewFactory() receiver.Factory {
	return newFactoryAdapter()
}
