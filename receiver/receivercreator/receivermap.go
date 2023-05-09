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

package receivercreator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receivercreator"

import (
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

// receiverMap is a multimap for mapping one id to many receivers. It does
// not deduplicate the same value being associated with the same key.
type receiverMap map[observer.EndpointID][]component.Component

// Put rcvr into key id. If rcvr is a duplicate it will still be added.
func (rm receiverMap) Put(id observer.EndpointID, rcvr component.Component) {
	rm[id] = append(rm[id], rcvr)
}

// Get receivers by id.
func (rm receiverMap) Get(id observer.EndpointID) []component.Component {
	return rm[id]
}

// Remove all receivers by id.
func (rm receiverMap) RemoveAll(id observer.EndpointID) {
	delete(rm, id)
}

// Get all receivers in the map.
func (rm receiverMap) Values() (out []component.Component) {
	for _, m := range rm {
		out = append(out, m...)
	}
	return
}

// Size is the number of total receivers in the map.
func (rm receiverMap) Size() (out int) {
	for _, m := range rm {
		out += len(m)
	}
	return
}
