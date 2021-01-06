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

package system

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/translator/conventions"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

type mockMetadata struct {
	mock.Mock
}

func (m *mockMetadata) FQDN() (string, error) {
	args := m.MethodCalled("FQDN")
	return args.String(0), args.Error(1)
}

func (m *mockMetadata) OSType() (string, error) {
	args := m.MethodCalled("OSType")
	return args.String(0), args.Error(1)
}

func TestNewDetector(t *testing.T) {
	d, err := NewDetector(component.ProcessorCreateParams{Logger: zap.NewNop()}, nil)
	require.NoError(t, err)
	assert.NotNil(t, d)
}

func TestDetectFQDNAvailable(t *testing.T) {
	md := &mockMetadata{}
	md.On("FQDN").Return("fqdn", nil)
	md.On("OSType").Return("DARWIN", nil)

	detector := &Detector{provider: md}
	res, err := detector.Detect(context.Background())
	require.NoError(t, err)
	md.AssertExpectations(t)
	res.Attributes().Sort()

	expected := internal.NewResource(map[string]interface{}{
		conventions.AttributeHostName: "fqdn",
		conventions.AttributeOSType:   "DARWIN",
	})
	expected.Attributes().Sort()

	assert.Equal(t, expected, res)

}

func TestDetectError(t *testing.T) {
	// FQDN fails
	mdFQDN := &mockMetadata{}
	mdFQDN.On("OSType").Return("WINDOWS", nil)
	mdFQDN.On("FQDN").Return("", errors.New("err"))

	detector := &Detector{provider: mdFQDN}
	res, err := detector.Detect(context.Background())
	assert.Error(t, err)
	assert.True(t, internal.IsEmptyResource(res))

	// Hostname fails
	mdHostname := &mockMetadata{}
	mdHostname.On("FQDN").Return("fqdn", nil)
	mdHostname.On("OSType").Return("", errors.New("err"))

	detector = &Detector{provider: mdHostname}
	res, err = detector.Detect(context.Background())
	assert.Error(t, err)
	assert.True(t, internal.IsEmptyResource(res))
}
