// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translation // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/translation"

import _ "embed"

// DefaultExcludeMetricsYaml holds list of hard coded metrics that will added to the
// exclude list from the config. It includes non-default metrics collected by
// receivers. This list is determined by categorization of metrics in the SignalFx
// Agent. Metrics in the OpenTelemetry convention, that have equivalents in the
// SignalFx Agent that are categorized as non-default are also included in this list.
//
//go:embed default_metrics.yaml
var DefaultExcludeMetricsYaml string

// DefaultTranslationRulesYaml defines default translation rules that will be applied to metrics if
// config.TranslationRules not specified explicitly.
//
//go:embed default_translation_rules.yaml
var DefaultTranslationRulesYaml string
