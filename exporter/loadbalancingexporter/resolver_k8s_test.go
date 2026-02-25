// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"context"
	"fmt"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func init() {
	// Disable WatchListClient feature gate for tests as fake clientset doesn't support bookmark events
	// See: https://github.com/kubernetes/kubernetes/issues/129408
	os.Setenv("KUBE_FEATURE_WatchListClient", "false")
}

func TestK8sResolve(t *testing.T) {
	type args struct {
		logger          *zap.Logger
		service         string
		ports           []int32
		namespace       string
		returnHostnames bool
	}
	type suiteContext struct {
		endpoint  *discoveryv1.EndpointSlice
		clientset *fake.Clientset
		resolver  *k8sResolver
	}
	hostname := "pod-0"
	hostname1 := "pod-1"
	setupSuite := func(t *testing.T, args args) (*suiteContext, func(*testing.T)) {
		service, defaultNs, ports, returnHostnames := args.service, args.namespace, args.ports, args.returnHostnames
		endpoint := &discoveryv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{
				Name:      service,
				Namespace: defaultNs,
				Labels: map[string]string{
					"kubernetes.io/service-name": service,
				},
			},
			Endpoints: []discoveryv1.Endpoint{
				{
					Addresses: []string{"192.168.10.100"},
					Hostname:  &hostname,
				},
			},
		}
		var expectInit []string
		for _, endpoint := range endpoint.Endpoints {
			for _, address := range endpoint.Addresses {
				for _, port := range args.ports {
					if returnHostnames {
						expectInit = append(expectInit, fmt.Sprintf("%s.%s.%s:%d", *endpoint.Hostname, service, defaultNs, port))
					} else {
						expectInit = append(expectInit, fmt.Sprintf("%s:%d", address, port))
					}
				}
			}
		}

		cl := fake.NewClientset(endpoint)
		_, tb := getTelemetryAssets(t)
		res, err := newK8sResolver(cl, zap.NewNop(), service, ports, defaultListWatchTimeout, returnHostnames, tb)
		require.NoError(t, err)

		require.NoError(t, res.start(t.Context()))
		// Wait for the initial endpoints to be populated by the informer
		// The informer cache sync only guarantees the cache is ready, but the OnAdd
		// handler runs asynchronously and may not have completed yet
		cErr := waitForCondition(t, 3*time.Second, 20*time.Millisecond, func(ctx context.Context) (bool, error) {
			if _, resErr := res.resolve(ctx); resErr != nil {
				return false, resErr
			}
			got := res.Endpoints()
			return slices.Equal(expectInit, got), nil
		})
		if cErr != nil {
			t.Logf("waitForCondition: timed out waiting for initial resolver endpoints: %v", cErr)
		}
		// verify endpoints should be the same as expectInit
		assert.NoError(t, err)
		assert.Equal(t, expectInit, res.Endpoints())

		return &suiteContext{
				endpoint:  endpoint,
				clientset: cl,
				resolver:  res,
			}, func(*testing.T) {
				require.NoError(t, res.shutdown(t.Context()))
			}
	}
	tests := []struct {
		name              string
		args              args
		simulateFn        func(*suiteContext, args) error
		onChangeFn        func([]string)
		expectedEndpoints []string
	}{
		{
			name: "add new IP to existing backends",
			args: args{
				logger:    zap.NewNop(),
				service:   "lb",
				namespace: "default",
				ports:     []int32{8080, 9090},
			},
			simulateFn: func(suiteCtx *suiteContext, args args) error {
				endpoint, exist := suiteCtx.endpoint.DeepCopy(), suiteCtx.endpoint.DeepCopy()
				endpoint.Endpoints = append(endpoint.Endpoints, discoveryv1.Endpoint{Addresses: []string{"10.10.0.11"}})
				patch := client.MergeFrom(exist)
				data, err := patch.Data(endpoint)
				if err != nil {
					return err
				}
				_, err = suiteCtx.clientset.DiscoveryV1().EndpointSlices(args.namespace).
					Patch(t.Context(), args.service, types.MergePatchType, data, metav1.PatchOptions{})
				return err
			},
			expectedEndpoints: []string{
				"10.10.0.11:8080",
				"10.10.0.11:9090",
				"192.168.10.100:8080",
				"192.168.10.100:9090",
			},
		},
		{
			name: "simulate re-list that does not change endpoints",
			args: args{
				logger:    zap.NewNop(),
				service:   "lb",
				namespace: "default",
				ports:     []int32{8080, 9090},
			},
			simulateFn: func(suiteCtx *suiteContext, args args) error {
				exist := suiteCtx.endpoint.DeepCopy()
				patch := client.MergeFrom(exist)
				data, err := patch.Data(exist)
				if err != nil {
					return err
				}
				_, err = suiteCtx.clientset.DiscoveryV1().EndpointSlices(args.namespace).
					Patch(t.Context(), args.service, types.MergePatchType, data, metav1.PatchOptions{})
				return err
			},
			onChangeFn: func([]string) {
				assert.Fail(t, "should not call onChange")
			},
			expectedEndpoints: []string{
				"192.168.10.100:8080",
				"192.168.10.100:9090",
			},
		},
		{
			name: "add new hostname to existing backends",
			args: args{
				logger:          zap.NewNop(),
				service:         "lb",
				namespace:       "default",
				ports:           []int32{8080, 9090},
				returnHostnames: true,
			},
			simulateFn: func(suiteCtx *suiteContext, args args) error {
				endpoint, exist := suiteCtx.endpoint.DeepCopy(), suiteCtx.endpoint.DeepCopy()
				endpoint.Endpoints = append(endpoint.Endpoints, discoveryv1.Endpoint{Addresses: []string{"10.10.0.11"}, Hostname: &hostname1})
				patch := client.MergeFrom(exist)
				data, err := patch.Data(endpoint)
				if err != nil {
					return err
				}
				_, err = suiteCtx.clientset.DiscoveryV1().EndpointSlices(args.namespace).
					Patch(t.Context(), args.service, types.MergePatchType, data, metav1.PatchOptions{})
				return err
			},
			expectedEndpoints: []string{
				"pod-0.lb.default:8080",
				"pod-0.lb.default:9090",
				"pod-1.lb.default:8080",
				"pod-1.lb.default:9090",
			},
		},
		{
			name: "change existing backend ip address",
			args: args{
				logger:    zap.NewNop(),
				service:   "lb",
				namespace: "default",
				ports:     []int32{4317},
			},
			simulateFn: func(suiteCtx *suiteContext, args args) error {
				endpoint, exist := suiteCtx.endpoint.DeepCopy(), suiteCtx.endpoint.DeepCopy()
				endpoint.Endpoints = []discoveryv1.Endpoint{
					{Addresses: []string{"10.10.0.11"}},
				}
				patch := client.MergeFrom(exist)
				data, err := patch.Data(endpoint)
				if err != nil {
					return err
				}
				_, err = suiteCtx.clientset.DiscoveryV1().EndpointSlices(args.namespace).
					Patch(t.Context(), args.service, types.MergePatchType, data, metav1.PatchOptions{})
				return err
			},
			expectedEndpoints: []string{
				"10.10.0.11:4317",
			},
		},
		{
			name: "simulate deletion of backends",
			args: args{
				logger:    zap.NewNop(),
				service:   "lb",
				namespace: "default",
				ports:     []int32{8080, 9090},
			},
			simulateFn: func(suiteCtx *suiteContext, args args) error {
				return suiteCtx.clientset.DiscoveryV1().EndpointSlices(args.namespace).
					Delete(t.Context(), args.service, metav1.DeleteOptions{})
			},
			expectedEndpoints: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suiteCtx, teardownSuite := setupSuite(t, tt.args)
			defer teardownSuite(t)

			if tt.onChangeFn != nil {
				suiteCtx.resolver.onChange(tt.onChangeFn)
			}

			err := tt.simulateFn(suiteCtx, tt.args)
			assert.NoError(t, err)

			slices.Sort(tt.expectedEndpoints)

			cErr := waitForCondition(t, 1200*time.Millisecond, 20*time.Millisecond, func(ctx context.Context) (bool, error) {
				if _, err := suiteCtx.resolver.resolve(ctx); err != nil {
					return false, err
				}
				got := suiteCtx.resolver.Endpoints()
				return slices.Equal(tt.expectedEndpoints, got), nil
			})
			if cErr != nil {
				t.Logf("waitForCondition: timed out waiting for resolver endpoints to match expected: %v", cErr)
			}
			assert.Equal(t, tt.expectedEndpoints, suiteCtx.resolver.Endpoints(), "resolver returned unexpected endpoints after update")
		})
	}
}

