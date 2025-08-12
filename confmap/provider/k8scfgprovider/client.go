// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8scfgprovider

import (
	"context"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func getClientSet() (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("error getting k8s config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return clientset, nil
}

const namespaceFile = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

// get the current namespace
func getCurrentNamespace() (string, error) {
	data, err := os.ReadFile(namespaceFile)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func getConfigMap(ctx context.Context, namespace, name string) (*corev1.ConfigMap, error) {
	if namespace == "." {
		n, err := getCurrentNamespace()
		if err != nil {
			return nil, err
		}
		namespace = n
	}

	clientset, err := getClientSet()
	if err != nil {
		return nil, err
	}
	cm, err := clientset.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return cm, nil
}

func getSecret(ctx context.Context, namespace, name string) (*corev1.Secret, error) {
	if namespace == "." {
		n, err := getCurrentNamespace()
		if err != nil {
			return nil, err
		}
		namespace = n
	}

	clientset, err := getClientSet()
	if err != nil {
		return nil, err
	}
	s, err := clientset.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return s, nil
}
