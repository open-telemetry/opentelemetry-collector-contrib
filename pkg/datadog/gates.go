// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadog // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog"

import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/featuregates"

// MetricRemappingDisabledFeatureGate is re-exported for backward compatibility with DataDog agent code
//
// Deprecated [v0.138.0]: This type has been deprecated, use pkg/datadog/featuregates.MetricRemappingDisabledFeatureGate instead
var MetricRemappingDisabledFeatureGate = featuregates.MetricRemappingDisabledFeatureGate
