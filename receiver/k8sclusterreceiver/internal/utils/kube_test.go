// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
