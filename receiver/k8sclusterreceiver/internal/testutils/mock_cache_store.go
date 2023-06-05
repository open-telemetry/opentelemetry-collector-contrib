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

package testutils // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/testutils"

import (
	"errors"

	"k8s.io/client-go/tools/cache"
)

type MockStore struct {
	cache.Store
	WantErr bool
	Cache   map[string]interface{}
}

func (ms *MockStore) GetByKey(id string) (interface{}, bool, error) {
	if ms.WantErr {
		return nil, false, errors.New("")
	}
	item, exits := ms.Cache[id]
	return item, exits, nil
}

func (ms *MockStore) List() []interface{} {
	out := make([]interface{}, len(ms.Cache))
	i := 0
	for _, item := range ms.Cache {
		out[i] = item
		i++
	}
	return out
}
