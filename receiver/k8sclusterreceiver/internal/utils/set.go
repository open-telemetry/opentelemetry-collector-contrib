// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package utils // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/utils"

import "go.opentelemetry.io/collector/featuregate"

var EnableStableMetrics = featuregate.GlobalRegistry().MustRegister(
	"semconv.k8s.enableStable",
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("When enabled, semconv stable metrics are enabled."),
	featuregate.WithRegisterFromVersion("v0.139.0"),
)

var DisableLegacyMetrics = featuregate.GlobalRegistry().MustRegister(
	"semconv.k8s.disableLegacy",
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("When enabled, semconv legacy metrics are disabled."),
	featuregate.WithRegisterFromVersion("v0.139.0"),
)

// StringSliceToMap converts a slice of strings into a map with keys from the slice
func StringSliceToMap(strings []string) map[string]bool {
	ret := map[string]bool{}
	for _, s := range strings {
		ret[s] = true
	}
	return ret
}
