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

package gcp // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/gcp"

import "cloud.google.com/go/compute/metadata"

type Provider interface {
	OnGCE() bool
	ProjectID() (string, error)
	Zone() (string, error)
	Hostname() (string, error)
	InstanceAttributeValue(attr string) (string, error)
	InstanceID() (string, error)
	InstanceName() (string, error)
	Get(suffix string) (string, error)
}

type providerImpl struct{}

var _ Provider = (*providerImpl)(nil)

func NewProvider() Provider {
	return &providerImpl{}
}

func (m *providerImpl) OnGCE() bool {
	return metadata.OnGCE()
}

func (m *providerImpl) ProjectID() (string, error) {
	return metadata.ProjectID()
}

func (m *providerImpl) Zone() (string, error) {
	return metadata.Zone()
}

func (m *providerImpl) Hostname() (string, error) {
	return metadata.Hostname()
}

func (m *providerImpl) InstanceAttributeValue(attr string) (string, error) {
	return metadata.InstanceAttributeValue(attr)
}

func (m *providerImpl) InstanceID() (string, error) {
	return metadata.InstanceID()
}

func (m *providerImpl) InstanceName() (string, error) {
	return metadata.InstanceName()
}

func (m *providerImpl) Get(suffix string) (string, error) {
	return metadata.Get(suffix)
}
