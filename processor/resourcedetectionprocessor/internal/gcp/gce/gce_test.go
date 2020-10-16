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
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/translator/conventions"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

type mockMetadata struct {
	mock.Mock
}

func (m *mockMetadata) OnGCE() bool {
	return m.MethodCalled("OnGCE").Bool(0)
}

func (m *mockMetadata) ProjectID() (string, error) {
	args := m.MethodCalled("ProjectID")
	return args.String(0), args.Error(1)
}

func (m *mockMetadata) Zone() (string, error) {
	args := m.MethodCalled("Zone")
	return args.String(0), args.Error(1)
}

func (m *mockMetadata) Hostname() (string, error) {
	args := m.MethodCalled("Hostname")
	return args.String(0), args.Error(1)
}

func (m *mockMetadata) InstanceID() (string, error) {
	args := m.MethodCalled("InstanceID")
	return args.String(0), args.Error(1)
}

func (m *mockMetadata) InstanceName() (string, error) {
	args := m.MethodCalled("InstanceName")
	return args.String(0), args.Error(1)
}

func (m *mockMetadata) Get(suffix string) (string, error) {
	args := m.MethodCalled("Get")
	return args.String(0), args.Error(1)
}

func TestNewDetector(t *testing.T) {
	d, err := NewDetector()
	assert.NotNil(t, d)
	assert.NoError(t, err)
}

func TestDetectTrue(t *testing.T) {
	md := &mockMetadata{}
	md.On("OnGCE").Return(true)
	md.On("ProjectID").Return("1", nil)
	md.On("Zone").Return("zone", nil)
	md.On("Hostname").Return("hostname", nil)
	md.On("InstanceID").Return("2", nil)
	md.On("InstanceName").Return("name", nil)
	md.On("Get").Return("machine-type", nil)

	detector := &Detector{metadata: md}
	res, err := detector.Detect(context.Background())

	require.NoError(t, err)

	expected := internal.NewResource(map[string]interface{}{
		conventions.AttributeCloudProvider: conventions.AttributeCloudProviderGCP,
		conventions.AttributeCloudAccount:  "1",
		conventions.AttributeCloudZone:     "zone",

		conventions.AttributeHostHostname: "hostname",
		conventions.AttributeHostID:       "2",
		conventions.AttributeHostName:     "name",
		conventions.AttributeHostType:     "machine-type",
	})

	res.Attributes().Sort()
	expected.Attributes().Sort()
	assert.Equal(t, expected, res)
}

func TestDetectFalse(t *testing.T) {
	md := &mockMetadata{}
	md.On("OnGCE").Return(false)

	detector := &Detector{metadata: md}
	res, err := detector.Detect(context.Background())

	require.NoError(t, err)
	assert.True(t, internal.IsEmptyResource(res))
}

func TestDetectError(t *testing.T) {
	md := &mockMetadata{}
	md.On("OnGCE").Return(true)
	md.On("ProjectID").Return("", errors.New("err1"))
	md.On("Zone").Return("", errors.New("err2"))
	md.On("Hostname").Return("", errors.New("err3"))
	md.On("InstanceID").Return("", errors.New("err4"))
	md.On("InstanceName").Return("", errors.New("err5"))
	md.On("Get").Return("", errors.New("err6"))

	detector := &Detector{metadata: md}
	res, err := detector.Detect(context.Background())

	assert.EqualError(t, err, "[err1; err2; err3; err4; err5; err6]")

	expected := internal.NewResource(map[string]interface{}{conventions.AttributeCloudProvider: conventions.AttributeCloudProviderGCP})

	res.Attributes().Sort()
	expected.Attributes().Sort()
	assert.Equal(t, expected, res)
}
