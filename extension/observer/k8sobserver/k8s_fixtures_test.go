// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobserver

import (
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// newPod is a helper function for creating Pods for testing.
func newPod(name, host string) *v1.Pod {
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
			Phase: v1.PodRunning,
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

var pod1V1 = newPod("pod1", "localhost")
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
	ContainerID:  "containerd://a808232bb4a57d421bb16f20dc9ab2a441343cb0aae8c369dc375838c7a49fd7",
	Started:      nil,
}

var container2StatusRunning = v1.ContainerStatus{
	Name: "container-2",
	State: v1.ContainerState{
		Running: &v1.ContainerStateRunning{StartedAt: metav1.Now()},
	},
	Ready:       true,
	Image:       "container-image-1",
	Started:     pointerBool(true),
	ContainerID: "containerd://a808232bb4a57d421bb16f20dc9ab2a441343cb0aae8c369dc375838c7a49fd7",
}

var podWithNamedPorts = func() *v1.Pod {
	pod := newPod("pod-2", "localhost")
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

func pointerBool(val bool) *bool {
	return &val
}

// newService is a helper function for creating Services for testing.
func newService(name string) *v1.Service {
	service := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      name,
			UID:       types.UID(name + "-UID"),
			Labels: map[string]string{
				"env": "prod",
			},
		},
		Spec: v1.ServiceSpec{
			Type:      v1.ServiceTypeClusterIP,
			ClusterIP: "1.2.3.4",
		},
	}

	return service
}

var serviceWithClusterIP = func() *v1.Service {
	return newService("service-1")
}()

var serviceWithClusterIPV2 = func() *v1.Service {
	service := serviceWithClusterIP.DeepCopy()
	service.Labels["service-version"] = "2"
	return service
}()

var ingress = &networkingv1.Ingress{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "default",
		Name:      "application-ingress",
		UID:       types.UID("ingress-1-UID"),
		Labels: map[string]string{
			"env": "prod",
		},
	},
	Spec: networkingv1.IngressSpec{
		Rules: []networkingv1.IngressRule{
			{
				Host: "host-1",
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{
							{
								Path: "/",
							},
						},
					},
				},
			},
		},
		TLS: []networkingv1.IngressTLS{
			{
				Hosts: []string{"host-1"},
			},
		},
	},
}

var ingressV2 = func() *networkingv1.Ingress {
	i2 := ingress.DeepCopy()
	i2.Labels["env"] = "hardening"
	return i2
}()

var ingressMultipleHost = &networkingv1.Ingress{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "default",
		Name:      "application-ingress",
		UID:       types.UID("ingress-1-UID"),
		Labels: map[string]string{
			"env": "prod",
		},
	},
	Spec: networkingv1.IngressSpec{
		Rules: []networkingv1.IngressRule{
			{
				Host: "host-invalid",
			},
			{
				Host: "host-1",
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{
							{
								Path: "/",
							},
						},
					},
				},
			},
			{
				Host: "host.2.host",
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{
							{
								Path: "/",
							},
						},
					},
				},
			},
			{
				Host: "host.3.host",
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{
							{
								Path: "/test",
							},
						},
					},
				},
			},
		},
		TLS: []networkingv1.IngressTLS{
			{
				Hosts: []string{"host-1", "*.2.host"},
			},
		},
	},
}

// newNode is a helper function for creating Nodes for testing.
func newNode(name, hostname string) *v1.Node {
	return &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "namespace",
			Name:      name,
			UID:       "uid",
			Labels: map[string]string{
				"label-key": "label-value",
			},
			Annotations: map[string]string{
				"annotation-key": "annotation-value",
			},
		},
		Spec: v1.NodeSpec{
			Taints: []v1.Taint{},
		},
		Status: v1.NodeStatus{
			Phase: v1.NodeRunning,
			Addresses: []v1.NodeAddress{
				{Type: v1.NodeHostName, Address: hostname},
				{Type: v1.NodeExternalDNS, Address: "externalDNS"},
				{Type: v1.NodeExternalIP, Address: "externalIP"},
				{Type: v1.NodeInternalDNS, Address: "internalDNS"},
				{Type: v1.NodeInternalIP, Address: "internalIP"},
			},
			DaemonEndpoints: v1.NodeDaemonEndpoints{KubeletEndpoint: v1.DaemonEndpoint{Port: 1234}},
			NodeInfo: v1.NodeSystemInfo{
				Architecture:            "architecture",
				BootID:                  "boot-id",
				ContainerRuntimeVersion: "runtime-version",
				KernelVersion:           "kernel-version",
				KubeProxyVersion:        "kube-proxy-version",
				KubeletVersion:          "kubelet-version",
				MachineID:               "machine-id",
				OperatingSystem:         "operating-system",
				OSImage:                 "os-image",
				SystemUUID:              "system-uuid",
			},
		},
	}
}

var node1V1 = newNode("node1", "localhost")
var node1V2 = func() *v1.Node {
	node := node1V1.DeepCopy()
	node.Labels["node-version"] = "2"
	return node
}()
