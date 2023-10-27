// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package operator // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/extension/experimental/storage"
)

type ScopedPersister struct {
	client storage.Client
	scope  string
}

func NewScopedPersister(s string, client storage.Client) storage.Client {
	return &ScopedPersister{
		client: client,
		scope:  s,
	}
}

func (p ScopedPersister) Get(ctx context.Context, key string) ([]byte, error) {
	return p.client.Get(ctx, fmt.Sprintf("%s.%s", p.scope, key))
}
func (p ScopedPersister) Set(ctx context.Context, key string, value []byte) error {
	return p.client.Set(ctx, fmt.Sprintf("%s.%s", p.scope, key), value)
}
func (p ScopedPersister) Delete(ctx context.Context, key string) error {
	return p.client.Delete(ctx, fmt.Sprintf("%s.%s", p.scope, key))
}
