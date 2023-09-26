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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestK8sResolve(t *testing.T) {
	type args struct {
		logger    *zap.Logger
		service   string
		ports     []int32
		namespace string
	}
	type suiteContext struct {
		endpoint  *corev1.Endpoints
		clientset *fake.Clientset
		resolver  *k8sResolver
	}
	setupSuite := func(t *testing.T, args args) (*suiteContext, func(*testing.T)) {
		service, defaultNs, ports := args.service, args.namespace, args.ports
		endpoint := &corev1.Endpoints{
			ObjectMeta: metav1.ObjectMeta{
				Name:      service,
				Namespace: defaultNs,
			},
			Subsets: []corev1.EndpointSubset{
				{
					Addresses: []corev1.EndpointAddress{
						{IP: "192.168.10.100"},
					},
				},
			},
		}
		var expectInit []string
		for _, subset := range endpoint.Subsets {
			for _, address := range subset.Addresses {
				for _, port := range args.ports {
					expectInit = append(expectInit, fmt.Sprintf("%s:%d", address.IP, port))
				}
			}
		}

		cl := fake.NewSimpleClientset(endpoint)
		res, err := newK8sResolver(cl, zap.NewNop(), service, ports)
		require.NoError(t, err)

		require.NoError(t, res.start(context.Background()))
		// verify endpoints should be the same as expectInit
		assert.NoError(t, err)
		assert.Equal(t, expectInit, res.Endpoints())

		return &suiteContext{
				endpoint:  endpoint,
				clientset: cl,
				resolver:  res,
			}, func(*testing.T) {
				require.NoError(t, res.shutdown(context.Background()))
			}
	}
	tests := []struct {
		name       string
		args       args
		simulateFn func(*suiteContext, args) error
		verifyFn   func(*suiteContext, args) error
	}{
		{
			name: "simulate append the backend ip address",
			args: args{
				logger:    zap.NewNop(),
				service:   "lb",
				namespace: "default",
				ports:     []int32{8080, 9090},
			},
			simulateFn: func(suiteCtx *suiteContext, args args) error {
				endpoint, exist := suiteCtx.endpoint.DeepCopy(), suiteCtx.endpoint.DeepCopy()
				endpoint.Subsets = append(endpoint.Subsets, corev1.EndpointSubset{
					Addresses: []corev1.EndpointAddress{{IP: "10.10.0.11"}},
				})
				patch := client.MergeFrom(exist)
				data, err := patch.Data(endpoint)
				if err != nil {
					return err
				}
				_, err = suiteCtx.clientset.CoreV1().Endpoints(args.namespace).
					Patch(context.TODO(), args.service, types.MergePatchType, data, metav1.PatchOptions{})
				return err

			},
			verifyFn: func(ctx *suiteContext, args args) error {
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
		{
			name: "simulate change the backend ip address",
			args: args{
				logger:    zap.NewNop(),
				service:   "lb",
				namespace: "default",
				ports:     []int32{4317},
			},
			simulateFn: func(suiteCtx *suiteContext, args args) error {
				endpoint, exist := suiteCtx.endpoint.DeepCopy(), suiteCtx.endpoint.DeepCopy()
				endpoint.Subsets = []corev1.EndpointSubset{
					{Addresses: []corev1.EndpointAddress{{IP: "10.10.0.11"}}},
				}
				patch := client.MergeFrom(exist)
				data, err := patch.Data(endpoint)
				if err != nil {
					return err
				}
				_, err = suiteCtx.clientset.CoreV1().Endpoints(args.namespace).
					Patch(context.TODO(), args.service, types.MergePatchType, data, metav1.PatchOptions{})
				return err

			},
			verifyFn: func(ctx *suiteContext, args args) error {
				if _, err := ctx.resolver.resolve(context.Background()); err != nil {
					return err
				}

				assert.Equal(t, []string{
					"10.10.0.11:4317",
				}, ctx.resolver.Endpoints(), "resolver failed, endpoints not equal")

				return nil
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
				return suiteCtx.clientset.CoreV1().Endpoints(args.namespace).
					Delete(context.TODO(), args.service, metav1.DeleteOptions{})
			},
			verifyFn: func(suiteCtx *suiteContext, args args) error {
				if _, err := suiteCtx.resolver.resolve(context.Background()); err != nil {
					return err
				}
				assert.Empty(t, suiteCtx.resolver.Endpoints(), "resolver failed, endpoints should empty")
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suiteCtx, teardownSuite := setupSuite(t, tt.args)
			defer teardownSuite(t)

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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newK8sResolver(fake.NewSimpleClientset(), tt.args.logger, tt.args.service, tt.args.ports)
			if tt.wantErr != nil {
				require.Error(t, err, tt.wantErr)
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
