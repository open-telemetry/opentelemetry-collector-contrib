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

package testutils // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/cadvisor/testutils"

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	cinfo "github.com/google/cadvisor/info/v1"
	"github.com/stretchr/testify/assert"
)

func LoadContainerInfo(t *testing.T, file string) []*cinfo.ContainerInfo {
	info, err := os.ReadFile(file)
	assert.Nil(t, err, "Fail to read file content")

	var result []*cinfo.ContainerInfo
	containers := map[string]*cinfo.ContainerInfo{}
	err = json.Unmarshal(info, &containers)
	assert.Nil(t, err, "Fail to parse json string")

	for _, containerInfo := range containers {
		result = append(result, containerInfo)
	}

	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	assert.NoError(t, enc.Encode(result))
	return result
}

type MockCPUMemInfo struct {
}

func (m MockCPUMemInfo) GetNumCores() int64 {
	return 2
}

func (m MockCPUMemInfo) GetMemoryCapacity() int64 {
	return 1073741824
}

type MockHostInfo struct {
	MockCPUMemInfo
	ClusterName string
	InstanceIP  string
}

func (m MockHostInfo) GetClusterName() string {
	return m.ClusterName
}

func (m MockHostInfo) GetEBSVolumeID(string) string {
	return "ebs-volume-id"
}

func (m MockHostInfo) GetInstanceID() string {
	return "instance-id"
}

func (m MockHostInfo) GetInstanceType() string {
	return "instance-id"
}

func (m MockHostInfo) GetAutoScalingGroupName() string {
	return "asg"
}

func (m MockHostInfo) ExtractEbsIDsUsedByKubernetes() map[string]string {
	return map[string]string{}
}
