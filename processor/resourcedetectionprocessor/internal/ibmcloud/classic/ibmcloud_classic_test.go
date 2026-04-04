// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package classic

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor/processortest"

	classicprovider "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/ibmcloud/classic"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/ibmcloud/classic/internal/metadata"
)

func TestNewDetector(t *testing.T) {
	cfg := CreateDefaultConfig()
	d, err := NewDetector(processortest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, err)
	require.NotNil(t, d)
}

func TestDetect(t *testing.T) {
	mp := &classicprovider.MockProvider{}
	mp.On("InstanceMetadata").Return(&classicprovider.InstanceMetadata{
		ID:               "156800198",
		Hostname:         "otel-collector",
		Datacenter:       "par01",
		AccountID:        "3186058",
		GlobalIdentifier: "06220b70-9072-4f83-ba16-d62f03106c1c",
	}, nil)

	detector := &Detector{
		provider: mp,
		logger:   processortest.NewNopSettings(metadata.Type).Logger,
		rb:       metadata.NewResourceBuilder(metadata.DefaultResourceAttributesConfig()),
	}

	res, schemaURL, err := detector.Detect(t.Context())
	require.NoError(t, err)
	assert.NotEmpty(t, schemaURL)

	attrs := res.Attributes()
	assert.Equal(t, 7, attrs.Len())

	assertAttribute(t, attrs, "cloud.provider", "ibm_cloud")
	assertAttribute(t, attrs, "cloud.platform", "ibm_cloud.classic")
	assertAttribute(t, attrs, "cloud.account.id", "3186058")
	assertAttribute(t, attrs, "cloud.availability_zone", "par01")
	assertAttribute(t, attrs, "cloud.resource_id", "06220b70-9072-4f83-ba16-d62f03106c1c")
	assertAttribute(t, attrs, "host.id", "156800198")
	assertAttribute(t, attrs, "host.name", "otel-collector")

	mp.AssertExpectations(t)
}

func TestDetectError(t *testing.T) {
	mp := &classicprovider.MockProvider{}
	mp.On("InstanceMetadata").Return(nil, errors.New("connection refused"))

	detector := &Detector{
		provider: mp,
		logger:   processortest.NewNopSettings(metadata.Type).Logger,
		rb:       metadata.NewResourceBuilder(metadata.DefaultResourceAttributesConfig()),
	}

	res, schemaURL, err := detector.Detect(t.Context())
	require.NoError(t, err) // errors are swallowed, not returned
	assert.Empty(t, schemaURL)
	assert.Equal(t, 0, res.Attributes().Len())

	mp.AssertExpectations(t)
}

func TestDetectWithDisabledAttributes(t *testing.T) {
	mp := &classicprovider.MockProvider{}
	mp.On("InstanceMetadata").Return(&classicprovider.InstanceMetadata{
		ID:               "12345",
		Hostname:         "test-host",
		Datacenter:       "dal13",
		AccountID:        "99999",
		GlobalIdentifier: "abcd-1234-efgh-5678",
	}, nil)

	cfg := metadata.DefaultResourceAttributesConfig()
	cfg.CloudAvailabilityZone.Enabled = false
	cfg.HostName.Enabled = false

	detector := &Detector{
		provider: mp,
		logger:   processortest.NewNopSettings(metadata.Type).Logger,
		rb:       metadata.NewResourceBuilder(cfg),
	}

	res, schemaURL, err := detector.Detect(t.Context())
	require.NoError(t, err)
	assert.NotEmpty(t, schemaURL)

	attrs := res.Attributes()
	// 7 total - 2 disabled = 5
	assert.Equal(t, 5, attrs.Len())

	// These should be present
	assertAttribute(t, attrs, "cloud.provider", "ibm_cloud")
	assertAttribute(t, attrs, "host.id", "12345")

	// These should be absent
	_, ok := attrs.Get("cloud.availability_zone")
	assert.False(t, ok)
	_, ok = attrs.Get("host.name")
	assert.False(t, ok)

	mp.AssertExpectations(t)
}

func assertAttribute(t *testing.T, attrs pcommon.Map, key, expected string) {
	t.Helper()
	val, ok := attrs.Get(key)
	assert.True(t, ok, "attribute %s not found", key)
	if ok {
		assert.Equal(t, expected, val.AsString())
	}
}
