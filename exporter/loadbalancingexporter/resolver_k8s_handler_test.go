// FILEPATH: /Users/matt.hoey/ml/tp-opentelemetry-collector-contrib/exporter/loadbalancingexporter/resolver_k8s_handler_test.go

package loadbalancingexporter

import (
  "testing"

  "github.com/stretchr/testify/assert"
  corev1 "k8s.io/api/core/v1"
  metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)


func TestConvertToEndpoints(tst *testing.T) {
  // Create dummy Endpoints objects
  endpoints1 := &corev1.Endpoints{
    ObjectMeta: metav1.ObjectMeta{
      Name:      "test-endpoints-1",
      Namespace: "test-namespace",
    },
    Subsets: []corev1.EndpointSubset{
      {
        Addresses: []corev1.EndpointAddress{
          {
            Hostname: "pod-1",
            IP: "192.168.10.101",
          },
        },
      },
    },
  }
  endpoints2 := &corev1.Endpoints{
    ObjectMeta: metav1.ObjectMeta{
      Name:      "test-endpoints-2",
      Namespace: "test-namespace",
    },
    Subsets: []corev1.EndpointSubset{
      {
        Addresses: []corev1.EndpointAddress{
          {
            Hostname: "pod-2",
            IP: "192.168.10.102",
          },
        },
      },
    },
  }
  endpoints3 := &corev1.Endpoints{
    ObjectMeta: metav1.ObjectMeta{
      Name:      "test-endpoints-3",
      Namespace: "test-namespace",
    },
    Subsets: []corev1.EndpointSubset{
      {
        Addresses: []corev1.EndpointAddress{
          {
            IP: "192.168.10.103",
          },
        },
      },
    },
  }

  tests := []struct {
    name string
    returnNames bool
    includedEndpoints []*corev1.Endpoints
    expectedEndpoints []string
    wantNil bool
  }{
    {
      name: "return hostnames",
      returnNames: true,
      includedEndpoints: []*corev1.Endpoints{endpoints1, endpoints2},
      expectedEndpoints: []string{"pod-1", "pod-2"},
      wantNil: false,
    },
    {
      name: "return IPs",
      returnNames: false,
      includedEndpoints: []*corev1.Endpoints{endpoints1, endpoints2, endpoints3},
      expectedEndpoints: []string{"192.168.10.101", "192.168.10.102", "192.168.10.103"},
      wantNil: false,
    },
    {
      name: "missing hostname",
      returnNames: true,
      includedEndpoints: []*corev1.Endpoints{endpoints1, endpoints3},
      expectedEndpoints: nil,
      wantNil: true,
    },
  }

  for _, tt := range tests {
    tst.Run(tt.name, func(tst *testing.T) {
      ok, res := convertToEndpoints(tt.returnNames, tt.includedEndpoints...)
      if tt.wantNil {
        assert.Nil(tst, res)
      } else {
        assert.Equal(tst, tt.expectedEndpoints, res)
      }
      assert.Equal(tst, !tt.wantNil, ok)
    })
  }

}
