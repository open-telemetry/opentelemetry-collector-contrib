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
func GetIDForCache(namespace string, resourceName string) string {
	return fmt.Sprintf("%s/%s", namespace, resourceName)
}

var re = regexp.MustCompile(`^[\w_-]+://`)

// StripContainerID returns a pure container id without the runtime scheme://.
func StripContainerID(id string) string {
	return re.ReplaceAllString(id, "")
}
