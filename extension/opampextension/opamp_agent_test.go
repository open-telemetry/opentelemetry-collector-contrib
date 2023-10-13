// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opampextension

import (
	"context"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension/extensiontest"
	semconv "go.opentelemetry.io/collector/semconv/v1.18.0"
)

func TestNewOpampAgent(t *testing.T) {
	cfg := createDefaultConfig()
	set := extensiontest.NewNopCreateSettings()
	set.BuildInfo = component.BuildInfo{Version: "test version", Command: "otelcoltest"}
	o, err := newOpampAgent(cfg.(*Config), set.Logger, set.BuildInfo, set.Resource)
	assert.NoError(t, err)
	assert.Equal(t, "otelcoltest", o.agentType)
	assert.Equal(t, "test version", o.agentVersion)
	assert.NotEmpty(t, o.instanceID.String())
	assert.NotEmpty(t, o.effectiveConfig)
	assert.Nil(t, o.agentDescription)
}

func TestNewOpampAgentAttributes(t *testing.T) {
	cfg := createDefaultConfig()
	set := extensiontest.NewNopCreateSettings()
	set.BuildInfo = component.BuildInfo{Version: "test version", Command: "otelcoltest"}
	set.Resource.Attributes().PutStr(semconv.AttributeServiceName, "otelcol-distro")
	set.Resource.Attributes().PutStr(semconv.AttributeServiceVersion, "distro.0")
	set.Resource.Attributes().PutStr(semconv.AttributeServiceInstanceID, "f8999bc1-4c9b-4619-9bae-7f009d2411ec")
	o, err := newOpampAgent(cfg.(*Config), set.Logger, set.BuildInfo, set.Resource)
	assert.NoError(t, err)
	assert.Equal(t, "otelcol-distro", o.agentType)
	assert.Equal(t, "distro.0", o.agentVersion)
	assert.Equal(t, "7RK6DW2K4V8RCSQBKZ02EJ84FC", o.instanceID.String())
}

func TestCreateAgentDescription(t *testing.T) {
	cfg := createDefaultConfig()
	set := extensiontest.NewNopCreateSettings()
	o, err := newOpampAgent(cfg.(*Config), set.Logger, set.BuildInfo, set.Resource)
	assert.NoError(t, err)

	assert.Nil(t, o.agentDescription)
	err = o.createAgentDescription()
	assert.NoError(t, err)
	assert.NotNil(t, o.agentDescription)
}

func TestUpdateAgentIdentity(t *testing.T) {
	cfg := createDefaultConfig()
	set := extensiontest.NewNopCreateSettings()
	o, err := newOpampAgent(cfg.(*Config), set.Logger, set.BuildInfo, set.Resource)
	assert.NoError(t, err)

	olduid := o.instanceID
	assert.NotEmpty(t, olduid.String())

	uid := ulid.Make()
	assert.NotEqual(t, uid, olduid)

	o.updateAgentIdentity(uid)
	assert.Equal(t, o.instanceID, uid)
}

func TestComposeEffectiveConfig(t *testing.T) {
	cfg := createDefaultConfig()
	set := extensiontest.NewNopCreateSettings()
	o, err := newOpampAgent(cfg.(*Config), set.Logger, set.BuildInfo, set.Resource)
	assert.NoError(t, err)
	assert.NotEmpty(t, o.effectiveConfig)

	ec := o.composeEffectiveConfig()
	assert.NotNil(t, ec)
}

func TestShutdown(t *testing.T) {
	cfg := createDefaultConfig()
	set := extensiontest.NewNopCreateSettings()
	o, err := newOpampAgent(cfg.(*Config), set.Logger, set.BuildInfo, set.Resource)
	assert.NoError(t, err)

	// Shutdown with no OpAMP client
	assert.NoError(t, o.Shutdown(context.TODO()))
}

func TestStart(t *testing.T) {
	cfg := createDefaultConfig()
	set := extensiontest.NewNopCreateSettings()
	o, err := newOpampAgent(cfg.(*Config), set.Logger, set.BuildInfo, set.Resource)
	assert.NoError(t, err)

	assert.NoError(t, o.Start(context.TODO(), componenttest.NewNopHost()))
}
