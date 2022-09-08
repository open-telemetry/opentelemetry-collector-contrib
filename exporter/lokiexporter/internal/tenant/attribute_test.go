// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	logs.ResourceLogs().At(0).Resource().Attributes().UpsertString("tenant.id", "acme")

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
	logs.ResourceLogs().At(0).Resource().Attributes().UpsertString("not.tenant.id", "acme")

	// test
	tenant, err := ts.GetTenant(context.Background(), logs)

	// verify
	assert.NoError(t, err)
	assert.Empty(t, tenant)
}
