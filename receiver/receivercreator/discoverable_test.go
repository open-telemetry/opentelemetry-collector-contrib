// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package receivercreator

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	rcvr "go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap/zaptest"
)

// targetsConfig models a receiver that keeps its targets somewhere other than the
// top-level endpoint field, which is the case the generic validation cannot cover.
type targetsConfig struct {
	Endpoint string   `mapstructure:"endpoint"`
	Targets  []string `mapstructure:"targets"`
}

func (*targetsConfig) ValidateForDiscovery(rawCfg map[string]any, discoveredEndpoint string) error {
	targets, ok := rawCfg["targets"].([]any)
	if !ok {
		return nil
	}
	for _, target := range targets {
		if target != discoveredEndpoint {
			return fmt.Errorf("target %v is not the discovered endpoint", target)
		}
	}
	return nil
}

type targetsFactory struct {
	rcvr.Factory
}

func (*targetsFactory) CreateDefaultConfig() component.Config {
	return &targetsConfig{}
}

func (*targetsFactory) Type() component.Type {
	return component.MustNewType("targets")
}

func TestValidateEndpointConfig(t *testing.T) {
	tests := []struct {
		name       string
		rawCfg     userConfigMap
		discovered string
		expectErr  string
	}{
		{
			name:       "endpoint matches discovered endpoint",
			rawCfg:     userConfigMap{"endpoint": "http://1.2.3.4:8080/metrics"},
			discovered: "1.2.3.4:8080",
		},
		{
			name:       "endpoint targets another host",
			rawCfg:     userConfigMap{"endpoint": "http://evil.example/metrics"},
			discovered: "1.2.3.4:8080",
			expectErr:  "configured endpoint should include target Pod's endpoint",
		},
		{
			name:       "host smuggled into the query string",
			rawCfg:     userConfigMap{"endpoint": "http://evil.example/metrics?x=1.2.3.4:8080"},
			discovered: "1.2.3.4:8080",
			expectErr:  "configured endpoint should include target Pod's endpoint",
		},
		{
			name:       "no endpoint key cannot be validated",
			rawCfg:     userConfigMap{"config": map[string]any{}},
			discovered: "1.2.3.4:8080",
		},
		{
			name:       "empty endpoint",
			rawCfg:     userConfigMap{"endpoint": ""},
			discovered: "1.2.3.4:8080",
		},
		{
			name:       "non-string endpoint",
			rawCfg:     userConfigMap{"endpoint": 8080},
			discovered: "1.2.3.4:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEndpointConfig(tt.rawCfg, tt.discovered)
			if tt.expectErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.expectErr)
		})
	}
}

func TestValidateDiscoveredConfigDispatchesToDiscoverable(t *testing.T) {
	run := &receiverRunner{logger: zaptest.NewLogger(t)}

	// The generic validation would accept this, since the foreign host is not in
	// the endpoint field.
	err := run.validateDiscoveredConfig(&targetsFactory{}, userConfigMap{
		"targets": []any{"evil.example:80"},
	}, "1.2.3.4:8080")
	require.ErrorContains(t, err, "target evil.example:80 is not the discovered endpoint")

	require.NoError(t, run.validateDiscoveredConfig(&targetsFactory{}, userConfigMap{
		"targets": []any{"1.2.3.4:8080"},
	}, "1.2.3.4:8080"))
}

func TestValidateDiscoveredConfigWithoutDiscoverable(t *testing.T) {
	run := &receiverRunner{logger: zaptest.NewLogger(t)}

	require.NoError(t, run.validateDiscoveredConfig(&nopWithEndpointFactory{},
		userConfigMap{"endpoint": "http://1.2.3.4:8080/metrics"}, "1.2.3.4:8080"))
	require.ErrorContains(t,
		run.validateDiscoveredConfig(&nopWithEndpointFactory{}, userConfigMap{"endpoint": "http://evil.example"}, "1.2.3.4:8080"),
		"configured endpoint should include target Pod's endpoint")
}

// A valid top-level endpoint must not stop the receiver's own validation from running,
// otherwise a target held in any other field goes unchecked.
func TestLoadRuntimeReceiverConfigValidatesHintsWithEndpointSet(t *testing.T) {
	run := &receiverRunner{logger: zaptest.NewLogger(t)}
	receiver := receiverConfig{
		id: component.MustNewIDWithName("targets", "1"),
		config: userConfigMap{
			"endpoint": "1.2.3.4:8080",
			"targets":  []any{"evil.example:80"},
		},
		fromHints:          true,
		discoveredEndpoint: "1.2.3.4:8080",
	}

	_, _, err := run.loadRuntimeReceiverConfig(&targetsFactory{}, receiver, userConfigMap{})
	require.ErrorContains(t, err, "invalid configuration from annotation hints")
	require.ErrorContains(t, err, "target evil.example:80 is not the discovered endpoint")
}

func TestLoadRuntimeReceiverConfigSkipsValidationForStaticTemplates(t *testing.T) {
	run := &receiverRunner{logger: zaptest.NewLogger(t)}
	receiver := receiverConfig{
		id: component.MustNewIDWithName("targets", "1"),
		config: userConfigMap{
			"targets": []any{"anything.example:80"},
		},
	}

	_, _, err := run.loadRuntimeReceiverConfig(&targetsFactory{}, receiver, userConfigMap{})
	require.NoError(t, err)
}
