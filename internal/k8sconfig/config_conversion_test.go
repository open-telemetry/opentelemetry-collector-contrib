// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sconfig

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig/k8sconfigtypes"
)

// Keep the dependency-light copy aligned with the original configuration while
// preserving the original defined types for existing consumers.
func TestAPIConfigTypesMatch(t *testing.T) {
	original := reflect.TypeFor[APIConfig]()
	lightweight := reflect.TypeFor[k8sconfigtypes.APIConfig]()
	assert.Equal(t, original.NumField(), lightweight.NumField(),
		"keep APIConfig fields in k8sconfig and k8sconfigtypes aligned; update FromConfigTypes and its tests when adding a field")

	for originalField := range original.Fields() {
		t.Run(originalField.Name, func(t *testing.T) {
			lightweightField, ok := lightweight.FieldByName(originalField.Name)
			require.True(t, ok, "add the matching field to k8sconfigtypes.APIConfig and update FromConfigTypes and its tests")

			expectedType := originalField.Type
			if expectedType == reflect.TypeFor[AuthType]() {
				expectedType = reflect.TypeFor[k8sconfigtypes.AuthType]()
			}
			assert.Equal(t, expectedType, lightweightField.Type,
				"keep APIConfig field types aligned and update FromConfigTypes and its tests")
			assert.Equal(t, originalField.Tag, lightweightField.Tag,
				"keep APIConfig struct tags aligned so both types use the same configuration keys")
		})
	}
}

func TestAuthTypesMatch(t *testing.T) {
	tests := []struct {
		name        string
		original    AuthType
		lightweight k8sconfigtypes.AuthType
	}{
		{"none", AuthTypeNone, k8sconfigtypes.AuthTypeNone},
		{"serviceAccount", AuthTypeServiceAccount, k8sconfigtypes.AuthTypeServiceAccount},
		{"kubeConfig", AuthTypeKubeConfig, k8sconfigtypes.AuthTypeKubeConfig},
		{"tls", AuthTypeTLS, k8sconfigtypes.AuthTypeTLS},
	}
	assert.Len(t, authTypes, len(tests),
		"update both authentication definitions and this compatibility table when supported auth types change")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, string(tt.original), string(tt.lightweight),
				"keep matching authentication constants in k8sconfig and k8sconfigtypes aligned")
			assert.True(t, authTypes[tt.original], "keep the compatibility table aligned with supported authentication types")
			assert.NoError(t, (k8sconfigtypes.APIConfig{AuthType: tt.lightweight}).Validate())
		})
	}
}

func TestFromConfigTypes(t *testing.T) {
	for _, authType := range []k8sconfigtypes.AuthType{
		k8sconfigtypes.AuthTypeNone,
		k8sconfigtypes.AuthTypeServiceAccount,
		k8sconfigtypes.AuthTypeKubeConfig,
		k8sconfigtypes.AuthTypeTLS,
	} {
		t.Run(string(authType), func(t *testing.T) {
			cfg := k8sconfigtypes.APIConfig{
				AuthType:     authType,
				Context:      "custom-context",
				KubeAPIQPS:   25.5,
				KubeAPIBurst: 50,
			}
			converted := FromConfigTypes(cfg)
			assert.Equal(t, APIConfig{
				AuthType:     AuthType(authType),
				Context:      "custom-context",
				KubeAPIQPS:   25.5,
				KubeAPIBurst: 50,
			}, converted)
			assert.NoError(t, converted.Validate())
		})
	}
}

func TestFromConfigTypesPreservesValidation(t *testing.T) {
	for _, cfg := range []k8sconfigtypes.APIConfig{
		{},
		{AuthType: "invalid"},
		{AuthType: k8sconfigtypes.AuthTypeServiceAccount},
		{AuthType: k8sconfigtypes.AuthTypeServiceAccount, KubeAPIQPS: -1},
		{AuthType: k8sconfigtypes.AuthTypeServiceAccount, KubeAPIBurst: -1},
	} {
		assert.Equal(t, cfg.Validate(), FromConfigTypes(cfg).Validate())
	}
}