func TestK8sResolveWithServiceFQDN(t *testing.T) {
	serviceName := "lb"
	namespace := "custom"
	serviceFQDN := fmt.Sprintf("%s.%s.svc.cluster.local", serviceName, namespace)
	port := int32(4317)
	hostname := "pod-0"

	endpointSlice := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: namespace,
			Labels: map[string]string{
				"kubernetes.io/service-name": serviceName,
			},
		},
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses: []string{"10.0.0.1"},
				Hostname:  &hostname,
			},
		},
	}

	cl := fake.NewClientset(endpointSlice)
	_, tb := getTelemetryAssets(t)
	res, err := newK8sResolver(cl, zap.NewNop(), serviceFQDN, []int32{port}, defaultListWatchTimeout, true, tb)
	require.NoError(t, err)
	require.Equal(t, serviceName, res.svcName)
	require.Equal(t, namespace, res.svcNs)

	require.NoError(t, res.start(t.Context()))
	t.Cleanup(func() {
		require.NoError(t, res.shutdown(t.Context()))
	})

	expected := []string{fmt.Sprintf("%s.%s.%s:%d", hostname, serviceName, namespace, port)}

	cErr := waitForCondition(t, 3*time.Second, 20*time.Millisecond, func(ctx context.Context) (bool, error) {
		if _, err := res.resolve(ctx); err != nil {
			return false, err
		}
		return slices.Equal(expected, res.Endpoints()), nil
	})
	if cErr != nil {
		t.Fatalf("timed out waiting for resolver endpoints to match expected: %v", cErr)
	}
}

