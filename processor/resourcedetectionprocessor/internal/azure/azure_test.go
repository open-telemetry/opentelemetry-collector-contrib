// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package azure

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/translator/conventions"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

type mockProvider struct {
	mock.Mock
}

func (m *mockProvider) metadata(_ context.Context) (*computeMetadata, error) {
	args := m.MethodCalled("metadata")
	return args.Get(0).(*computeMetadata), args.Error(1)
}

func TestNewDetector(t *testing.T) {
	d, err := NewDetector(component.ProcessorCreateParams{Logger: zap.NewNop()}, nil)
	require.NoError(t, err)
	assert.NotNil(t, d)
}

func TestDetectAzureAvailable(t *testing.T) {
	mp := &mockProvider{}
	mp.On("metadata").Return(&computeMetadata{
		Location:          "location",
		Name:              "name",
		VMID:              "vmID",
		VMSize:            "vmSize",
		SubscriptionID:    "subscriptionID",
		ResourceGroupName: "resourceGroup",
	}, nil)

	detector := &Detector{provider: mp}
	res, err := detector.Detect(context.Background())
	require.NoError(t, err)
	mp.AssertExpectations(t)
	res.Attributes().Sort()

	expected := internal.NewResource(map[string]interface{}{
		conventions.AttributeCloudProvider: conventions.AttributeCloudProviderAzure,
		conventions.AttributeHostName:      "name",
		conventions.AttributeCloudRegion:   "location",
		conventions.AttributeHostID:        "vmID",
		conventions.AttributeCloudAccount:  "subscriptionID",
		"azure.vm.size":                    "vmSize",
		"azure.resourcegroup.name":         "resourceGroup",
	})
	expected.Attributes().Sort()

	assert.Equal(t, expected, res)
}

func TestDetectError(t *testing.T) {
	mp := &mockProvider{}
	mp.On("metadata").Return(&computeMetadata{}, fmt.Errorf("mock error"))

	detector := &Detector{provider: mp}
	res, err := detector.Detect(context.Background())
	assert.Error(t, err)
	assert.True(t, internal.IsEmptyResource(res))
}
