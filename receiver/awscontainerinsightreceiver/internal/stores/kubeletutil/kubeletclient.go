// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kubeletutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores/kubeletutil"

import (
	"encoding/json"
	"fmt"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kubelet"
)

type KubeletClient struct {
	KubeIP     string
	Port       string
	restClient kubelet.Client
}

func NewKubeletClient(kubeIP string, port string, logger *zap.Logger) (*KubeletClient, error) {
	kubeClient := &KubeletClient{
		Port:   port,
		KubeIP: kubeIP,
	}

	endpoint := kubeIP + ":" + port

	// use service account for authentication
	clientConfig := &kubelet.ClientConfig{
		APIConfig: k8sconfig.APIConfig{
			AuthType: k8sconfig.AuthTypeServiceAccount,
		},
	}

	clientProvider, err := kubelet.NewClientProvider(endpoint, clientConfig, logger)
	if err != nil {
		return nil, err
	}
	client, err := clientProvider.BuildClient()
	if err != nil {
		return nil, err
	}
	kubeClient.restClient = client
	return kubeClient, nil
}

func (k *KubeletClient) ListPods() ([]corev1.Pod, error) {
	var result []corev1.Pod
	b, err := k.restClient.Get("/pods")
	if err != nil {
		return result, fmt.Errorf("call to /pods endpoint failed: %w", err)
	}

	pods := corev1.PodList{}
	err = json.Unmarshal(b, &pods)
	if err != nil {
		return result, fmt.Errorf("parsing response failed: %w", err)
	}

	return pods.Items, nil
}
