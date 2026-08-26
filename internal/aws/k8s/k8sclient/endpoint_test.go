// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sclient

import (
	"log"
	goruntime "runtime"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

var endpointsArray = []runtime.Object{
	&discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "guestbook-abc123",
			GenerateName:    "",
			Namespace:       "default",
			UID:             "a885b78c-5573-11e9-b47e-066a7a20bac8",
			ResourceVersion: "1550348",
			Generation:      0,
			CreationTimestamp: metav1.Time{
				Time: time.Now(),
			},
			Labels: map[string]string{
				"app":                        "guestbook",
				discoveryv1.LabelServiceName: "guestbook",
				discoveryv1.LabelManagedBy:   "endpointslice-controller.k8s.io",
			},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses: []string{"192.168.122.125"},
				Conditions: discoveryv1.EndpointConditions{
					Ready: aws.Bool(true),
				},
				NodeName: aws.String("ip-192-168-76-61.eu-west-1.compute.internal"),
				TargetRef: &v1.ObjectReference{
					Kind:            "Pod",
					Namespace:       "default",
					Name:            "guestbook-qjqnz",
					UID:             "9ca74e86-5573-11e9-b47e-066a7a20bac8",
					APIVersion:      "v1",
					ResourceVersion: "1550311",
				},
			},
			{
				Addresses: []string{"192.168.176.235"},
				Conditions: discoveryv1.EndpointConditions{
					Ready: aws.Bool(true),
				},
				NodeName: aws.String("ip-192-168-153-1.eu-west-1.compute.internal"),
				TargetRef: &v1.ObjectReference{
					Kind:            "Pod",
					Namespace:       "default",
					Name:            "guestbook-92wmq",
					UID:             "9ca662bb-5573-11e9-b47e-066a7a20bac8",
					APIVersion:      "v1",
					ResourceVersion: "1550313",
				},
			},
			{
				Addresses: []string{"192.168.251.65"},
				Conditions: discoveryv1.EndpointConditions{
					Ready: aws.Bool(true),
				},
				NodeName: aws.String("ip-192-168-200-63.eu-west-1.compute.internal"),
				TargetRef: &v1.ObjectReference{
					Kind:            "Pod",
					Namespace:       "default",
					Name:            "guestbook-qbdv8",
					UID:             "9ca76fd6-5573-11e9-b47e-066a7a20bac8",
					APIVersion:      "v1",
					ResourceVersion: "1550319",
				},
			},
		},
		Ports: []discoveryv1.EndpointPort{
			{
				Name:     aws.String(""),
				Port:     aws.Int32(3000),
				Protocol: func() *v1.Protocol { p := v1.ProtocolTCP; return &p }(),
			},
		},
	},
	&discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "kubernetes-xyz789",
			GenerateName:    "",
			Namespace:       "default",
			UID:             "4daf1688-4c0a-11e9-b47e-066a7a20bac8",
			ResourceVersion: "5807557",
			Generation:      0,
			CreationTimestamp: metav1.Time{
				Time: time.Now(),
			},
			Labels: map[string]string{
				discoveryv1.LabelServiceName: "kubernetes",
				discoveryv1.LabelManagedBy:   "endpointslice-controller.k8s.io",
			},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses: []string{"192.168.174.242"},
				Conditions: discoveryv1.EndpointConditions{
					Ready: aws.Bool(true),
				},
			},
			{
				Addresses: []string{"192.168.82.3"},
				Conditions: discoveryv1.EndpointConditions{
					Ready: aws.Bool(true),
				},
			},
		},
		Ports: []discoveryv1.EndpointPort{
			{
				Name:     aws.String("https"),
				Port:     aws.Int32(443),
				Protocol: func() *v1.Protocol { p := v1.ProtocolTCP; return &p }(),
			},
		},
	},
	&discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "redis-master-def456",
			GenerateName:    "",
			Namespace:       "default",
			UID:             "74ac431b-5573-11e9-b47e-066a7a20bac8",
			ResourceVersion: "1550146",
			Generation:      0,
			CreationTimestamp: metav1.Time{
				Time: time.Now(),
			},
			Labels: map[string]string{
				"app":                        "redis",
				"role":                       "master",
				discoveryv1.LabelServiceName: "redis-master",
				discoveryv1.LabelManagedBy:   "endpointslice-controller.k8s.io",
			},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses: []string{"192.168.108.68"},
				Conditions: discoveryv1.EndpointConditions{
					Ready: aws.Bool(true),
				},
				NodeName: aws.String("ip-192-168-76-61.eu-west-1.compute.internal"),
				TargetRef: &v1.ObjectReference{
					Kind:            "Pod",
					Namespace:       "default",
					Name:            "redis-master-rh2bd",
					UID:             "5d7825f3-5573-11e9-b47e-066a7a20bac8",
					APIVersion:      "v1",
					ResourceVersion: "1550097",
				},
			},
		},
		Ports: []discoveryv1.EndpointPort{
			{
				Name:     aws.String(""),
				Port:     aws.Int32(6379),
				Protocol: func() *v1.Protocol { p := v1.ProtocolTCP; return &p }(),
			},
		},
	},
	&discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "redis-slave-ghi789",
			GenerateName:    "",
			Namespace:       "default",
			UID:             "8dee375e-5573-11e9-b47e-066a7a20bac8",
			ResourceVersion: "1550242",
			Generation:      0,
			CreationTimestamp: metav1.Time{
				Time: time.Now(),
			},
			Labels: map[string]string{
				"app":                        "redis",
				"role":                       "slave",
				discoveryv1.LabelServiceName: "redis-slave",
				discoveryv1.LabelManagedBy:   "endpointslice-controller.k8s.io",
			},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses: []string{"192.168.186.217"},
				Conditions: discoveryv1.EndpointConditions{
					Ready: aws.Bool(true),
				},
				NodeName: aws.String("ip-192-168-153-1.eu-west-1.compute.internal"),
				TargetRef: &v1.ObjectReference{
					Kind:            "Pod",
					Namespace:       "default",
					Name:            "redis-slave-mdjsj",
					UID:             "8137c74b-5573-11e9-b47e-066a7a20bac8",
					APIVersion:      "v1",
					ResourceVersion: "1550223",
				},
			},
			{
				Addresses: []string{"192.168.68.108"},
				Conditions: discoveryv1.EndpointConditions{
					Ready: aws.Bool(true),
				},
				NodeName: aws.String("ip-192-168-76-61.eu-west-1.compute.internal"),
				TargetRef: &v1.ObjectReference{
					Kind:            "Pod",
					Namespace:       "default",
					Name:            "redis-slave-gtd5x",
					UID:             "813878c3-5573-11e9-b47e-066a7a20bac8",
					APIVersion:      "v1",
					ResourceVersion: "1550226",
				},
			},
			{
				Addresses: []string{"192.168.68.109"},
				Conditions: discoveryv1.EndpointConditions{
					Ready: aws.Bool(true),
				},
				NodeName: aws.String("ip-192-168-76-61.eu-west-1.compute.internal"),
				TargetRef: &v1.ObjectReference{
					Kind:            "Pod",
					Namespace:       "",
					Name:            "",
					UID:             "813878c3-5573-11e9-b47e-077b8b31cbd9",
					APIVersion:      "v1",
					ResourceVersion: "1550226",
				},
			},
		},
		Ports: []discoveryv1.EndpointPort{
			{
				Name:     aws.String(""),
				Port:     aws.Int32(6379),
				Protocol: func() *v1.Protocol { p := v1.ProtocolTCP; return &p }(),
			},
		},
	},
	&discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "kube-controller-manager-jkl012",
			GenerateName:    "",
			Namespace:       "kube-system",
			UID:             "4f77dc4b-4c0a-11e9-b47e-066a7a20bac8",
			ResourceVersion: "6461574",
			Generation:      0,
			CreationTimestamp: metav1.Time{
				Time: time.Now(),
			},
			Annotations: map[string]string{
				"control-plane.alpha.kubernetes.io/leader": "{\"holderIdentity\":\"ip-10-0-189-120.eu-west-1.compute.internal_89407f85-57e1-11e9-b6ea-02eb484bead6\",\"leaseDurationSeconds\":15,\"acquireTime\":\"2019-04-05T20:34:54Z\",\"renewTime\":\"2019-05-06T20:04:02Z\",\"leaderTransitions\":1}",
			},
			Labels: map[string]string{
				discoveryv1.LabelServiceName: "kube-controller-manager",
				discoveryv1.LabelManagedBy:   "endpointslice-controller.k8s.io",
			},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints:   []discoveryv1.Endpoint{},
	},
	&discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "kube-dns-mno345",
			GenerateName:    "",
			Namespace:       "kube-system",
			UID:             "5049bf97-4c0a-11e9-b47e-066a7a20bac8",
			ResourceVersion: "5847",
			Generation:      0,
			CreationTimestamp: metav1.Time{
				Time: time.Now(),
			},
			Labels: map[string]string{
				"eks.amazonaws.com/component":   "kube-dns",
				"k8s-app":                       "kube-dns",
				"kubernetes.io/cluster-service": "true",
				"kubernetes.io/name":            "CoreDNS",
				discoveryv1.LabelServiceName:    "kube-dns",
				discoveryv1.LabelManagedBy:      "endpointslice-controller.k8s.io",
			},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses: []string{"192.168.212.227"},
				Conditions: discoveryv1.EndpointConditions{
					Ready: aws.Bool(true),
				},
				NodeName: aws.String("ip-192-168-200-63.eu-west-1.compute.internal"),
				TargetRef: &v1.ObjectReference{
					Kind:            "Pod",
					Namespace:       "kube-system",
					Name:            "coredns-7554568866-26jdf",
					UID:             "503e1eae-4c0a-11e9-b47e-066a7a20bac8",
					APIVersion:      "v1",
					ResourceVersion: "5842",
				},
			},
			{
				Addresses: []string{"192.168.222.250"},
				Conditions: discoveryv1.EndpointConditions{
					Ready: aws.Bool(true),
				},
				NodeName: aws.String("ip-192-168-200-63.eu-west-1.compute.internal"),
				TargetRef: &v1.ObjectReference{
					Kind:            "Pod",
					Namespace:       "kube-system",
					Name:            "coredns-7554568866-shwn6",
					UID:             "503f9b07-4c0a-11e9-b47e-066a7a20bac8",
					APIVersion:      "v1",
					ResourceVersion: "5839",
				},
			},
		},
		Ports: []discoveryv1.EndpointPort{
			{
				Name:     aws.String("dns"),
				Port:     aws.Int32(53),
				Protocol: func() *v1.Protocol { p := v1.ProtocolUDP; return &p }(),
			},
			{
				Name:     aws.String("dns-tcp"),
				Port:     aws.Int32(53),
				Protocol: func() *v1.Protocol { p := v1.ProtocolTCP; return &p }(),
			},
		},
	},
	&discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "kube-scheduler-pqr678",
			GenerateName:    "",
			Namespace:       "kube-system",
			UID:             "4e8782bc-4c0a-11e9-b47e-066a7a20bac8",
			ResourceVersion: "6461575",
			Generation:      0,
			CreationTimestamp: metav1.Time{
				Time: time.Now(),
			},
			Annotations: map[string]string{
				"control-plane.alpha.kubernetes.io/leader": "{\"holderIdentity\":\"ip-10-0-189-120.eu-west-1.compute.internal_949a4400-57e1-11e9-a7bb-02eb484bead6\",\"leaseDurationSeconds\":15,\"acquireTime\":\"2019-04-05T20:34:57Z\",\"renewTime\":\"2019-05-06T20:04:02Z\",\"leaderTransitions\":1}",
			},
			Labels: map[string]string{
				discoveryv1.LabelServiceName: "kube-scheduler",
				discoveryv1.LabelManagedBy:   "endpointslice-controller.k8s.io",
			},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints:   []discoveryv1.Endpoint{},
	},
}

