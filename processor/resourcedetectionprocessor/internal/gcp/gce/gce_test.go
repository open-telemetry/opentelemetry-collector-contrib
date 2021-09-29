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

package gce

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/gcp"
)

func TestNewDetector(t *testing.T) {
	d, err := NewDetector(componenttest.NewNopProcessorCreateSettings(), nil)
	assert.NotNil(t, d)
	assert.NoError(t, err)
}

func TestDetectTrue(t *testing.T) {
	md := &gcp.MockMetadata{}
	md.On("OnGCE").Return(true)
	md.On("ProjectID").Return("1", nil)
	md.On("Zone").Return("zone", nil)
	md.On("Hostname").Return("hostname", nil)
	md.On("InstanceID").Return("2", nil)
	md.On("InstanceName").Return("name", nil)
	md.On("Get", "instance/machine-type").Return("machine-type", nil)

	detector := &Detector{metadata: md}
	res, schemaURL, err := detector.Detect(context.Background())

	require.NoError(t, err)
	assert.Equal(t, conventions.SchemaURL, schemaURL)

	expected := internal.NewResource(map[string]interface{}{
		conventions.AttributeCloudProvider:         conventions.AttributeCloudProviderGCP,
		conventions.AttributeCloudPlatform:         conventions.AttributeCloudPlatformGCPComputeEngine,
		conventions.AttributeCloudAccountID:        "1",
		conventions.AttributeCloudAvailabilityZone: "zone",

		conventions.AttributeHostID:   "2",
		conventions.AttributeHostName: "hostname",
		conventions.AttributeHostType: "machine-type",
	})

	res.Attributes().Sort()
	expected.Attributes().Sort()
	assert.Equal(t, expected, res)
}

func TestDetectFalse(t *testing.T) {
	md := &gcp.MockMetadata{}
	md.On("OnGCE").Return(false)

	detector := &Detector{metadata: md}
	res, _, err := detector.Detect(context.Background())

	require.NoError(t, err)
	assert.True(t, internal.IsEmptyResource(res))
}

func TestDetectError(t *testing.T) {
	md := &gcp.MockMetadata{}
	md.On("OnGCE").Return(true)
	md.On("ProjectID").Return("", errors.New("err1"))
	md.On("Zone").Return("", errors.New("err2"))
	md.On("Hostname").Return("", errors.New("err3"))
	md.On("InstanceID").Return("", errors.New("err4"))
	md.On("InstanceName").Return("", errors.New("err5"))
	md.On("Get", "instance/machine-type").Return("", errors.New("err6"))

	detector := &Detector{metadata: md}
	res, _, err := detector.Detect(context.Background())

	assert.EqualError(t, err, "err1; err2; err3; err4; err6")

	expected := internal.NewResource(map[string]interface{}{
		conventions.AttributeCloudProvider: conventions.AttributeCloudProviderGCP,
		conventions.AttributeCloudPlatform: conventions.AttributeCloudPlatformGCPComputeEngine,
	})

	res.Attributes().Sort()
	expected.Attributes().Sort()
	assert.Equal(t, expected, res)
}
