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

package source

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/client"
)

func TestContextSourceSuccess(t *testing.T) {
	ts := &ContextSource{Key: "X-Scope-OrgID"}
	cl := client.FromContext(context.Background())
	cl.Metadata = client.NewMetadata(map[string][]string{"X-Scope-OrgID": {"acme"}})
	ctx := client.NewContext(context.Background(), cl)

	header, err := ts.Get(ctx)

	assert.NoError(t, err)
	assert.Equal(t, "acme", header)
}

func TestContextSourceNotFound(t *testing.T) {
	ts := &ContextSource{Key: "X-Scope-OrgID"}
	cl := client.FromContext(context.Background())
	cl.Metadata = client.NewMetadata(map[string][]string{"Not-Scope-OrgID": {"acme"}})
	ctx := client.NewContext(context.Background(), cl)

	header, err := ts.Get(ctx)

	assert.NoError(t, err)
	assert.Empty(t, header)
}

func TestContextSourceMultipleFound(t *testing.T) {
	ts := &ContextSource{Key: "X-Scope-OrgID"}
	cl := client.FromContext(context.Background())
	cl.Metadata = client.NewMetadata(map[string][]string{"X-Scope-OrgID": {"acme", "globex"}})
	ctx := client.NewContext(context.Background(), cl)

	header, err := ts.Get(ctx)

	assert.Error(t, err)
	assert.Empty(t, header)
}
