// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opampextension

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/uuid"
	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/extension/extensiontest"
	semconv "go.opentelemetry.io/collector/semconv/v1.27.0"
)

func TestNewOpampAgent(t *testing.T) {
	cfg := createDefaultConfig()
	set := extensiontest.NewNopSettings()
	set.BuildInfo = component.BuildInfo{Version: "test version", Command: "otelcoltest"}
	o, err := newOpampAgent(cfg.(*Config), set)
	assert.NoError(t, err)
	assert.Equal(t, "otelcoltest", o.agentType)
	assert.Equal(t, "test version", o.agentVersion)
	assert.NotEmpty(t, o.instanceID.String())
	assert.True(t, o.capabilities.ReportsEffectiveConfig)
	assert.Empty(t, o.effectiveConfig)
	assert.Nil(t, o.agentDescription)
}

func TestNewOpampAgentAttributes(t *testing.T) {
	cfg := createDefaultConfig()
	set := extensiontest.NewNopSettings()
	set.BuildInfo = component.BuildInfo{Version: "test version", Command: "otelcoltest"}
	set.Resource.Attributes().PutStr(semconv.AttributeServiceName, "otelcol-distro")
	set.Resource.Attributes().PutStr(semconv.AttributeServiceVersion, "distro.0")
	set.Resource.Attributes().PutStr(semconv.AttributeServiceInstanceID, "f8999bc1-4c9b-4619-9bae-7f009d2411ec")
	o, err := newOpampAgent(cfg.(*Config), set)
	assert.NoError(t, err)
	assert.Equal(t, "otelcol-distro", o.agentType)
	assert.Equal(t, "distro.0", o.agentVersion)
	assert.Equal(t, "f8999bc1-4c9b-4619-9bae-7f009d2411ec", o.instanceID.String())
}

