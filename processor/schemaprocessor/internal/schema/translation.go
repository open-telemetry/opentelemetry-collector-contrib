// Copyright  The OpenTelemetry Authors
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

package schema

import (
	"sync"

	"go.opentelemetry.io/otel/schema/v1.0/ast"
)

type translation struct {
	rw sync.RWMutex

	min, max *Version
}

func newTranslation(max *Version) *translation {
	return &translation{
		min: &Version{0, 0, 0},
		max: &Version{max.Major, max.Minor, max.Patch},
	}
}

var _ Schema = (*translation)(nil)

func (t *translation) SupportedVersion(version *Version) bool {
	t.rw.RLock()
	defer t.rw.RUnlock()

	// This implementation will change to check if the version is known
	// for the time being
	return version.Compare(t.min) >= 0 && version.Compare(t.max) <= 0
}

func (t *translation) Merge(content *ast.Schema) error {
	t.rw.Lock()
	defer t.rw.Unlock()
	// Implementation to come
	return nil
}
