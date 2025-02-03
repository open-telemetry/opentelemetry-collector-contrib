// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xk8stest // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"

import (
	"context"
	"os"
	"path/filepath"

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
	deletePolicy := metav1.DeletePropagationForeground
	return resource.Delete(context.Background(), obj.GetName(), metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	})
}

func CreateObjects(client *K8sClient, dir string) ([]*unstructured.Unstructured, error) {
	var objs []*unstructured.Unstructured
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if file.IsDir() {
			continue // Skip directories
		}
		manifest, err := os.ReadFile(filepath.Join(dir, file.Name()))
		if err != nil {
			return nil, err
		}
		obj, err := CreateObject(client, manifest)
		if err != nil {
			return nil, err
		}
		objs = append(objs, obj)
	}
	return objs, nil
}

func DeleteObjects(client *K8sClient, objs []*unstructured.Unstructured) error {
	for _, obj := range objs {
		if err := DeleteObject(client, obj); err != nil {
			return err
		}
	}
	return nil
}
