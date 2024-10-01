// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8snode

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

func TestNewProvider(t *testing.T) {
	// set k8s cluster env variables to make the API client happy
	t.Setenv("KUBERNETES_SERVICE_HOST", "127.0.0.1")
	t.Setenv("KUBERNETES_SERVICE_PORT", "6443")

	_, err := NewProvider("nodeName", k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeNone})
	assert.NoError(t, err)
	_, err = NewProvider("", k8sconfig.APIConfig{AuthType: k8sconfig.AuthTypeNone})
	assert.EqualError(t, err, "nodeName can't be empty")
}

func TestNodeUID(t *testing.T) {
	client := fake.NewSimpleClientset()
	err := setupNodes(client)
	assert.NoError(t, err)

	tests := []struct {
		testName string
		nodeName string
		nodeUID  string
		errMsg   string
	}{
		{
			testName: "valid node name",
			nodeName: "1",
			nodeUID:  "node1",
			errMsg:   "",
		},
		{
			testName: "node does not exist",
			nodeName: "5",
			nodeUID:  "",
			errMsg:   "failed to fetch node with name 5 from K8s API: nodes \"5\" not found",
		},
		{
			testName: "node name not set",
			nodeName: "",
			nodeUID:  "",
			errMsg:   "failed to fetch node with name  from K8s API: nodes \"\" not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			k8snodeP := &k8snodeProvider{
				k8snodeClient: client,
				nodeName:      tt.nodeName,
			}
			nodeUID, err := k8snodeP.NodeUID(context.Background())
			if tt.errMsg != "" {
				assert.EqualError(t, err, tt.errMsg)
			} else {
				assert.Equal(t, nodeUID, tt.nodeUID)
				nodeName, err := k8snodeP.NodeName(context.Background())
				assert.NoError(t, err)
				assert.Equal(t, nodeName, tt.nodeName)
			}
		})
	}
}

func setupNodes(client *fake.Clientset) error {
	for i := 0; i < 3; i++ {
		n := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				UID:  types.UID("node" + strconv.Itoa(i)),
				Name: strconv.Itoa(i),
			},
		}
		_, err := client.CoreV1().Nodes().Create(context.Background(), n, metav1.CreateOptions{})
		if err != nil {
			return err
		}
		time.Sleep(2 * time.Millisecond)
	}
	return nil
}
