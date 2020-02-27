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
	"go.uber.org/zap"
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
	objectOwnersCache  *gocache.Cache
	namespaces         *gocache.Cache
	clientset          *kubernetes.Clientset
	logger             *zap.Logger
	cacheWarmupEnabled bool
}

// OwnerProvider allows to dynamically assign constructor
type OwnerProvider func(
	logger *zap.Logger,
	client *kubernetes.Clientset,
	cacheWarmupEnabled bool,
) (OwnerAPI, error)

func newOwnerProvider(
	logger *zap.Logger,
	clientset *kubernetes.Clientset,
	cacheWarmupEnabled bool) (OwnerAPI, error) {
	ownerCache := OwnerCache{}
	ownerCache.objectOwnersCache = gocache.New(15*time.Minute, 30*time.Minute)
	ownerCache.namespaces = gocache.New(15*time.Minute, 30*time.Minute)
	ownerCache.clientset = clientset
	ownerCache.logger = logger
	ownerCache.cacheWarmupEnabled = cacheWarmupEnabled
	return &ownerCache, nil
}

// GetNamespace retrieves relevant metadata from API or from cache
func (op *OwnerCache) GetNamespace(namespace string) *ObjectOwner {
	ns, found := op.namespaces.Get(string(namespace))
	if !found {
		startTime := time.Now()
		nn, _ := op.clientset.CoreV1().Namespaces().Get(namespace, meta_v1.GetOptions{})
		observability.RecordAPICallMadeAndLatency(&startTime)

		if nn != nil {
			oo := ObjectOwner{
				UID:       nn.UID,
				namespace: nn.Namespace,
				ownerUIDs: []types.UID{},
				kind:      "namespace",
				name:      nn.Name,
			}

			op.namespaces.Add(namespace, &oo, gocache.DefaultExpiration)

			// This is a good opportunity to do cache warmup!
			if op.cacheWarmupEnabled {
				op.warmupCache(namespace)
			}

			return &oo
		}
	}

	return ns.(*ObjectOwner)
}

func (op *OwnerCache) cacheMetadataObject(kind string, obj *meta_v1.ObjectMeta) {
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
}

type objectOrList struct {
	obj  *meta_v1.ObjectMeta
	list []*meta_v1.ObjectMeta
}

// objectApiCall queries for ObjectMeta; if name is nil, then all objects of given type from
// namespace are retrieved; otherwise only the one with specified name. Errors are quietly ignored
func (op *OwnerCache) objectApiCall(namespace string, kind string, name *string) objectOrList {
	result := objectOrList{}
	result.list = make([]*meta_v1.ObjectMeta, 0)

	startTime := time.Now()
	callNotMade := false

	// This *terribly* misses that: https://blog.golang.org/why-generics
	switch kind {
	case "DaemonSet":
		daemonSetAPI := op.clientset.ExtensionsV1beta1().DaemonSets(namespace)

		if name != nil {
			ds, _ := daemonSetAPI.Get(*name, meta_v1.GetOptions{})
			if ds != nil {
				result.obj = &ds.ObjectMeta
			}
		} else {
			dss, _ := daemonSetAPI.List(meta_v1.ListOptions{})
			if dss != nil {
				result.list = make([]*meta_v1.ObjectMeta, len(dss.Items))
				for i, it := range dss.Items {
					result.list[i] = &it.ObjectMeta
				}
			}
		}
	case "Deployment":
		deploymentsAPI := op.clientset.ExtensionsV1beta1().Deployments(namespace)

		if name != nil {
			ds, _ := deploymentsAPI.Get(*name, meta_v1.GetOptions{})
			if ds != nil {
				result.obj = &ds.ObjectMeta
			}
		} else {
			dss, _ := deploymentsAPI.List(meta_v1.ListOptions{})
			if dss != nil {
				result.list = make([]*meta_v1.ObjectMeta, len(dss.Items))
				for i, it := range dss.Items {
					result.list[i] = &it.ObjectMeta
				}
			}
		}
	case "Pod":
		podsAPI := op.clientset.CoreV1().Pods(namespace)

		if name != nil {
			ds, _ := podsAPI.Get(*name, meta_v1.GetOptions{})
			if ds != nil {
				result.obj = &ds.ObjectMeta
			}
		} else {
			dss, _ := podsAPI.List(meta_v1.ListOptions{})
			if dss != nil {
				result.list = make([]*meta_v1.ObjectMeta, len(dss.Items))
				for i, it := range dss.Items {
					result.list[i] = &it.ObjectMeta
				}
			}
		}
	case "ReplicaSet":
		replicaSetsAPI := op.clientset.ExtensionsV1beta1().ReplicaSets(namespace)

		if name != nil {
			ds, _ := replicaSetsAPI.Get(*name, meta_v1.GetOptions{})
			if ds != nil {
				result.obj = &ds.ObjectMeta
			}
		} else {
			dss, _ := replicaSetsAPI.List(meta_v1.ListOptions{})
			if dss != nil {
				result.list = make([]*meta_v1.ObjectMeta, len(dss.Items))
				for i, it := range dss.Items {
					result.list[i] = &it.ObjectMeta
				}
			}
		}
	case "Service":
		servicesAPI := op.clientset.CoreV1().Services(namespace)

		if name != nil {
			ds, _ := servicesAPI.Get(*name, meta_v1.GetOptions{})
			if ds != nil {
				result.obj = &ds.ObjectMeta
			}
		} else {
			dss, _ := servicesAPI.List(meta_v1.ListOptions{})
			if dss != nil {
				result.list = make([]*meta_v1.ObjectMeta, len(dss.Items))
				for i, it := range dss.Items {
					result.list[i] = &it.ObjectMeta
				}
			}
		}
	case "StatefulSet":
		statefulSetsAPI := op.clientset.AppsV1().StatefulSets(namespace)

		if name != nil {
			ds, _ := statefulSetsAPI.Get(*name, meta_v1.GetOptions{})
			if ds != nil {
				result.obj = &ds.ObjectMeta
			}
		} else {
			dss, _ := statefulSetsAPI.List(meta_v1.ListOptions{})
			if dss != nil {
				result.list = make([]*meta_v1.ObjectMeta, len(dss.Items))
				for i, it := range dss.Items {
					result.list[i] = &it.ObjectMeta
				}
			}
		}
	default:
		// Just mark we didn't call any API
		callNotMade = true
	}

	if !callNotMade {
		observability.RecordAPICallMadeAndLatency(&startTime)
	}

	return result
}

// warmupCache makes sure that enough objects are cached to not spend too much time on lazy
// requests via deepCacheObject
func (op *OwnerCache) warmupCache(namespace string) {
	startTime := time.Now()
	for _, kind := range []string{"DaemonSet", "Deployment", "ReplicaSet", "Service", "StatefulSet"} {
		res := op.objectApiCall(namespace, kind, nil)
		for _, it := range res.list {
			op.cacheMetadataObject("DaemonSet", it)
		}
	}
	op.logger.Info("Warming up cache",
		zap.String("namespace", namespace),
		zap.Int64("duration_ms", time.Since(startTime).Milliseconds()))
}

// deepCacheObject is a lazily executed function that makes a set of API calls to
// traverse the owners tree and cache it
func (op *OwnerCache) deepCacheObject(namespace string, kind string, name string) {
	res := op.objectApiCall(namespace, kind, &name)
	if res.obj != nil {
		// First set the cache
		op.cacheMetadataObject(kind, res.obj)

		// Then continue traverse
		for _, or := range res.obj.OwnerReferences {
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
