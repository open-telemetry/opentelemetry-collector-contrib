// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tenant // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/lokiexporter/internal/tenant"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/pdata/plog"
)

func TestContextTenantSourceSuccess(t *testing.T) {
	// prepare
	ts := &ContextTenantSource{Key: "X-Scope-OrgID"}
	cl := client.FromContext(t.Context())
	cl.Metadata = client.NewMetadata(map[string][]string{"X-Scope-OrgID": {"acme"}})
	ctx := client.NewContext(t.Context(), cl)

	// test
	tenant, err := ts.GetTenant(ctx, plog.NewLogs())

	// verify
	assert.NoError(t, err)
	assert.Equal(t, "acme", tenant)
}

func TestContextTenantSourceNotFound(t *testing.T) {
	// prepare
	ts := &ContextTenantSource{Key: "X-Scope-OrgID"}
	cl := client.FromContext(t.Context())
	cl.Metadata = client.NewMetadata(map[string][]string{"Not-Scope-OrgID": {"acme"}})
	ctx := client.NewContext(t.Context(), cl)

	// test
	tenant, err := ts.GetTenant(ctx, plog.NewLogs())

	// verify
	assert.NoError(t, err)
	assert.Empty(t, tenant)
}

func TestContextTenantSourceMultipleFound(t *testing.T) {
	// prepare
	ts := &ContextTenantSource{Key: "X-Scope-OrgID"}
	cl := client.FromContext(t.Context())
	cl.Metadata = client.NewMetadata(map[string][]string{"X-Scope-OrgID": {"acme", "globex"}})
	ctx := client.NewContext(t.Context(), cl)

	// test
	tenant, err := ts.GetTenant(ctx, plog.NewLogs())

	// verify
	assert.Error(t, err)
	assert.Empty(t, tenant)
}
