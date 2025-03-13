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
	attrs map[string]any
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

func TestAttributeSourceSuccessString(t *testing.T) {
	ts := &AttributeSource{Key: "X-Scope-OrgID"}
	cl := client.FromContext(context.Background())
	cl.Auth = authData{attrs: map[string]any{"X-Scope-OrgID": "acme"}}
	ctx := client.NewContext(context.Background(), cl)

	val, err := ts.Get(ctx)

	assert.NoError(t, err)
	assert.Equal(t, "acme", val)
}

func TestAttributeSourceSuccessStruct(t *testing.T) {
	ts := &AttributeSource{Key: "X-Scope-OrgID"}
	cl := client.FromContext(context.Background())
	cl.Auth = authData{attrs: map[string]any{"X-Scope-OrgID": struct {
		Foo string
	}{
		Foo: "bar",
	}}}
	ctx := client.NewContext(context.Background(), cl)

	val, err := ts.Get(ctx)

	assert.NoError(t, err)
	assert.Equal(t, "{\"Foo\":\"bar\"}", val)
}

func TestAttributeSourceNotFound(t *testing.T) {
	ts := &AttributeSource{Key: "X-Scope-OrgID"}
	cl := client.FromContext(context.Background())
	cl.Auth = authData{attrs: map[string]any{"Not-Scope-OrgID": "acme"}}
	ctx := client.NewContext(context.Background(), cl)

	val, err := ts.Get(ctx)

	assert.NoError(t, err)
	assert.Empty(t, val)
}
