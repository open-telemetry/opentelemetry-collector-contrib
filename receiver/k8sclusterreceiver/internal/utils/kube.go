// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package utils // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/utils"

import (
	"errors"
	"fmt"
	"regexp"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
)

// GetUIDForObject returns the UID for a Kubernetes object.
func GetUIDForObject(obj runtime.Object) (types.UID, error) {
	var key types.UID
	oma, ok := obj.(metav1.ObjectMetaAccessor)
	if !ok || oma.GetObjectMeta() == nil {
		return key, errors.New("kubernetes object is not of the expected form")
	}
	key = oma.GetObjectMeta().GetUID()
	return key, nil
}

// FindOwnerWithKind returns the OwnerReference of the matching kind from
// the provided list of owner references.
func FindOwnerWithKind(ors []metav1.OwnerReference, kind string) *metav1.OwnerReference {
	for _, or := range ors {
		if or.Kind == kind {
			return &or
		}
	}
	return nil
}

// GetIDForCache returns keys to lookup resources from the cache exposed
// by shared informers.
func GetIDForCache(namespace, resourceName string) string {
	return fmt.Sprintf("%s/%s", namespace, resourceName)
}

var re = regexp.MustCompile(`^[\w_-]+://`)

// StripContainerID returns a pure container id without the runtime scheme://.
func StripContainerID(id string) string {
	return re.ReplaceAllString(id, "")
}

// GetObjectFromStore retrieves the requested object from the given stores.
// first, the object is attempted to be retrieved from the store for all namespaces,
// and if it is not found there, the namespace-specific store is used
func GetObjectFromStore(namespace, objName string, stores map[string]cache.Store) (any, error) {
	for _, storeKey := range [2]string{metadata.ClusterWideInformerKey, namespace} {
		if store, ok := stores[storeKey]; ok {
			obj, exists, err := store.GetByKey(GetIDForCache(namespace, objName))
			if err != nil {
				return nil, err
			}
			if exists {
				return obj, nil
			}
		}
	}
	return nil, nil
}
