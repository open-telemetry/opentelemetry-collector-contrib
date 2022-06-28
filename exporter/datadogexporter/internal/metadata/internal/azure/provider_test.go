// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package azure

import (
	"context"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/model/source"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/azure"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	src, err := provider.Source(context.Background())
	require.NoError(t, err)
	assert.Equal(t, source.HostnameKind, src.Kind)
	assert.Equal(t, "vmID", src.Identifier)

	clusterName, err := provider.ClusterName(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "aks-kenafeh-eu", clusterName)
}
