// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheus // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"

import (
	"strings"
	"unicode"

	"github.com/prometheus/common/model"
	"go.opentelemetry.io/collector/featuregate"
)

var dropSanitizationGate = featuregate.GlobalRegistry().MustRegister(
	"pkg.translator.prometheus.PermissiveLabelSanitization",
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("Controls whether to change labels starting with '_' to 'key_'."),
	featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/8950"),
)

// Normalizes the specified label to follow Prometheus label names standard
//
// See rules at https://prometheus.io/docs/concepts/data_model/#metric-names-and-labels
//
// Labels that start with non-letter rune will be prefixed with "key_"
//
// Exception is made for double-underscores which are allowed
func NormalizeLabel(label string, allowUTF8 bool) string {

	// Trivial case
	if len(label) == 0 {
		return label
	}

	if allowUTF8 {
		return label
	}

	// If label starts with a number, prepend with "key_"
	if unicode.IsDigit(rune(label[0])) {
		label = "key_" + label
	} else if strings.HasPrefix(label, "_") && !strings.HasPrefix(label, "__") && !dropSanitizationGate.IsEnabled() {
		label = "key" + label
	}

	return model.EscapeName(label, model.UnderscoreEscaping)
}
