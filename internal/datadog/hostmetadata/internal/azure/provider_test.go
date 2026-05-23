// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azure

import (
	"testing"

	"github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/attributes/source"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/azure"
)

func TestProvider(t *testing.T) {
	mp := &azure.MockProvider{}
	mp.On("Metadata").Return(&azure.ComputeMetadata{
		Location:          "location",
		Name:              "name",
		VMID:              "vmID",
		VMSize:            "vmSize",
		SubscriptionID:    "subscriptionID",
		ResourceGroupName: "MC_aks-kenafeh_aks-kenafeh-eu_westeurope",
		VMScaleSetName:    "myScaleset",
	}, nil)

	provider := &Provider{detector: mp}
	src, err := provider.Source(t.Context())
	require.NoError(t, err)
	assert.Equal(t, source.HostnameKind, src.Kind)
	assert.Equal(t, "vmID", src.Identifier)

	clusterName, err := provider.ClusterName(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "aks-kenafeh-eu", clusterName)
}
