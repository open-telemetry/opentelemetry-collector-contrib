// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package podmanreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/podmanreceiver"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

func newReceiver(
	_ context.Context,
	settings receiver.CreateSettings,
	config *Config,
	nextConsumer consumer.Metrics,
	clientFactory interface{},
) (receiver.Metrics, error) {
	return nil, fmt.Errorf("podman receiver is not supported on windows")
}
