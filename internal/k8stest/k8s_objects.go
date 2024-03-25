// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8stest // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8stest"

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/client-go/dynamic"
)

func CreateObject(client *K8sClient, manifest []byte) (*unstructured.Unstructured, error) {
	decoder := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	obj := &unstructured.Unstructured{}
	_, gvk, err := decoder.Decode(manifest, nil, obj)
	if err != nil {
		return nil, err
	}
	gvr, err := client.Mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, err
	}
	var resource dynamic.ResourceInterface
	if gvr.Scope.Name() == meta.RESTScopeNameNamespace {
		resource = client.DynamicClient.Resource(gvr.Resource).Namespace(obj.GetNamespace())
	} else {
		// cluster-scoped resources
		resource = client.DynamicClient.Resource(gvr.Resource)
	}

	return resource.Create(context.Background(), obj, metav1.CreateOptions{})
}

func DeleteObject(client *K8sClient, obj *unstructured.Unstructured) error {
	gvk := obj.GroupVersionKind()
	gvr, err := client.Mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return err
	}
	var resource dynamic.ResourceInterface
	if gvr.Scope.Name() == meta.RESTScopeNameNamespace {
		resource = client.DynamicClient.Resource(gvr.Resource).Namespace(obj.GetNamespace())
	} else {
		// cluster-scoped resources
		resource = client.DynamicClient.Resource(gvr.Resource)
	}

	gracePeriod := int64(0)
	deletePolicy := metav1.DeletePropagationForeground
	options := metav1.DeleteOptions{
		GracePeriodSeconds: &gracePeriod,
		PropagationPolicy:  &deletePolicy,
	}

	return resource.Delete(context.Background(), obj.GetName(), options)
}

func GetObject(client *K8sClient, gvk schema.GroupVersionKind, namespace string, name string) (*unstructured.Unstructured, error) {
	gvr := schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: strings.ToLower(gvk.Kind + "s"),
	}

	if client.DynamicClient == nil {
		return nil, fmt.Errorf("no dynamic client, can't get object")
	}

	return client.DynamicClient.Resource(gvr).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
}
