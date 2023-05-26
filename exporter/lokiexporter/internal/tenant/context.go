// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tenant // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/lokiexporter/internal/tenant"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/pdata/plog"
)

var _ Source = (*ContextTenantSource)(nil)

type ContextTenantSource struct {
	Key string
}

func (ts *ContextTenantSource) GetTenant(ctx context.Context, logs plog.Logs) (string, error) {
	cl := client.FromContext(ctx)
	ss := cl.Metadata.Get(ts.Key)

	if len(ss) == 0 {
		return "", nil
	}

	if len(ss) > 1 {
		return "", fmt.Errorf("%d tenant keys found in the context, can't determine which one to use", len(ss))
	}

	return ss[0], nil
}
