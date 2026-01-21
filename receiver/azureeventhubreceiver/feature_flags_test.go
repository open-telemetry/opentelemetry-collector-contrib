// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"testing"

	"go.opentelemetry.io/collector/featuregate"
)

func TestAzEventHubFeatureGateRegistration(t *testing.T) {
	if azEventHubFeatureGate == nil {
		t.Fatalf("expected feature gate %q to be registered, but it was not", azEventHubFeatureGateName)
	}
	if azEventHubFeatureGate.ID() != azEventHubFeatureGateName {
		t.Errorf("expected gate ID %q, got %q", azEventHubFeatureGateName, azEventHubFeatureGate.ID())
	}
	if azEventHubFeatureGate.Stage() != featuregate.StageStable {
		t.Errorf("expected stage %q, got %q", featuregate.StageBeta, azEventHubFeatureGate.Stage())
	}
	expectedDesc := "When enabled, the Azure Event Hubs receiver will use the azeventhub library."
	if azEventHubFeatureGate.Description() != expectedDesc {
		t.Errorf("expected description %q, got %q", expectedDesc, azEventHubFeatureGate.Description())
	}
	expectedVersion := "v0.129.0"
	if azEventHubFeatureGate.FromVersion() != expectedVersion {
		t.Errorf("expected FromVersion %q, got %q", expectedVersion, azEventHubFeatureGate.FromVersion())
	}
}
