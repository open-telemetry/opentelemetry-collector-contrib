// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import "go.opentelemetry.io/collector/featuregate"

// azeventhubs is the name of the feature gate for franz-go consumer
const azEventHubFeatureGateName = "receiver.azureeventhubreceiver.UseAzeventhubs"

// azEventHubFeatureGateName is a feature gate that controls whether the azureeventhub receiver
// uses the new azeventhub library or the deprecated azure-event-hubs library
// for consuming messages. When enabled, the azureeventhub receiver will use the new azeventhub library.
var azEventHubFeatureGate = featuregate.GlobalRegistry().MustRegister(
	azEventHubFeatureGateName, featuregate.StageAlpha,
	featuregate.WithRegisterDescription("When enabled, the Azure Event Hubs receiver will use the azeventhub library."),
	featuregate.WithRegisterFromVersion("v0.129.0"),
)
