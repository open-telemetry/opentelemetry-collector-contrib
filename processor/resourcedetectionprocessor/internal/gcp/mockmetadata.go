// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gcp

import (
	"github.com/stretchr/testify/mock"
)

var _ Metadata = (*MockMetadata)(nil)

type MockMetadata struct {
	mock.Mock
}

func (m *MockMetadata) OnGCE() bool {
	return m.MethodCalled("OnGCE").Bool(0)
}

func (m *MockMetadata) ProjectID() (string, error) {
	args := m.MethodCalled("ProjectID")
	return args.String(0), args.Error(1)
}

func (m *MockMetadata) Zone() (string, error) {
	args := m.MethodCalled("Zone")
	return args.String(0), args.Error(1)
}

func (m *MockMetadata) Hostname() (string, error) {
	args := m.MethodCalled("Hostname")
	return args.String(0), args.Error(1)
}

func (m *MockMetadata) InstanceAttributeValue(attr string) (string, error) {
	args := m.MethodCalled("InstanceAttributeValue", attr)
	return args.String(0), args.Error(1)
}

func (m *MockMetadata) InstanceID() (string, error) {
	args := m.MethodCalled("InstanceID")
	return args.String(0), args.Error(1)
}

func (m *MockMetadata) InstanceName() (string, error) {
	args := m.MethodCalled("InstanceName")
	return args.String(0), args.Error(1)
}

func (m *MockMetadata) Get(suffix string) (string, error) {
	args := m.MethodCalled("Get", suffix)
	return args.String(0), args.Error(1)
}
