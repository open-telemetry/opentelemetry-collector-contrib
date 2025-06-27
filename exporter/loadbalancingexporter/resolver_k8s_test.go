// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"
)

func TestK8sResolve(t *testing.T) {
	type args struct {
		logger          *zap.Logger
		service         string
		ports           []int32
		namespace       string
		returnHostnames bool
	}

	type suiteContext struct {
		endpointSlice *discoveryv1.EndpointSlice
		clientset     *fake.Clientset
		resolver      *k8sResolver
	}

	setupSuite := func(t *testing.T, args args) (*suiteContext, func(*testing.T)) {
		service, defaultNs, ports, returnHostnames := args.service, args.namespace, args.ports, args.returnHostnames

		endpointSlice := &discoveryv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "lb-slice",
				Namespace: defaultNs,
				Labels: map[string]string{
					discoveryv1.LabelServiceName: args.service,
				},
			},
			AddressType: discoveryv1.AddressTypeIPv4,
			Endpoints: []discoveryv1.Endpoint{
				{
					Addresses: []string{"10.10.0.11"},
					Hostname:  ptr.To("pod-0"),
				},
				{
					Addresses: []string{"192.168.10.100"},
					Hostname:  ptr.To("pod-1"),
				},
			},
			Ports: []discoveryv1.EndpointPort{
				{
					Name:     ptr.To("http"),
					Port:     ptr.To(int32(8080)),
					Protocol: ptr.To(corev1.ProtocolTCP),
				},
				{
					Name:     ptr.To("other"),
					Port:     ptr.To(int32(9090)),
					Protocol: ptr.To(corev1.ProtocolTCP),
				},
			},
		}

		var expectInit []string
		for _, ep := range endpointSlice.Endpoints {
			for _, port := range endpointSlice.Ports {
				if returnHostnames && ep.Hostname != nil {
					expectInit = append(expectInit,
						fmt.Sprintf("%s.%s.%s:%d", *ep.Hostname, service, defaultNs, *port.Port))
				} else {
					expectInit = append(expectInit,
						fmt.Sprintf("%s:%d", ep.Addresses[0], *port.Port))
				}
			}
		}

		cl := fake.NewClientset(endpointSlice)

		_, tb := getTelemetryAssets(t)
		res, err := newK8sResolver(cl, zap.NewNop(), service, ports, defaultListWatchTimeout, returnHostnames, tb)
		require.NoError(t, err)

		require.NoError(t, res.start(context.Background()))
		// verify endpoints should be the same as expectInit
		assert.NoError(t, err)
		assert.Equal(t, expectInit, res.Endpoints())

		return &suiteContext{
				endpointSlice: endpointSlice,
				clientset:     cl,
				resolver:      res,
			}, func(*testing.T) {
				require.NoError(t, res.shutdown(context.Background()))
			}
	}

	tests := []struct {
		name       string
		args       args
		simulateFn func(*suiteContext, args) error
		onChangeFn func([]string)
		verifyFn   func(*suiteContext, args) error
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
				// DeepCopy the existing EndpointSlice
				updated := suiteCtx.endpointSlice.DeepCopy()
				// Append a new Endpoint
				updated.Endpoints = append(updated.Endpoints, discoveryv1.Endpoint{
					Addresses: []string{"10.10.0.11"},
					Hostname:  ptr.To("pod-1"),
				})
				// Update in fake client
				_, err := suiteCtx.clientset.DiscoveryV1().EndpointSlices(args.namespace).
					Update(context.TODO(), updated, metav1.UpdateOptions{})
				return err
			},
			verifyFn: func(ctx *suiteContext, _ args) error {
				// Force resolver to re-resolve
				if _, err := ctx.resolver.resolve(context.Background()); err != nil {
					return err
				}

				assert.Equal(t, []string{
					"10.10.0.11:8080",
					"10.10.0.11:9090",
					"192.168.10.100:8080",
					"192.168.10.100:9090",
				}, ctx.resolver.Endpoints(), "resolver failed, endpoints not equal")

				return nil
			},
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

			assert.Eventually(t, func() bool {
				err := tt.verifyFn(suiteCtx, tt.args)
				assert.NoError(t, err)
				return true
			}, time.Second, 20*time.Millisecond)
		})
	}
}
