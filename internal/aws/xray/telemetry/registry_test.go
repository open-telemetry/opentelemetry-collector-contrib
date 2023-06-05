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

package telemetry

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
)

func TestRegistry(t *testing.T) {
	r := NewRegistry()
	newID := component.NewID("new")
	contribID := component.NewID("contrib")
	notCreatedID := component.NewID("not-created")
	original := r.Register(
		newID,
		Config{
			IncludeMetadata: false,
			Contributors:    []component.ID{contribID},
		},
		&mockClient{},
	)
	withSameID := r.Register(
		newID,
		Config{
			IncludeMetadata: true,
			Contributors:    []component.ID{notCreatedID},
		},
		&mockClient{},
		WithResourceARN("arn"),
	)
	// still the same recorder
	assert.Same(t, original, withSameID)
	// contributors have access to same recorder
	contrib := r.Load(contribID)
	assert.NotNil(t, contrib)
	assert.Same(t, original, contrib)
	// second attempt with same ID did not give contributors access
	assert.Nil(t, r.Load(notCreatedID))
	nop := r.LoadOrNop(notCreatedID)
	assert.NotNil(t, nop)
	assert.Equal(t, NewNopSender(), nop)
}

func TestGlobalRegistry(t *testing.T) {
	assert.Same(t, globalRegistry, GlobalRegistry())
}
