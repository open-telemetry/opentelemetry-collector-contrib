// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//go:build !linux

package systemdreceiver

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pmetric"
)

// Send empty metrics on anything that is not linux
func (s *systemdReceiver) scrape(_ context.Context) (pmetric.Metrics, error) {
	return pmetric.NewMetrics(), nil
}
