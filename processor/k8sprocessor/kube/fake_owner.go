// Copyright 2019 Omnition Authors
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

package kube

import (
	api_v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

// fakeOwnerCache is a simple structure which aids querying for owners
type fakeOwnerCache struct{}

// NewOwnerProvider creates new instance of the owners api
func newFakeOwnerProvider(clientset *kubernetes.Clientset) OwnerAPI {
	ownerCache := fakeOwnerCache{}
	return &ownerCache
}

// GetNamespace retrieves relevant metadata from API or from cache
func (op *fakeOwnerCache) GetNamespace(namespace string) *ObjectOwner {
	oo := ObjectOwner{
		UID:       "33333-66666",
		namespace: namespace,
		ownerUIDs: []types.UID{},
		kind:      "namespace",
		name:      namespace,
	}

	return &oo
}

// GetOwners fetches deep tree of owners for a given pod
func (op *fakeOwnerCache) GetOwners(pod *api_v1.Pod) []*ObjectOwner {
	objectOwners := []*ObjectOwner{}

	oo := ObjectOwner{
		UID:       "12345",
		namespace: pod.Namespace,
		ownerUIDs: []types.UID{},
		kind:      "ReplicaSet",
		name:      "SomeReplicaSet",
	}

	objectOwners = append(objectOwners, &oo)
	return objectOwners
}
