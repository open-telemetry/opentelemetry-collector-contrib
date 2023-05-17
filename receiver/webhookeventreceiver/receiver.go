// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package webhookeventreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/webhookeventreceiver"

import (
	"errors"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

func newLogsReceiver(params receiver.CreateSettings, cfg Config, consumer consumer.Logs) (receiver.Logs, error) {
	return nil, errors.New("unimplemented")
}
