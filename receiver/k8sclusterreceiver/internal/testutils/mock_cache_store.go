// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testutils // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/testutils"

import (
	"errors"

	"k8s.io/client-go/tools/cache"
)

type MockStore struct {
	cache.Store
	WantErr bool
	Cache   map[string]any
}

func (ms *MockStore) GetByKey(id string) (any, bool, error) {
	if ms.WantErr {
		return nil, false, errors.New("")
	}
	item, exits := ms.Cache[id]
	return item, exits, nil
}

func (ms *MockStore) List() []any {
	out := make([]any, len(ms.Cache))
	i := 0
	for _, item := range ms.Cache {
		out[i] = item
		i++
	}
	return out
}
