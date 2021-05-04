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
	"sync"
	"time"

	"go.uber.org/zap"
)

// MachineInfo contains information about a host
type MachineInfo struct {
	sync.RWMutex
	logger          *zap.Logger
	refreshInterval time.Duration
	shutdownC       chan bool
}

// NewMachineInfo creates a new MachineInfo struct
func NewMachineInfo(refreshInterval time.Duration, logger *zap.Logger) *MachineInfo {
	mInfo := &MachineInfo{
		refreshInterval: refreshInterval,
		shutdownC:       make(chan bool),
		logger:          logger,
	}

	// TODO: add more initializations
	return mInfo
}

// Shutdown stops the refreshing of machine info
func (m *MachineInfo) Shutdown() {
	close(m.shutdownC)
}

// GetInstanceID returns the ec2 instance id for the host
func (m *MachineInfo) GetInstanceID() string {
	//TODO: add implementation
	return ""
}

// GetInstanceType returns the ec2 instance type for the host
func (m *MachineInfo) GetInstanceType() string {
	//TODO: add implementation
	return ""
}

// GetNumCores returns the number of cpu cores on the host
func (m *MachineInfo) GetNumCores() int64 {
	//TODO: add implementation
	return 0
}

// GetMemoryCapacity returns the total memory (in bytes) on the host
func (m *MachineInfo) GetMemoryCapacity() int64 {
	//TODO: add implementation
	return 0
}

// GetEbsVolumeID returns the ebs volume id corresponding to the given device name
func (m *MachineInfo) GetEbsVolumeID(devName string) string {
	//TODO: add implementation
	return ""
}

// GetClusterName returns the cluster name associated with the host
func (m *MachineInfo) GetClusterName() string {
	//TODO: add implementation
	return ""
}

// GetAutoScalingGroupName returns the auto scaling group associated with the host
func (m *MachineInfo) GetAutoScalingGroupName() string {
	//TODO: add implementation
	return ""
}