func setUpEndpointClient() (*epClient, chan struct{}) {
	stopChan := make(chan struct{})

	client := &epClient{
		stopChan: stopChan,
		store:    NewObjStore(transformFuncEndpoint, zap.NewNop()),
	}
	return client, stopChan
}

func TestEpClient_PodKeyToServiceNames(t *testing.T) {
	client, stopChan := setUpEndpointClient()
	defer close(stopChan)
	arrays := make([]any, len(endpointsArray))
	for i := range arrays {
		arrays[i] = endpointsArray[i]
	}
	assert.NoError(t, client.store.Replace(convertToInterfaceArray(endpointsArray), ""))

	expectedMap := map[string][]string{
		"namespace:default,podName:redis-master-rh2bd":           {"redis-master"},
		"namespace:default,podName:redis-slave-mdjsj":            {"redis-slave"},
		"namespace:default,podName:redis-slave-gtd5x":            {"redis-slave"},
		"namespace:kube-system,podName:coredns-7554568866-26jdf": {"kube-dns"},
		"namespace:kube-system,podName:coredns-7554568866-shwn6": {"kube-dns"},
		"namespace:default,podName:guestbook-qjqnz":              {"guestbook"},
		"namespace:default,podName:guestbook-92wmq":              {"guestbook"},
		"namespace:default,podName:guestbook-qbdv8":              {"guestbook"},
	}
	resultMap := client.PodKeyToServiceNames()
	log.Printf("PodKeyToServiceNames (len=%v): %v", len(resultMap), resultMap)
	assert.Equal(t, expectedMap, resultMap)
}

