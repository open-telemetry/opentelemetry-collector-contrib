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

package gcp

import "cloud.google.com/go/compute/metadata"

type Metadata interface {
	OnGCE() bool
	ProjectID() (string, error)
	Zone() (string, error)
	Hostname() (string, error)
	InstanceAttributeValue(attr string) (string, error)
	InstanceID() (string, error)
	InstanceName() (string, error)
	Get(suffix string) (string, error)
}

type MetadataImpl struct{}

var _ Metadata = (*MetadataImpl)(nil)

func (m *MetadataImpl) OnGCE() bool {
	return metadata.OnGCE()
}

func (m *MetadataImpl) ProjectID() (string, error) {
	return metadata.ProjectID()
}

func (m *MetadataImpl) Zone() (string, error) {
	return metadata.Zone()
}

func (m *MetadataImpl) Hostname() (string, error) {
	return metadata.Hostname()
}

func (m *MetadataImpl) InstanceAttributeValue(attr string) (string, error) {
	return metadata.InstanceAttributeValue(attr)
}

func (m *MetadataImpl) InstanceID() (string, error) {
	return metadata.InstanceID()
}

func (m *MetadataImpl) InstanceName() (string, error) {
	return metadata.InstanceName()
}

func (m *MetadataImpl) Get(suffix string) (string, error) {
	return metadata.Get(suffix)
}
