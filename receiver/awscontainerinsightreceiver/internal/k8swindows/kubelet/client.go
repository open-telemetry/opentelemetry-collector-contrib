// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package kubelet // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/k8swindows/kubelet"

import (
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	stats "k8s.io/kubelet/pkg/apis/stats/v1alpha1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores/kubeletutil"
)

// KubeletProvider Represents interface to kubelet.
type KubeletProvider interface { //nolint:revive
	GetSummary() (*stats.Summary, error)
	GetPods() ([]corev1.Pod, error)
}

type kubeletProvider struct {
	logger   *zap.Logger
	hostIP   string
	hostPort string
	client   *kubeletutil.KubeletClient
}

// getClient Returns singleton kubelet client.
func (kp *kubeletProvider) getClient() (*kubeletutil.KubeletClient, error) {
	if kp.client != nil {
		return kp.client, nil
	}
	kclient, err := kubeletutil.NewKubeletClient(kp.hostIP, kp.hostPort, nil, kp.logger)
	if err != nil {
		kp.logger.Error("failed to initialize new kubelet client, ", zap.Error(err))
		return nil, err
	}
	kp.client = kclient
	return kclient, nil
}

// GetSummary Get Summary from kubelet API.
func (kp *kubeletProvider) GetSummary() (*stats.Summary, error) {
	kclient, err := kp.getClient()
	if err != nil {
		kp.logger.Error("failed to get kubelet client, ", zap.Error(err))
		return nil, err
	}

	summary, err := kclient.Summary(kp.logger)
	if err != nil {
		kp.logger.Error("failure from kubelet on getting summary, ", zap.Error(err))
		return nil, err
	}
	return summary, nil
}

func (kp *kubeletProvider) GetPods() ([]corev1.Pod, error) {
	kclient, err := kp.getClient()
	if err != nil {
		kp.logger.Error("failed to get kubelet client, ", zap.Error(err))
		return nil, err
	}
	return kclient.ListPods()
}
