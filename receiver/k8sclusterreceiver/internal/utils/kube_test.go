// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestGetUIDForObject(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			UID: "test-pod-uid",
		},
	}
	actual, _ := GetUIDForObject(pod)
	require.Equal(t, types.UID("test-pod-uid"), actual)

	node := &corev1.Node{
		ObjectMeta: v1.ObjectMeta{
			UID: "test-node-uid",
		},
	}
	actual, _ = GetUIDForObject(node)
	require.Equal(t, types.UID("test-node-uid"), actual)
}

func TestStripContainerID(t *testing.T) {
	id1 := "docker://some-id"
	id2 := "crio://some-id"

	require.Equal(t, "some-id", StripContainerID(id1))
	require.Equal(t, "some-id", StripContainerID(id2))
}

func TestGetIDForCache(t *testing.T) {
	ns := "namespace"
	resName := "resName"

	actual := GetIDForCache(ns, resName)

	require.Equal(t, ns+"/"+resName, actual)
}
