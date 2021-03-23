// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package k8sutil

import (
	"fmt"
)

func CreatePodKey(namespace, podName string) string {
	if namespace == "" || podName == "" {
		return ""
	}
	return fmt.Sprintf("namespace:%s,podName:%s", namespace, podName)
}

func CreateContainerKey(namespace, podName, containerName string) string {
	if namespace == "" || podName == "" || containerName == "" {
		return ""
	}
	return fmt.Sprintf("namespace:%s,podName:%s,containerName:%s", namespace, podName, containerName)
}
