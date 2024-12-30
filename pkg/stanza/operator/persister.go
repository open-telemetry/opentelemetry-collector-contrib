// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package operator // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/extension/experimental/storage"
)

// Persister is an interface used to persist data
type Persister interface {
	Get(context.Context, string) ([]byte, error)
	Set(context.Context, string, []byte) error
	Delete(context.Context, string) error
	Batch(ctx context.Context, ops ...storage.Operation) error
}

type scopedPersister struct {
	Persister
	scope string
}

func NewScopedPersister(s string, p Persister) Persister {
	return &scopedPersister{
		Persister: p,
		scope:     s,
	}
}

func (p scopedPersister) Get(ctx context.Context, key string) ([]byte, error) {
	return p.Persister.Get(ctx, fmt.Sprintf("%s.%s", p.scope, key))
}

func (p scopedPersister) Set(ctx context.Context, key string, value []byte) error {
	return p.Persister.Set(ctx, fmt.Sprintf("%s.%s", p.scope, key), value)
}

func (p scopedPersister) Delete(ctx context.Context, key string) error {
	return p.Persister.Delete(ctx, fmt.Sprintf("%s.%s", p.scope, key))
}

func (p scopedPersister) Batch(ctx context.Context, ops ...storage.Operation) error {
	for _, op := range ops {
		op.Key = fmt.Sprintf("%s.%s", p.scope, op.Key)
	}
	return p.Persister.Batch(ctx, ops...)
}
