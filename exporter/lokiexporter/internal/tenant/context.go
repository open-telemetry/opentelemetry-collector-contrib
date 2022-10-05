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
