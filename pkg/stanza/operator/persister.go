// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package operator // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"

import (
	"context"
	"fmt"
)

// Persister is an interface used to persist data
type Persister interface {
	Get(context.Context, string) ([]byte, error)
	Set(context.Context, string, []byte) error
	Delete(context.Context, string) error
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
