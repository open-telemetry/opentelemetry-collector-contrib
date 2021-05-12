// Copyright  OpenTelemetry Authors
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

package host

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestMachineInfo(t *testing.T) {
	m := NewMachineInfo(time.Minute, zap.NewNop())
	assert.Equal(t, "", m.GetInstanceID())
	assert.Equal(t, "", m.GetInstanceType())
	assert.Equal(t, int64(0), m.GetNumCores())
	assert.Equal(t, int64(0), m.GetMemoryCapacity())
	assert.Equal(t, "", m.GetEbsVolumeID("dev"))
	assert.Equal(t, "", m.GetClusterName())
	assert.Equal(t, "", m.GetAutoScalingGroupName())
	m.Shutdown()
}
