// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vpc

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor/processortest"

	vpcprovider "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/ibmcloud/vpc"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/ibmcloud/vpc/internal/metadata"
)

func TestNewDetector(t *testing.T) {
	cfg := CreateDefaultConfig()
	d, err := NewDetector(processortest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, err)
	require.NotNil(t, d)
}

func TestNewDetectorHTTPS(t *testing.T) {
	cfg := CreateDefaultConfig()
	cfg.Protocol = "https"
	d, err := NewDetector(processortest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, err)
	require.NotNil(t, d)
}

func TestNewDetectorInvalidProtocol(t *testing.T) {
	cfg := CreateDefaultConfig()
	cfg.Protocol = "ftp"
	d, err := NewDetector(processortest.NewNopSettings(metadata.Type), cfg)
	require.Error(t, err)
	require.Nil(t, d)
	require.Contains(t, err.Error(), `invalid protocol "ftp"`)
}

func TestDetect(t *testing.T) {
	mp := &vpcprovider.MockProvider{}
	mp.On("InstanceMetadata").Return(&vpcprovider.InstanceMetadata{
		ID:   "0717_1e09281b-f177-46fb-b1f1-bc152b2e391a",
		CRN:  "crn:v1:bluemix:public:is:us-south-1:a/123456789012::instance:0717_1e09281b-f177-46fb-b1f1-bc152b2e391a",
		Name: "my-instance",
		Profile: struct {
			Name string `json:"name"`
		}{Name: "bx2-2x8"},
		Zone: struct {
			Name string `json:"name"`
		}{Name: "us-south-1"},
		VPC: struct {
			ID   string `json:"id"`
			CRN  string `json:"crn"`
			Name string `json:"name"`
		}{
			ID:   "r006-4727d842-f94f-4a2d-824a-9bc9b02c523b",
			CRN:  "crn:v1:bluemix:public:is:us-south:a/123456789012::vpc:r006-4727d842-f94f-4a2d-824a-9bc9b02c523b",
			Name: "my-vpc",
		},
		ResourceGroup: struct {
			ID string `json:"id"`
		}{ID: "fee82deba12e4c0fb69c3b09d1f12345"},
		Image: struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}{
			ID:   "r006-ed3f775f-ad7e-4e37-ae62-7199b4988b00",
			Name: "ibm-ubuntu-22-04-4-minimal-amd64-3",
		},
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
	assert.Equal(t, 11, attrs.Len())

	assertAttribute(t, attrs, "cloud.provider", "ibm_cloud")
	assertAttribute(t, attrs, "cloud.platform", "ibm_cloud.vpc")
	assertAttribute(t, attrs, "cloud.account.id", "123456789012")
	assertAttribute(t, attrs, "cloud.region", "us-south")
	assertAttribute(t, attrs, "cloud.availability_zone", "us-south-1")
	assertAttribute(t, attrs, "cloud.resource_id", "crn:v1:bluemix:public:is:us-south-1:a/123456789012::instance:0717_1e09281b-f177-46fb-b1f1-bc152b2e391a")
	assertAttribute(t, attrs, "host.id", "0717_1e09281b-f177-46fb-b1f1-bc152b2e391a")
	assertAttribute(t, attrs, "host.image.id", "r006-ed3f775f-ad7e-4e37-ae62-7199b4988b00")
	assertAttribute(t, attrs, "host.image.name", "ibm-ubuntu-22-04-4-minimal-amd64-3")
	assertAttribute(t, attrs, "host.name", "my-instance")
	assertAttribute(t, attrs, "host.type", "bx2-2x8")

	mp.AssertExpectations(t)
}

func TestDetectError(t *testing.T) {
	mp := &vpcprovider.MockProvider{}
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

func TestRegionFromZone(t *testing.T) {
	tests := []struct {
		zone   string
		region string
	}{
		{"us-south-1", "us-south"},
		{"eu-de-2", "eu-de"},
		{"jp-tok-3", "jp-tok"},
		{"au-syd-1", "au-syd"},
		{"br-sao-1", "br-sao"},
		{"ca-tor-1", "ca-tor"},
		{"us-east-1", "us-east"},
		{"nodash", "nodash"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.zone, func(t *testing.T) {
			assert.Equal(t, tt.region, regionFromZone(tt.zone))
		})
	}
}

func TestAccountIDFromCRN(t *testing.T) {
	tests := []struct {
		name    string
		crn     string
		account string
	}{
		{
			name:    "standard CRN with a/ prefix",
			crn:     "crn:v1:bluemix:public:is:us-south-1:a/123456789012::instance:0717_xxx",
			account: "123456789012",
		},
		{
			name:    "CRN without a/ prefix",
			crn:     "crn:v1:bluemix:public:is:us-south-1:123456789012::instance:0717_xxx",
			account: "123456789012",
		},
		{
			name:    "short CRN",
			crn:     "crn:v1:bluemix",
			account: "",
		},
		{
			name:    "empty CRN",
			crn:     "",
			account: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.account, accountIDFromCRN(tt.crn))
		})
	}
}

func TestDetectWithDisabledAttributes(t *testing.T) {
	mp := &vpcprovider.MockProvider{}
	mp.On("InstanceMetadata").Return(&vpcprovider.InstanceMetadata{
		ID:   "test-id",
		CRN:  "crn:v1:bluemix:public:is:us-south-1:a/123456::instance:test-id",
		Name: "test-instance",
		Profile: struct {
			Name string `json:"name"`
		}{Name: "bx2-2x8"},
		Zone: struct {
			Name string `json:"name"`
		}{Name: "us-south-1"},
		VPC: struct {
			ID   string `json:"id"`
			CRN  string `json:"crn"`
			Name string `json:"name"`
		}{},
		ResourceGroup: struct {
			ID string `json:"id"`
		}{},
		Image: struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}{},
	}, nil)

	cfg := metadata.DefaultResourceAttributesConfig()
	cfg.CloudRegion.Enabled = false
	cfg.HostType.Enabled = false

	detector := &Detector{
		provider: mp,
		logger:   processortest.NewNopSettings(metadata.Type).Logger,
		rb:       metadata.NewResourceBuilder(cfg),
	}

	res, schemaURL, err := detector.Detect(t.Context())
	require.NoError(t, err)
	assert.NotEmpty(t, schemaURL)

	attrs := res.Attributes()
	// 11 total - 2 disabled = 9
	assert.Equal(t, 9, attrs.Len())

	// These should be present
	assertAttribute(t, attrs, "cloud.provider", "ibm_cloud")
	assertAttribute(t, attrs, "host.id", "test-id")

	// These should be absent
	_, ok := attrs.Get("cloud.region")
	assert.False(t, ok)
	_, ok = attrs.Get("host.type")
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
