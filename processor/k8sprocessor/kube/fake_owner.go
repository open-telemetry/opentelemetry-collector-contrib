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
	"go.uber.org/zap"
	api_v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

// fakeOwnerCache is a simple structure which aids querying for owners
type fakeOwnerCache struct {
	logger       *zap.Logger
	objectOwners map[string]*ObjectOwner
}

// NewOwnerProvider creates new instance of the owners api
func newFakeOwnerProvider(logger *zap.Logger,
	client kubernetes.Interface,
	labelSelector labels.Selector,
	fieldSelector fields.Selector,
	namespace string) (OwnerAPI, error) {
	ownerCache := fakeOwnerCache{}
	ownerCache.objectOwners = map[string]*ObjectOwner{}
	ownerCache.logger = logger

	oo := ObjectOwner{
		UID:       "1a1658f9-7818-11e9-90f1-02324f7e0d1e",
		namespace: "kube-system",
		ownerUIDs: []types.UID{},
		kind:      "ReplicaSet",
		name:      "SomeReplicaSet",
	}
	ownerCache.objectOwners[string(oo.UID)] = &oo

	return &ownerCache, nil
}

// Start
func (op *fakeOwnerCache) Start() {}

// Stop
func (op *fakeOwnerCache) Stop() {}

// GetServices fetches list of services for a given pod
func (op *fakeOwnerCache) GetServices(pod *api_v1.Pod) []string {
	return []string{"foo", "bar"}
}

// GetNamespace returns a namespace
func (op *fakeOwnerCache) GetNamespace(pod *api_v1.Pod) *api_v1.Namespace {
	namespace := api_v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   pod.Namespace,
			Labels: map[string]string{"label": "namespace_label_value"},
		},
	}
	return &namespace
}

// GetOwners fetches deep tree of owners for a given pod
func (op *fakeOwnerCache) GetOwners(pod *api_v1.Pod) []*ObjectOwner {
	objectOwners := []*ObjectOwner{}

	// Make sure the tree is cached/traversed first
	for _, or := range pod.OwnerReferences {
		oo, found := op.objectOwners[string(or.UID)]
		if found {
			objectOwners = append(objectOwners, oo)
		}
	}

	return objectOwners
}
