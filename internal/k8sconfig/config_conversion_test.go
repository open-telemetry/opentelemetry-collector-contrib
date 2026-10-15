// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig/k8sconfigtypes"
)

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
