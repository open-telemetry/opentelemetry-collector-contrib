// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"

import "go.opentelemetry.io/collector/featuregate"

// useRequestTypeFeatureGate, when enabled, routes traces through a custom
// exporterhelper.Request implementation that holds pre-marshaled Kafka
// records. Staged rollout: Alpha (opt-in) → Beta (opt-out) → Stable.
// Tracking: opentelemetry-collector-contrib#48090
var useRequestTypeFeatureGate = featuregate.GlobalRegistry().MustRegister(
	"exporter.kafka.useRequestType",
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription(
		"When enabled, the Kafka exporter converts pdata into Kafka records "+
			"at request-creation time and uses a custom exporterhelper.Request "+
			"for queue/batch sizing. Improves MaxMessageBytes handling and "+
			"enables Kafka-native sizers. PoC scope: traces only.",
	),
	featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/48090"),
)
