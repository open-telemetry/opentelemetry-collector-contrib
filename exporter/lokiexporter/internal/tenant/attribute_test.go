// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tenant // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/lokiexporter/internal/tenant"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/plog"
)

func TestAttributeTenantSourceSuccess(t *testing.T) {
	// prepare
	ts := &AttributeTenantSource{Value: "tenant.id"}

	logs := plog.NewLogs()
	logs.ResourceLogs().AppendEmpty() // who's on first
	logs.ResourceLogs().AppendEmpty() // what's on second
	logs.ResourceLogs().At(0).Resource().Attributes().PutStr("tenant.id", "acme")

	// test
	tenant, err := ts.GetTenant(context.Background(), logs)

	// verify
	assert.NoError(t, err)
	assert.Equal(t, "acme", tenant)
}

func TestAttributeTenantSourceNotFound(t *testing.T) {
	// prepare
	ts := &AttributeTenantSource{Value: "tenant.id"}

	logs := plog.NewLogs()
	logs.ResourceLogs().AppendEmpty() // who's on first
	logs.ResourceLogs().AppendEmpty() // what's on second
	logs.ResourceLogs().At(0).Resource().Attributes().PutStr("not.tenant.id", "acme")

	// test
	tenant, err := ts.GetTenant(context.Background(), logs)

	// verify
	assert.NoError(t, err)
	assert.Empty(t, tenant)
}
