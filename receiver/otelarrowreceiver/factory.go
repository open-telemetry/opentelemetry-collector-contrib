// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelarrowreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otelarrowreceiver"

import (
	"github.com/open-telemetry/otel-arrow/collector/receiver/otelarrowreceiver"
	"go.opentelemetry.io/collector/receiver"
)

// NewFactory creates a new OTLP receiver factory.
func NewFactory() receiver.Factory {
	return otelarrowreceiver.NewFactory()
}
