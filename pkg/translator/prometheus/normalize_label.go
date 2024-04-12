// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheus // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"

import (
	"fmt"
	"strings"
	"unicode"

	"go.opentelemetry.io/collector/featuregate"
)

var dropSanitizationGate = mustRegisterOrLoadGate(
	"pkg.translator.prometheus.PermissiveLabelSanitization",
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("Controls whether to change labels starting with '_' to 'key_'."),
	featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/8950"),
)

// mustRegisterOrLoadGate() is a temporary workaround for a featuregate collision.
// Remove this after https://github.com/open-telemetry/opentelemetry-collector/issues/9859 is fixed.
func mustRegisterOrLoadGate(id string, stage featuregate.Stage, opts ...featuregate.RegisterOption) *featuregate.Gate {

	// Try to register
	gate, err := featuregate.GlobalRegistry().Register(
		id,
		stage,
		opts...,
	)

	// If registration failed, try to find existing gate
	if err != nil {
		featuregate.GlobalRegistry().VisitAll(func(g *featuregate.Gate) {
			if g.ID() == id {
				gate = g
			}
		})
	}

	// If gate is still nil, panic() like MustRegister() would
	if gate == nil {
		panic(fmt.Sprintf("failed to register or load feature gate %s", id))
	}

	return gate
}

// Normalizes the specified label to follow Prometheus label names standard
//
// See rules at https://prometheus.io/docs/concepts/data_model/#metric-names-and-labels
//
// Labels that start with non-letter rune will be prefixed with "key_"
//
// Exception is made for double-underscores which are allowed
func NormalizeLabel(label string) string {

	// Trivial case
	if len(label) == 0 {
		return label
	}

	// Replace all non-alphanumeric runes with underscores
	label = strings.Map(sanitizeRune, label)

	// If label starts with a number, prepend with "key_"
	if unicode.IsDigit(rune(label[0])) {
		label = "key_" + label
	} else if strings.HasPrefix(label, "_") && !strings.HasPrefix(label, "__") && !dropSanitizationGate.IsEnabled() {
		label = "key" + label
	}

	return label
}

// Return '_' for anything non-alphanumeric
func sanitizeRune(r rune) rune {
	if unicode.IsLetter(r) || unicode.IsDigit(r) {
		return r
	}
	return '_'
}
