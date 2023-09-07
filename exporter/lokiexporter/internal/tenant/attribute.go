// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tenant // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/lokiexporter/internal/tenant"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/plog"
)

var _ Source = (*AttributeTenantSource)(nil)

type AttributeTenantSource struct {
	Value string
}

func (ts *AttributeTenantSource) GetTenant(_ context.Context, logs plog.Logs) (string, error) {
	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		rl := logs.ResourceLogs().At(i)
		if v, found := rl.Resource().Attributes().Get(ts.Value); found {
			return v.Str(), nil
		}
	}
	return "", nil
}
