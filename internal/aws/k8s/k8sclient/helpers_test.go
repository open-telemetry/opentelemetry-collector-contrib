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

package k8sclient

import (
	"os"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
)

type mockReflectorSyncChecker struct {
}

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
	if err != nil {
		t.Error(err)
	}
	if err := os.WriteFile(tmpfile.Name(), []byte(content), 0600); err != nil {
		t.Error(err)
	}
	// overwrite the default kube config path
	kubeConfigPath = tmpfile.Name()
	return kubeConfigPath
}

func removeTempKubeConfig() {
	os.Remove(kubeConfigPath)
	kubeConfigPath = ""
}

func convertToInterfaceArray(objArray []runtime.Object) []interface{} {
	array := make([]interface{}, len(objArray))
	for i := range array {
		array[i] = objArray[i]
	}
	return array
}
