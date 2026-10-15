// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sconfig // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"

import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig/k8sconfigtypes"

// FromConfigTypes converts dependency-light detector settings to the existing
// client configuration type without changing its identity for other components.
func FromConfigTypes(cfg k8sconfigtypes.APIConfig) APIConfig {
	return APIConfig{
		AuthType:     AuthType(cfg.AuthType),
		Context:      cfg.Context,
		KubeAPIQPS:   cfg.KubeAPIQPS,
		KubeAPIBurst: cfg.KubeAPIBurst,
	}
}
