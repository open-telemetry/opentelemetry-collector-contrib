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
	"time"

	gocache "github.com/patrickmn/go-cache"
	api_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sprocessor/observability"
)

// ObjectOwner keeps single entry
type ObjectOwner struct {
	UID       types.UID
	ownerUIDs []types.UID
	namespace string
	kind      string
	name      string
}

// OwnerAPI describes functions that could allow retrieving owner info
type OwnerAPI interface {
	GetNamespace(namespace string) *ObjectOwner
	GetOwners(pod *api_v1.Pod) []*ObjectOwner
}

// OwnerCache is a simple structure which aids querying for owners
type OwnerCache struct {
	objectOwnersCache *gocache.Cache
	namespaces        *gocache.Cache
	clientset         *kubernetes.Clientset
}

// OwnerProvider allows to dynamically assign constructor
type OwnerProvider func(
	client *kubernetes.Clientset,
) OwnerAPI

func newOwnerProvider(
	clientset *kubernetes.Clientset) OwnerAPI {
	ownerCache := OwnerCache{}
	ownerCache.objectOwnersCache = gocache.New(15*time.Minute, 30*time.Minute)
	ownerCache.namespaces = gocache.New(15*time.Minute, 30*time.Minute)
	ownerCache.clientset = clientset
	return &ownerCache
}

// GetNamespace retrieves relevant metadata from API or from cache
func (op *OwnerCache) GetNamespace(namespace string) *ObjectOwner {
	ns, found := op.namespaces.Get(string(namespace))
	if !found {
		nn, _ := op.clientset.CoreV1().Namespaces().Get(namespace, meta_v1.GetOptions{})
		observability.RecordAPICallMade()
		if nn != nil {
			oo := ObjectOwner{
				UID:       nn.UID,
				namespace: nn.Namespace,
				ownerUIDs: []types.UID{},
				kind:      "namespace",
				name:      nn.Name,
			}

			op.namespaces.Add(namespace, &oo, gocache.DefaultExpiration)

			return &oo
		}
	}

	return ns.(*ObjectOwner)
}

func (op *OwnerCache) deepCacheObject(namespace string, kind string, name string) {
	var obj *meta_v1.ObjectMeta = nil
	switch kind {
	case "DaemonSet":
		ds, _ := op.clientset.ExtensionsV1beta1().DaemonSets(namespace).Get(name, meta_v1.GetOptions{})
		observability.RecordAPICallMade()
		if ds != nil {
			obj = &ds.ObjectMeta
		}
	case "Deployment":
		dp, _ := op.clientset.ExtensionsV1beta1().Deployments(namespace).Get(name, meta_v1.GetOptions{})
		observability.RecordAPICallMade()
		if dp != nil {
			obj = &dp.ObjectMeta
		}
	case "Pod":
		pp, _ := op.clientset.CoreV1().Pods(namespace).Get(name, meta_v1.GetOptions{})
		observability.RecordAPICallMade()
		if pp != nil {
			obj = &pp.ObjectMeta
		}
	case "ReplicaSet":
		rs, _ := op.clientset.ExtensionsV1beta1().ReplicaSets(namespace).Get(name, meta_v1.GetOptions{})
		observability.RecordAPICallMade()
		if rs != nil {
			obj = &rs.ObjectMeta
		}
	case "Service":
		srv, _ := op.clientset.CoreV1().Services(namespace).Get(name, meta_v1.GetOptions{})
		observability.RecordAPICallMade()
		if srv != nil {
			obj = &srv.ObjectMeta
		}
	case "StatefulSet":
		ss, _ := op.clientset.AppsV1().StatefulSets(namespace).Get(name, meta_v1.GetOptions{})
		observability.RecordAPICallMade()
		if ss != nil {
			obj = &ss.ObjectMeta
		}
	default:
		// Just do nothing here
	}

	if obj != nil {
		oo := ObjectOwner{
			UID:       obj.UID,
			namespace: obj.Namespace,
			ownerUIDs: []types.UID{},
			kind:      kind,
			name:      obj.Name,
		}
		for _, or := range obj.OwnerReferences {
			oo.ownerUIDs = append(oo.ownerUIDs, or.UID)
		}

		// First set the cache
		op.objectOwnersCache.Add(string(obj.UID), &oo, gocache.DefaultExpiration)

		// Then continue traverse
		for _, or := range obj.OwnerReferences {
			_, found := op.objectOwnersCache.Get(string(or.UID))
			if !found {
				op.deepCacheObject(namespace, or.Kind, or.Name)
			}
		}
	}
}

// GetOwners fetches deep tree of owners for a given pod
func (op *OwnerCache) GetOwners(pod *api_v1.Pod) []*ObjectOwner {
	objectOwners := []*ObjectOwner{}

	visited := map[types.UID]bool{}
	queue := []types.UID{}

	// Make sure the tree is cached/traversed first
	for _, or := range pod.OwnerReferences {
		_, found := op.objectOwnersCache.Get(string(or.UID))
		if !found {
			op.deepCacheObject(pod.Namespace, or.Kind, or.Name)
		}
	}

	// Now we can address this by UID, which should make things simpler, starting with pod owner references
	for _, or := range pod.OwnerReferences {
		if _, uidVisited := visited[or.UID]; !uidVisited {
			queue = append(queue, or.UID)
			visited[or.UID] = true
		}
	}

	for len(queue) > 0 {
		uid := queue[0]
		queue = queue[1:]
		if x, found := op.objectOwnersCache.Get(string(uid)); found {
			oo := x.(*ObjectOwner)
			objectOwners = append(objectOwners, oo)

			for _, ownerUID := range oo.ownerUIDs {
				if _, uidVisited := visited[ownerUID]; !uidVisited {
					queue = append(queue, ownerUID)
					visited[ownerUID] = true
				}
			}
		}
	}

	return objectOwners
}
