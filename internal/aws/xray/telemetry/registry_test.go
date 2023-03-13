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

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"
)

func TestRegistry(t *testing.T) {
	registry := NewRegistry()
	newID := component.NewID("new")
	contribID := component.NewID("contrib")
	notCreatedID := component.NewID("not-created")
	original := registry.Register(
		newID,
		nil,
		nil,
		&Config{
			IncludeMetadata: false,
			Contributors:    []component.ID{contribID},
		},
		nil,
	).(*telemetryRecorder)
	assert.Empty(t, original.hostname)
	assert.Empty(t, original.instanceID)
	assert.Empty(t, original.resourceARN)
	withSameID := registry.Register(
		newID,
		nil,
		nil,
		&Config{
			IncludeMetadata: true,
			Contributors:    []component.ID{notCreatedID},
		},
		&awsutil.AWSSessionSettings{ResourceARN: "arn"},
	).(*telemetryRecorder)
	// still the same recorder
	assert.Same(t, original, withSameID)
	// contributors have access to same recorder
	contrib := registry.Get(contribID)
	assert.NotNil(t, contrib)
	assert.Same(t, original, contrib)
	// second attempt with same ID did not give contributors access
	assert.Nil(t, registry.Get(notCreatedID))
}

func TestGlobalRegistry(t *testing.T) {
	assert.Same(t, globalRegistry, GlobalRegistry())
}
