// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sclient

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
)

type mockReflectorSyncChecker struct{}

func (m *mockReflectorSyncChecker) Check(_ cacheReflector, _ string) {
}

var kubeConfigPath string

func setKubeConfigPath(t *testing.T) string {
	content := `
apiVersion: v1
clusters:
- cluster:
    server: https://localhost:8080
    extensions:
    - name: client.authentication.k8s.io/exec
      extension:
        audience: foo
        other: bar
  name: foo-cluster
contexts:
- context:
    cluster: foo-cluster
    user: foo-user
    namespace: bar
  name: foo-context
current-context: foo-context
kind: Config
users:
- name: foo-user
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      args:
      - arg-1
      - arg-2
      command: foo-command
      provideClusterInfo: true
`
	tmpfile, err := os.CreateTemp("", "kubeconfig")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(tmpfile.Name(), []byte(content), 0o600))
	// overwrite the default kube config path
	kubeConfigPath = tmpfile.Name()
	return kubeConfigPath
}

func removeTempKubeConfig() {
	os.Remove(kubeConfigPath)
	kubeConfigPath = ""
}

func convertToInterfaceArray(objArray []runtime.Object) []any {
	array := make([]any, len(objArray))
	for i := range array {
		array[i] = objArray[i]
	}
	return array
}
