// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package source

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/client"
)

type authData struct {
	attrs map[string]string
}

func (a authData) GetAttribute(s string) any {
	return a.attrs[s]
}

func (a authData) GetAttributeNames() []string {
	keys := make([]string, len(a.attrs))
	for key := range a.attrs {
		keys = append(keys, key)
	}
	return keys
}

func TestAttributeSourceSuccess(t *testing.T) {
	ts := &AttributeSource{Key: "X-Scope-OrgID"}
	cl := client.FromContext(context.Background())
	cl.Auth = authData{attrs: map[string]string{"X-Scope-OrgID": "acme"}}
	ctx := client.NewContext(context.Background(), cl)

	header, err := ts.Get(ctx)

	assert.NoError(t, err)
	assert.Equal(t, "acme", header)
}

func TestAttributeSourceNotFound(t *testing.T) {
	ts := &AttributeSource{Key: "X-Scope-OrgID"}
	cl := client.FromContext(context.Background())
	cl.Auth = authData{attrs: map[string]string{"Not-Scope-OrgID": "acme"}}
	ctx := client.NewContext(context.Background(), cl)

	header, err := ts.Get(ctx)

	assert.NoError(t, err)
	assert.Empty(t, header)
}
