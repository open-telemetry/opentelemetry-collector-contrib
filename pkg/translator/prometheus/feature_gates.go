// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheus // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"

import (
	"go.opentelemetry.io/collector/featuregate"
)

var (
	dropSanitizationGate = featuregate.GlobalRegistry().MustRegister(
		"pkg.translator.prometheus.PermissiveLabelSanitization",
		featuregate.StageAlpha,
		featuregate.WithRegisterDescription("Controls whether to change labels starting with '_' to 'key_'."),
		featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/8950"),
	)

	normalizeNameGate = featuregate.GlobalRegistry().MustRegister(
		"pkg.translator.prometheus.NormalizeName",
		featuregate.StageBeta,
		featuregate.WithRegisterDescription("Controls whether metrics names are automatically normalized to follow Prometheus naming convention. Attention: if 'pkg.translator.prometheus.allowUTF8' is enabled, UTF-8 characters will not be normalized."),
		featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/8950"),
	)

	// AllowUTF8FeatureGate could be a private variable, it is used mostly to toogle the
	// value of a global variable and it's intended usage is:
	/*
		import "github.com/prometheus/common/model"

		if AllowUTF8FeatureGate.IsEnabled() {
			model.NameValidationScheme = model.UTF8Validation
		}
	*/
	// We could do this with a init function in the translator package, but we can't guarantee that
	// the featuregate.GlobalRegistry is initialized before the init function here.
	//
	// Components that want to respect this feature gate behavior should check if it is enabled and
	// set the model.NameValidationScheme accordingly.
	//
	// WARNING: Since it's a global variable, if one component enables it then it works for all components
	// that are using this package.
	AllowUTF8FeatureGate = featuregate.GlobalRegistry().MustRegister(
		"pkg.translator.prometheus.allowUTF8",
		featuregate.StageAlpha,
		featuregate.WithRegisterDescription("When enabled, UTF-8 characters will not be translated to underscores."),
		featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/35459"),
		featuregate.WithRegisterFromVersion("v0.110.0"),
	)
)

func init() {

}
