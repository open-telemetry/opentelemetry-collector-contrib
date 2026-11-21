// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig/k8sconfigtypes"
)

func TestCheckInClusterEnv(t *testing.T) {
	tests := []struct {
		name     string
		authType k8sconfigtypes.AuthType
		host     string
		port     string
		wantErr  error
	}{
		{name: "service account in a pod", authType: k8sconfigtypes.AuthTypeServiceAccount, host: "10.96.0.1", port: "443"},
		{name: "service account outside a pod", authType: k8sconfigtypes.AuthTypeServiceAccount, wantErr: ErrNotInKubernetes},
		{name: "service account without port", authType: k8sconfigtypes.AuthTypeServiceAccount, host: "10.96.0.1", wantErr: ErrNotInKubernetes},
		{name: "none outside a pod", authType: k8sconfigtypes.AuthTypeNone, wantErr: ErrNotInKubernetes},
		{name: "tls outside a pod", authType: k8sconfigtypes.AuthTypeTLS, wantErr: ErrNotInKubernetes},
		{name: "kubeconfig outside a pod", authType: k8sconfigtypes.AuthTypeKubeConfig},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("KUBERNETES_SERVICE_HOST", tt.host)
			t.Setenv("KUBERNETES_SERVICE_PORT", tt.port)
			err := CheckInClusterEnv(k8sconfigtypes.APIConfig{AuthType: tt.authType})
			if tt.wantErr == nil {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}
