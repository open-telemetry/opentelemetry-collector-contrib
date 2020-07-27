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

import "cloud.google.com/go/compute/metadata"

type gceMetadata interface {
	OnGCE() bool
	ProjectID() (string, error)
	Zone() (string, error)
	Hostname() (string, error)
	InstanceID() (string, error)
	InstanceName() (string, error)
	Get(suffix string) (string, error)
}

type gceMetadataImpl struct{}

func (m *gceMetadataImpl) OnGCE() bool {
	return metadata.OnGCE()
}

func (m *gceMetadataImpl) ProjectID() (string, error) {
	return metadata.ProjectID()
}

func (m *gceMetadataImpl) Zone() (string, error) {
	return metadata.Zone()
}

func (m *gceMetadataImpl) Hostname() (string, error) {
	return metadata.Hostname()
}

func (m *gceMetadataImpl) InstanceID() (string, error) {
	return metadata.InstanceID()
}

func (m *gceMetadataImpl) InstanceName() (string, error) {
	return metadata.InstanceName()
}

func (m *gceMetadataImpl) Get(suffix string) (string, error) {
	return metadata.Get(suffix)
}
