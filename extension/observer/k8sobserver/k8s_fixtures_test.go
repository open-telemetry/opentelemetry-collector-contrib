// Copyright 2020, OpenTelemetry Authors
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

package k8sobserver

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
)

// NewPod is a helper function for creating Pods for testing.
func NewPod(name, host string) *v1.Pod {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      name,
			UID:       types.UID(name + "-UID"),
			Labels: map[string]string{
				"env": "prod",
			},
		},
		Spec: v1.PodSpec{
			NodeName: host,
		},
		Status: v1.PodStatus{
			PodIP: "1.2.3.4",
			Conditions: []v1.PodCondition{
				{
					Type:   v1.PodReady,
					Status: v1.ConditionTrue,
				},
			},
		},
	}

	return pod
}

var pod1V1 = NewPod("pod1", "localhost")
var pod1V2 = func() *v1.Pod {
	pod := pod1V1.DeepCopy()
	pod.Labels["pod-version"] = "2"
	return pod
}()

var container1 = v1.Container{
	Name:  "container-1",
	Image: "container-image-1",
	Ports: []v1.ContainerPort{
		{Name: "http", HostPort: 0, ContainerPort: 80, Protocol: v1.ProtocolTCP, HostIP: ""},
	},
}

var container2 = v1.Container{
	Name:  "container-2",
	Image: "container-image-2",
	Ports: []v1.ContainerPort{
		{Name: "https", HostPort: 0, ContainerPort: 443, Protocol: v1.ProtocolTCP, HostIP: ""},
	},
}

var container1StatusWaiting = v1.ContainerStatus{
	Name: "container-1",
	State: v1.ContainerState{
		Waiting: &v1.ContainerStateWaiting{Reason: "waiting"},
	},
	Ready:        false,
	RestartCount: 1,
	Image:        "container-image-1",
	ImageID:      "12345",
	ContainerID:  "82389",
	Started:      nil,
}

var container2StatusRunning = v1.ContainerStatus{
	Name: "container-2",
	State: v1.ContainerState{
		Running: &v1.ContainerStateRunning{StartedAt: metav1.Now()},
	},
	Ready:   true,
	Image:   "container-image-1",
	Started: pointer.BoolPtr(true),
}

var podWithNamedPorts = func() *v1.Pod {
	pod := NewPod("pod-2", "localhost")
	pod.Labels = map[string]string{
		"env": "prod",
	}
	pod.Status.ContainerStatuses = []v1.ContainerStatus{
		container1StatusWaiting,
		container2StatusRunning,
	}
	pod.Spec.Containers = []v1.Container{
		container1,
		container2,
	}
	return pod
}()
