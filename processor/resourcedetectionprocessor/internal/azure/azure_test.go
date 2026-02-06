// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azure

import (
	"errors"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/azure"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/azure/internal/metadata"
)

func TestNewDetector(t *testing.T) {
	dcfg := CreateDefaultConfig()
	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), dcfg)
	require.NoError(t, err)
	assert.NotNil(t, d)
}

func TestDetectAzureAvailable(t *testing.T) {
	mp := &azure.MockProvider{}
	mp.On("Metadata").Return(&azure.ComputeMetadata{
		Location:          "location",
		Name:              "name",
		VMID:              "vmID",
		VMSize:            "vmSize",
		SubscriptionID:    "subscriptionID",
		ResourceGroupName: "resourceGroup",
		VMScaleSetName:    "myScaleset",
		AvailabilityZone:  "availabilityZone",
		OSProfile: azure.OSProfile{
			ComputerName: "computerName",
		},
		TagsList: []azure.ComputeTagsListMetadata{
			{
				Name:  "tag1key",
				Value: "value1",
			},
			{
				Name:  "tag2key",
				Value: "value2",
			},
		},
	}, nil)

	detector := &Detector{
		provider: mp,
		tagKeyRegexes: []*regexp.Regexp{
			regexp.MustCompile("^tag1key$"),
		},
		rb: metadata.NewResourceBuilder(metadata.DefaultResourceAttributesConfig()),
	}
	res, schemaURL, err := detector.Detect(t.Context())
	require.NoError(t, err)
	require.Contains(t, schemaURL, "https://opentelemetry.io/schemas/")
	mp.AssertExpectations(t)

	expected := map[string]any{
		"cloud.provider":           "azure",
		"cloud.platform":           "azure_vm",
		"host.name":                "computerName",
		"cloud.region":             "location",
		"host.id":                  "vmID",
		"cloud.account.id":         "subscriptionID",
		"azure.vm.name":            "name",
		"azure.vm.size":            "vmSize",
		"azure.resourcegroup.name": "resourceGroup",
		"azure.vm.scaleset.name":   "myScaleset",
		"azure.tag.tag1key":        "value1",
	}

	notExpected := map[string]any{
		"azure.tag.tag2key": "value2",
	}

	assert.Equal(t, expected, res.Attributes().AsRaw())
	assert.NotEqual(t, notExpected, res.Attributes().AsRaw())
}

func TestDetectEmptyComputerNameFallsBackToVMName(t *testing.T) {
	mp := &azure.MockProvider{}
	mp.On("Metadata").Return(&azure.ComputeMetadata{
		Location:          "location",
		Name:              "vm-name",
		VMID:              "vmID",
		VMSize:            "vmSize",
		SubscriptionID:    "subscriptionID",
		ResourceGroupName: "resourceGroup",
		OSProfile: azure.OSProfile{
			ComputerName: "",
		},
	}, nil)

	detector := &Detector{
		provider: mp,
		rb:       metadata.NewResourceBuilder(metadata.DefaultResourceAttributesConfig()),
	}
	res, _, err := detector.Detect(t.Context())
	require.NoError(t, err)
	mp.AssertExpectations(t)

	// host.name should fall back to compute.Name when computerName is empty
	hostName, ok := res.Attributes().Get("host.name")
	require.True(t, ok)
	require.Equal(t, "vm-name", hostName.Str())
}

func TestDetectEmptyFieldsOmitted(t *testing.T) {
	mp := &azure.MockProvider{}
	mp.On("Metadata").Return(&azure.ComputeMetadata{
		Location:          "location",
		Name:              "vm-name",
		VMID:              "vmID",
		VMSize:            "vmSize",
		SubscriptionID:    "subscriptionID",
		ResourceGroupName: "resourceGroup",
		VMScaleSetName:    "",
		AvailabilityZone:  "",
		OSProfile: azure.OSProfile{
			ComputerName: "computerName",
		},
	}, nil)

	cfg := metadata.DefaultResourceAttributesConfig()
	cfg.CloudAvailabilityZone.Enabled = true

	detector := &Detector{
		provider: mp,
		rb:       metadata.NewResourceBuilder(cfg),
	}
	res, _, err := detector.Detect(t.Context())
	require.NoError(t, err)
	mp.AssertExpectations(t)

	// Empty VMScaleSetName and AvailabilityZone should not be set
	_, hasScaleSet := res.Attributes().Get("azure.vm.scaleset.name")
	require.False(t, hasScaleSet, "empty azure.vm.scaleset.name should not be set")

	_, hasZone := res.Attributes().Get("cloud.availability_zone")
	require.False(t, hasZone, "empty cloud.availability_zone should not be set")
}

func TestDetectError(t *testing.T) {
	mp := &azure.MockProvider{}
	mp.On("Metadata").Return(&azure.ComputeMetadata{}, errors.New("mock error"))
	detector := &Detector{
		provider: mp,
		logger:   zap.NewNop(),
		rb:       metadata.NewResourceBuilder(metadata.DefaultResourceAttributesConfig()),
	}
	res, _, err := detector.Detect(t.Context())
	assert.NoError(t, err)
	assert.True(t, internal.IsEmptyResource(res))
}
