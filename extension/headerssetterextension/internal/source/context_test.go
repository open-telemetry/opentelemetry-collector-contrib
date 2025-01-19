// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
