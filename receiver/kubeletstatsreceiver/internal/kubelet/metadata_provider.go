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

package kubelet // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/kubelet"

import (
	"encoding/json"

	v1 "k8s.io/api/core/v1"
)

// MetadataProvider wraps a RestClient, returning an unmarshaled metadata.
type MetadataProvider struct {
	rc RestClient
}

func NewMetadataProvider(rc RestClient) *MetadataProvider {
	return &MetadataProvider{rc: rc}
}

// Pods calls the /pods endpoint and unmarshals the
// results into a v1.PodList struct.
func (p *MetadataProvider) Pods() (*v1.PodList, error) {
	pods, err := p.rc.Pods()
	if err != nil {
		return nil, err
	}
	var out v1.PodList
	err = json.Unmarshal(pods, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}
