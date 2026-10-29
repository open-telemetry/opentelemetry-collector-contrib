// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package activedirectoryinvreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/activedirectoryinvreceiver"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/activedirectoryinvreceiver/internal/metadata"
)

var errReceiverNotSupported = fmt.Errorf("the '%s' receiver is only supported on Windows", metadata.Type)

func createLogsReceiver(
	_ context.Context,
	_ receiver.Settings,
	_ component.Config,
	_ consumer.Logs,
) (receiver.Logs, error) {
	return nil, errReceiverNotSupported
}
