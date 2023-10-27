// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package operator // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/extension/experimental/storage"
)

type ScopedStorage struct {
	client storage.Client
	scope  string
}

func NewScopedStorage(s string, client storage.Client) storage.Client {
	return &ScopedStorage{
		client: client,
		scope:  s,
	}
}

func (p ScopedStorage) Get(ctx context.Context, key string) ([]byte, error) {
	return p.client.Get(ctx, fmt.Sprintf("%s.%s", p.scope, key))
}
func (p ScopedStorage) Set(ctx context.Context, key string, value []byte) error {
	return p.client.Set(ctx, fmt.Sprintf("%s.%s", p.scope, key), value)
}
func (p ScopedStorage) Delete(ctx context.Context, key string) error {
	return p.client.Delete(ctx, fmt.Sprintf("%s.%s", p.scope, key))
}
func (p ScopedStorage) Batch(ctx context.Context, ops ...storage.Operation) error {
	return p.client.Batch(ctx, ops...)
}
func (p ScopedStorage) Close(ctx context.Context) error {
	return p.client.Close(ctx)
}
