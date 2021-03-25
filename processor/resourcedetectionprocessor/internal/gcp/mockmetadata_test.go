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
	"testing"
)

func TestMockGCEMetadata(t *testing.T) {
	metadata := &MockMetadata{}
	metadata.On("OnGCE").Return(false)
	metadata.On("ProjectID").Return("", nil)
	metadata.On("Zone").Return("", nil)
	metadata.On("Hostname").Return("", nil)
	metadata.On("InstanceAttributeValue", "").Return("", nil)
	metadata.On("InstanceID").Return("", nil)
	metadata.On("InstanceName").Return("", nil)
	metadata.On("Get", "").Return("", nil)

	metadata.OnGCE()
	metadata.ProjectID()
	metadata.Zone()
	metadata.Hostname()
	metadata.InstanceAttributeValue("")
	metadata.InstanceID()
	metadata.InstanceName()
	metadata.Get("")

	metadata.AssertExpectations(t)
}
