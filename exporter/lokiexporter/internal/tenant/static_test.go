// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tenant // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/lokiexporter/internal/tenant"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/plog"
)

func TestStaticTenantSource(t *testing.T) {
	ts := &StaticTenantSource{Value: "acme"}
	tenant, err := ts.GetTenant(t.Context(), plog.NewLogs())
	assert.NoError(t, err)
	assert.Equal(t, "acme", tenant)
}
