// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translation // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/translation"

import _ "embed"

//go:embed default_metrics.yaml
var DefaultExcludeMetricsYaml string

//go:embed default_translation_rules.yaml
var DefaultTranslationRulesYaml string
