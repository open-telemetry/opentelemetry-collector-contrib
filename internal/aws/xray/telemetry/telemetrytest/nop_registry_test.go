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

package telemetrytest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray/telemetry"
)

func TestNopRegistry(t *testing.T) {
	assert.Same(t, nopRegistryInstance, NewNopRegistry())
	r := NewNopRegistry()
	assert.NotPanics(t, func() {
		recorder := r.Register(component.NewID("a"), telemetry.Config{}, nil)
		assert.Same(t, recorder, r.Load(component.NewID("b")))
		r.LoadOrStore(component.NewID("c"), recorder)
		r.LoadOrNop(component.NewID("d"))
	})
}
