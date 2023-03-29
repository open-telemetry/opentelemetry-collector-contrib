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

package telemetry // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray/telemetry"

import (
	awsxray "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray"
	"go.opentelemetry.io/collector/component"
)

func NewNopRegistry() Registry {
	return nopRegistryInstance
}

type nopRegistry struct {
	recorder Recorder
}

var _ Registry = (*nopRegistry)(nil)

var nopRegistryInstance = &nopRegistry{
	recorder: NewNopRecorder(),
}

func (n nopRegistry) Register(component.ID, Config, awsxray.XRayClient, ...RecorderOption) Recorder {
	return n.recorder
}

func (n nopRegistry) LoadOrStore(component.ID, Recorder) (Recorder, bool) {
	return n.recorder, false
}

func (n nopRegistry) Load(component.ID) Recorder {
	return n.recorder
}