func TestEpClient_ServiceNameToPodNum(t *testing.T) {
	client, stopChan := setUpEndpointClient()

	assert.NoError(t, client.store.Replace(convertToInterfaceArray(endpointsArray), ""))

	expectedMap := map[Service]int{
		NewService("redis-slave", "default"):  2,
		NewService("kube-dns", "kube-system"): 2,
		NewService("redis-master", "default"): 1,
		NewService("guestbook", "default"):    3,
	}
	resultMap := client.ServiceToPodNum()
	log.Printf("ServiceNameToPodNum (len=%v): %v", len(resultMap), resultMap)
	assert.Equal(t, expectedMap, resultMap)
	client.shutdown()
	time.Sleep(2 * time.Millisecond)
	select {
	case <-stopChan:
	default:
		t.Error("The shutdown channel is not closed")
	}
}

func TestTransformFuncEndpoint(t *testing.T) {
	info, err := transformFuncEndpoint(nil)
	assert.Nil(t, info)
	assert.Error(t, err)

	// Test with a valid EndpointSlice
	validSlice := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-slice",
			Namespace: "default",
			Labels: map[string]string{
				discoveryv1.LabelServiceName: "test-service",
			},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses: []string{"192.168.1.1"},
				Conditions: discoveryv1.EndpointConditions{
					Ready: aws.Bool(true),
				},
				TargetRef: &v1.ObjectReference{
					Kind:      "Pod",
					Namespace: "default",
					Name:      "test-pod",
				},
			},
		},
	}

	result, err := transformFuncEndpoint(validSlice)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	epInfo := result.(*endpointInfo)
	assert.Equal(t, "test-service", epInfo.name)
	assert.Equal(t, "default", epInfo.namespace)
	assert.Len(t, epInfo.podKeyList, 1)
	assert.Equal(t, "namespace:default,podName:test-pod", epInfo.podKeyList[0])
}

func TestNewEndpointClient(t *testing.T) {
	if goruntime.GOOS == "windows" {
		t.Skip("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/38903")
	}
	setKubeConfigPath(t)
	setOption := epSyncCheckerOption(&mockReflectorSyncChecker{})

	fakeClientSet := fake.NewClientset(endpointsArray...)
	client := newEpClient(fakeClientSet, zap.NewNop(), setOption)
	assert.NotNil(t, client)
	client.shutdown()
	removeTempKubeConfig()
}