func TestCreateAgentDescription(t *testing.T) {
	hostname, err := os.Hostname()
	require.NoError(t, err)

	serviceName := "otelcol-distrot"
	serviceVersion := "distro.0"
	serviceInstanceUUID := "f8999bc1-4c9b-4619-9bae-7f009d2411ec"

	testCases := []struct {
		name string
		cfg  func(*Config)

		expected *protobufs.AgentDescription
	}{
		{
			name: "No extra attributes",
			cfg:  func(_ *Config) {},
			expected: &protobufs.AgentDescription{
				IdentifyingAttributes: []*protobufs.KeyValue{
					stringKeyValue(semconv.AttributeServiceInstanceID, serviceInstanceUUID),
					stringKeyValue(semconv.AttributeServiceName, serviceName),
					stringKeyValue(semconv.AttributeServiceVersion, serviceVersion),
				},
				NonIdentifyingAttributes: []*protobufs.KeyValue{
					stringKeyValue(semconv.AttributeHostArch, runtime.GOARCH),
					stringKeyValue(semconv.AttributeHostName, hostname),
					stringKeyValue(semconv.AttributeOSType, runtime.GOOS),
				},
			},
		},
		{
			name: "Extra attributes specified",
			cfg: func(c *Config) {
				c.AgentDescription.NonIdentifyingAttributes = map[string]string{
					"env":                       "prod",
					semconv.AttributeK8SPodName: "my-very-cool-pod",
				}
			},
			expected: &protobufs.AgentDescription{
				IdentifyingAttributes: []*protobufs.KeyValue{
					stringKeyValue(semconv.AttributeServiceInstanceID, serviceInstanceUUID),
					stringKeyValue(semconv.AttributeServiceName, serviceName),
					stringKeyValue(semconv.AttributeServiceVersion, serviceVersion),
				},
				NonIdentifyingAttributes: []*protobufs.KeyValue{
					stringKeyValue("env", "prod"),
					stringKeyValue(semconv.AttributeHostArch, runtime.GOARCH),
					stringKeyValue(semconv.AttributeHostName, hostname),
					stringKeyValue(semconv.AttributeK8SPodName, "my-very-cool-pod"),
					stringKeyValue(semconv.AttributeOSType, runtime.GOOS),
				},
			},
		},
		{
			name: "Extra attributes override",
			cfg: func(c *Config) {
				c.AgentDescription.NonIdentifyingAttributes = map[string]string{
					semconv.AttributeHostName: "override-host",
				}
			},
			expected: &protobufs.AgentDescription{
				IdentifyingAttributes: []*protobufs.KeyValue{
					stringKeyValue(semconv.AttributeServiceInstanceID, serviceInstanceUUID),
					stringKeyValue(semconv.AttributeServiceName, serviceName),
					stringKeyValue(semconv.AttributeServiceVersion, serviceVersion),
				},
				NonIdentifyingAttributes: []*protobufs.KeyValue{
					stringKeyValue(semconv.AttributeHostArch, runtime.GOARCH),
					stringKeyValue(semconv.AttributeHostName, "override-host"),
					stringKeyValue(semconv.AttributeOSType, runtime.GOOS),
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			cfg := createDefaultConfig().(*Config)
			tc.cfg(cfg)

			set := extensiontest.NewNopSettings()
			set.Resource.Attributes().PutStr(semconv.AttributeServiceName, serviceName)
			set.Resource.Attributes().PutStr(semconv.AttributeServiceVersion, serviceVersion)
			set.Resource.Attributes().PutStr(semconv.AttributeServiceInstanceID, serviceInstanceUUID)

			o, err := newOpampAgent(cfg, set)
			require.NoError(t, err)
			assert.Nil(t, o.agentDescription)

			err = o.createAgentDescription()
			assert.NoError(t, err)
			require.Equal(t, tc.expected, o.agentDescription)
		})
	}
}

func TestUpdateAgentIdentity(t *testing.T) {
	cfg := createDefaultConfig()
	set := extensiontest.NewNopSettings()
	o, err := newOpampAgent(cfg.(*Config), set)
	assert.NoError(t, err)

	olduid := o.instanceID
	assert.NotEmpty(t, olduid.String())

	uid := uuid.Must(uuid.NewV7())
	assert.NotEqual(t, uid, olduid)

	o.updateAgentIdentity(uid)
	assert.Equal(t, o.instanceID, uid)
}

func TestComposeEffectiveConfig(t *testing.T) {
	cfg := createDefaultConfig()
	set := extensiontest.NewNopSettings()
	o, err := newOpampAgent(cfg.(*Config), set)
	assert.NoError(t, err)
	assert.Empty(t, o.effectiveConfig)

	ec := o.composeEffectiveConfig()
	assert.Nil(t, ec)

	ecFileName := filepath.Join("testdata", "effective.yaml")
	cm, err := confmaptest.LoadConf(ecFileName)
	assert.NoError(t, err)
	expected, err := os.ReadFile(ecFileName)
	assert.NoError(t, err)

	o.updateEffectiveConfig(cm)
	ec = o.composeEffectiveConfig()
	assert.NotNil(t, ec)
	assert.YAMLEq(t, string(expected), string(ec.ConfigMap.ConfigMap[""].Body))
}

func TestShutdown(t *testing.T) {
	cfg := createDefaultConfig()
	set := extensiontest.NewNopSettings()
	o, err := newOpampAgent(cfg.(*Config), set)
	assert.NoError(t, err)

	// Shutdown with no OpAMP client
	assert.NoError(t, o.Shutdown(context.TODO()))
}

func TestStart(t *testing.T) {
	cfg := createDefaultConfig()
	set := extensiontest.NewNopSettings()
	o, err := newOpampAgent(cfg.(*Config), set)
	assert.NoError(t, err)

	assert.NoError(t, o.Start(context.TODO(), componenttest.NewNopHost()))
	assert.NoError(t, o.Shutdown(context.TODO()))
}

func TestParseInstanceIDString(t *testing.T) {
	testCases := []struct {
		name         string
		in           string
		expectedUUID uuid.UUID
		expectedErr  string
	}{
		{
			name:         "Parses ULID",
			in:           "7RK6DW2K4V8RCSQBKZ02EJ84FC",
			expectedUUID: uuid.MustParse("f8999bc1-4c9b-4619-9bae-7f009d2411ec"),
		},
		{
			name:         "Parses UUID",
			in:           "f8999bc1-4c9b-4619-9bae-7f009d2411ec",
			expectedUUID: uuid.MustParse("f8999bc1-4c9b-4619-9bae-7f009d2411ec"),
		},
		{
			name:        "Fails on invalid format",
			in:          "not-a-valid-id",
			expectedErr: "invalid UUID length: 14\nulid: bad data size when unmarshaling",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			id, err := parseInstanceIDString(tc.in)
			if tc.expectedErr != "" {
				require.ErrorContains(t, err, tc.expectedErr)
				return
			}

			require.Equal(t, tc.expectedUUID, id)
		})
	}
}

func TestOpAMPAgent_Dependencies(t *testing.T) {
	t.Run("No server specified", func(t *testing.T) {
		o := opampAgent{
			cfg: &Config{},
		}

		require.Nil(t, o.Dependencies())
	})

	t.Run("No auth extension specified", func(t *testing.T) {
		o := opampAgent{
			cfg: &Config{
				Server: &OpAMPServer{
					WS: &commonFields{},
				},
			},
		}

		require.Nil(t, o.Dependencies())
	})

	t.Run("auth extension specified", func(t *testing.T) {
		authID := component.MustNewID("basicauth")
		o := opampAgent{
			cfg: &Config{
				Server: &OpAMPServer{
					WS: &commonFields{
						Auth: authID,
					},
				},
			},
		}

		require.Equal(t, []component.ID{authID}, o.Dependencies())
	})
}
