// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadog // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog"

import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/featuregates"

// MetricRemappingDisabledFeatureGate is re-exported for backward compatibility with DataDog agent code
var MetricRemappingDisabledFeatureGate = featuregates.MetricRemappingDisabledFeatureGate
