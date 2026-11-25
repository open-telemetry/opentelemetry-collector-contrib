// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"

import (
	"errors"
	"os"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig/k8sconfigtypes"
)

// ErrNotInKubernetes reports that the Kubernetes API cannot be reached because the
// collector is not running in a Kubernetes pod.
var ErrNotInKubernetes = errors.New("not running in Kubernetes: KUBERNETES_SERVICE_HOST and KUBERNETES_SERVICE_PORT are not set")

// CheckInClusterEnv returns ErrNotInKubernetes when apiConf can only reach the API
// server from inside a pod and the in-cluster service environment is missing. Only
// auth_type kubeConfig can reach a cluster from outside; every other auth type builds
// its client from KUBERNETES_SERVICE_HOST and KUBERNETES_SERVICE_PORT.
func CheckInClusterEnv(apiConf k8sconfigtypes.APIConfig) error {
	if apiConf.AuthType == k8sconfigtypes.AuthTypeKubeConfig {
		return nil
	}
	if os.Getenv("KUBERNETES_SERVICE_HOST") == "" || os.Getenv("KUBERNETES_SERVICE_PORT") == "" {
		return ErrNotInKubernetes
	}
	return nil
}