// waitForCondition will poll the condition function until it returns true or times out.
// Any errors returned from the condition are treated as test failures.
func waitForCondition(t *testing.T, timeout, interval time.Duration, condition func(context.Context) (bool, error)) error {
	ctx, cancel := context.WithTimeout(t.Context(), timeout)
	defer cancel()
	t.Helper()

	return wait.PollUntilContextTimeout(ctx, interval, timeout, true, condition)
}

func Test_newK8sResolver(t *testing.T) {
	type args struct {
		logger  *zap.Logger
		service string
		ports   []int32
	}
	tests := []struct {
		name          string
		args          args
		wantNil       bool
		wantErr       error
		wantService   string
		wantNamespace string
	}{
		{
			name: "invalid name of k8s service",
			args: args{
				logger:  zap.NewNop(),
				service: "",
				ports:   []int32{8080},
			},
			wantNil: true,
			wantErr: errNoSvc,
		},
		{
			name: "use `default` namespace if namespace is not specified",
			args: args{
				logger:  zap.NewNop(),
				service: "lb",
				ports:   []int32{8080},
			},
			wantNil:       false,
			wantErr:       nil,
			wantService:   "lb",
			wantNamespace: "default",
		},
		{
			name: "use specified namespace",
			args: args{
				logger:  zap.NewNop(),
				service: "lb.kube-public",
				ports:   []int32{8080},
			},
			wantNil:       false,
			wantErr:       nil,
			wantService:   "lb",
			wantNamespace: "kube-public",
		},
		{
			name: "reject service FQDN with unexpected labels",
			args: args{
				logger:  zap.NewNop(),
				service: "lb.kube-public.foo.bar",
				ports:   []int32{8080},
			},
			wantNil: true,
			wantErr: errInvalidSvcFQDN,
		},
		{
			name: "reject service FQDN without cluster domain",
			args: args{
				logger:  zap.NewNop(),
				service: "lb.kube-public.svc",
				ports:   []int32{8080},
			},
			wantNil: true,
			wantErr: errInvalidSvcFQDN,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, tb := getTelemetryAssets(t)
			got, err := newK8sResolver(fake.NewClientset(), tt.args.logger, tt.args.service, tt.args.ports, defaultListWatchTimeout, false, tb)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.wantNil, got == nil)
				if !tt.wantNil {
					require.Equal(t, tt.wantService, got.svcName)
					require.Equal(t, tt.wantNamespace, got.svcNs)
				}
			}
		})
	}
}
