// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tenant // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/lokiexporter/internal/tenant"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/plog"
)

// LogsPerTenant holds a registry of plog.Logs per tenant
type LogsPerTenant map[string]plog.Logs

type Source interface {
	GetTenant(context.Context, plog.Logs) (string, error)
}
