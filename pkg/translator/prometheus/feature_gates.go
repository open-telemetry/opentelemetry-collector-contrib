// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheus // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"

import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus/internal/metadata"

// DropSanitizationGate is the stable permissive label sanitization feature gate.
//
// Deprecated: Permissive label sanitization is permanently enabled.
var DropSanitizationGate = metadata.PkgTranslatorPrometheusPermissiveLabelSanitizationFeatureGate

// SpecCompliantUCUMUnitsGate controls whether to use spec-compliant UCUM unit codes.
var SpecCompliantUCUMUnitsGate = metadata.PkgPrometheusSpecCompliantUCUMUnitsFeatureGate
