// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tenant // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/lokiexporter/internal/tenant"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/plog"
)

var _ Source = (*StaticTenantSource)(nil)

type StaticTenantSource struct {
	Value string
}

func (ts *StaticTenantSource) GetTenant(_ context.Context, _ plog.Logs) (string, error) {
	return ts.Value, nil
}
