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
	nodeCapacity    *NodeCapacity
	ec2metadata     *EC2Metadata
	ebsVolume       *EBSVolume
	ec2Tags         *EC2Tags
	refreshInterval time.Duration
	shutdownC       chan bool
}

// NewMachineInfo creates a new MachineInfo struct
func NewMachineInfo(refreshInterval time.Duration, logger *zap.Logger) *MachineInfo {
	nodeCapacity, err := NewNodeCapacity(logger)
	if err != nil {
		logger.Error("Failed to initialize NodeCapacity", zap.Error(err))
	}

	mInfo := &MachineInfo{
		nodeCapacity:    nodeCapacity,
		ec2metadata:     NewEC2Metadata(refreshInterval, logger),
		refreshInterval: refreshInterval,
		shutdownC:       make(chan bool),
		logger:          logger,
	}

	mInfo.lazyInitEBSVolume()
	mInfo.lazyInitEC2Tags()
	return mInfo
}

func (m *MachineInfo) lazyInitEBSVolume() {
	if m.ebsVolume == nil {
		//delay the initialization. If instance id is not available, ebsVolume is set to nil
		//Because ebs volumes only change occasionally, we refresh every 5 collection intervals to reduce ec2 api calls
		m.ebsVolume = NewEBSVolume(m.GetInstanceID(), 5*m.refreshInterval, m.logger)
	}

	go func() {
		refreshTicker := time.NewTicker(m.refreshInterval)
		defer refreshTicker.Stop()
		for {
			select {
			case <-refreshTicker.C:
				if m.ebsVolume != nil {
					return
				}
				m.logger.Info("refresh to initialize ebsVolume")
				m.ebsVolume = NewEBSVolume(m.GetInstanceID(), m.refreshInterval, m.logger)
			case <-m.shutdownC:
				return
			}
		}
	}()
}

func (m *MachineInfo) lazyInitEC2Tags() {
	if m.ec2Tags == nil {
		//delay the initialization. If instance id is not available, c2Tags is set to nil
		m.ec2Tags = NewEC2Tags(m.GetInstanceID(), m.refreshInterval, m.logger)
	}

	go func() {
		refreshTicker := time.NewTicker(m.refreshInterval)
		defer refreshTicker.Stop()
		for {
			select {
			case <-refreshTicker.C:
				if m.ec2Tags != nil {
					return
				}
				m.logger.Info("refresh to initialize ec2Tags")
				m.ec2Tags = NewEC2Tags(m.GetInstanceID(), m.refreshInterval, m.logger)
			case <-m.shutdownC:
				return
			}
		}
	}()
}

// Shutdown stops the refreshing of machine info
func (m *MachineInfo) Shutdown() {
	close(m.shutdownC)
}

// GetInstanceID returns the ec2 instance id for the host
func (m *MachineInfo) GetInstanceID() string {
	return m.ec2metadata.GetInstanceID()
}

// GetInstanceType returns the ec2 instance type for the host
func (m *MachineInfo) GetInstanceType() string {
	return m.ec2metadata.GetInstanceType()
}

// GetNumCores returns the number of cpu cores on the host
func (m *MachineInfo) GetNumCores() int64 {
	return m.nodeCapacity.CPUCapacity
}

// GetMemoryCapacity returns the total memory (in bytes) on the host
func (m *MachineInfo) GetMemoryCapacity() int64 {
	return m.nodeCapacity.MemCapacity
}

// GetEBSVolumeID returns the ebs volume id corresponding to the given device name
func (m *MachineInfo) GetEBSVolumeID(devName string) string {
	if m.ebsVolume != nil {
		return m.ebsVolume.GetEBSVolumeID(devName)
	}

	return ""
}

// GetClusterName returns the cluster name associated with the host
func (m *MachineInfo) GetClusterName() string {
	if m.ec2Tags != nil {
		return m.ec2Tags.GetClusterName()
	}

	return ""
}

// GetAutoScalingGroupName returns the auto scaling group associated with the host
func (m *MachineInfo) GetAutoScalingGroupName() string {
	if m.ec2Tags != nil {
		return m.ec2Tags.GetAutoScalingGroupName()
	}

	return ""
}
