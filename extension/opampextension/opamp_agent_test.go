// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package opampextension

import (
	"context"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension/extensiontest"
)

func TestNewOpampAgent(t *testing.T) {
	cfg := createDefaultConfig()
	set := extensiontest.NewNopCreateSettings()
	o, err := newOpampAgent(cfg.(*Config), set.Logger)
	assert.NoError(t, err)
	assert.NotEmpty(t, o.instanceId.String())
	assert.NotEmpty(t, o.effectiveConfig)
	assert.Nil(t, o.agentDescription)
}

func TestCreateAgentDescription(t *testing.T) {
	cfg := createDefaultConfig()
	set := extensiontest.NewNopCreateSettings()
	o, err := newOpampAgent(cfg.(*Config), set.Logger)
	assert.NoError(t, err)

	assert.Nil(t, o.agentDescription)
	o.createAgentDescription()
	assert.NotNil(t, o.agentDescription)
}

func TestUpdateAgentIdentity(t *testing.T) {
	cfg := createDefaultConfig()
	set := extensiontest.NewNopCreateSettings()
	o, err := newOpampAgent(cfg.(*Config), set.Logger)
	assert.NoError(t, err)

	olduid := o.instanceId
	assert.NotEmpty(t, olduid.String())

	uid := ulid.Make()
	assert.NotEqual(t, uid, olduid)

	o.updateAgentIdentity(uid)
	assert.Equal(t, o.instanceId, uid)
}

func TestComposeEffectiveConfig(t *testing.T) {
	cfg := createDefaultConfig()
	set := extensiontest.NewNopCreateSettings()
	o, err := newOpampAgent(cfg.(*Config), set.Logger)
	assert.NoError(t, err)
	assert.NotEmpty(t, o.effectiveConfig)

	ec := o.composeEffectiveConfig()
	assert.NotNil(t, ec)
}

func TestShutdown(t *testing.T) {
	cfg := createDefaultConfig()
	set := extensiontest.NewNopCreateSettings()
	o, err := newOpampAgent(cfg.(*Config), set.Logger)
	assert.NoError(t, err)

	// Shutdown with no OpAMP client
	assert.NoError(t, o.Shutdown(context.TODO()))
}

func TestStart(t *testing.T) {
	cfg := createDefaultConfig()
	set := extensiontest.NewNopCreateSettings()
	o, err := newOpampAgent(cfg.(*Config), set.Logger)
	assert.NoError(t, err)

	assert.NoError(t, o.Start(context.TODO(), componenttest.NewNopHost()))
}
