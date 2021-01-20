// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awsecscontainermetrics

// TaskMetadata defines task metadata for a task
type TaskMetadata struct {
	Cluster          string `json:"Cluster,omitempty"`
	TaskARN          string `json:"TaskARN,omitempty"`
	Family           string `json:"Family,omitempty"`
	Revision         string `json:"Revision,omitempty"`
	AvailabilityZone string `json:"AvailabilityZone,omitempty"`
	PullStartedAt    string `json:"PullStartedAt,omitempty"`
	PullStoppedAt    string `json:"PullStoppedAt,omitempty"`
	KnownStatus      string `json:"KnownStatus,omitempty"`
	LaunchType       string `json:"LaunchType,omitempty"`

	Limits     Limit               `json:"Limits,omitempty"`
	Containers []ContainerMetadata `json:"Containers,omitempty"`
}

// ContainerMetadata defines container metadata for a container
type ContainerMetadata struct {
	DockerID      string            `json:"DockerId,omitempty"`
	ContainerName string            `json:"Name,omitempty"`
	DockerName    string            `json:"DockerName,omitempty"`
	Image         string            `json:"Image,omitempty"`
	Labels        map[string]string `json:"Labels,omitempty"`
	Limits        Limit             `json:"Limits,omitempty"`
	ImageID       string            `json:"ImageID,omitempty"`
	CreatedAt     string            `json:"CreatedAt,omitempty"`
	StartedAt     string            `json:"StartedAt,omitempty"`
	FinishedAt    string            `json:"FinishedAt,omitempty"`
	KnownStatus   string            `json:"KnownStatus,omitempty"`
	ExitCode      *int64            `json:"ExitCode,omitempty"`
}

// Limit defines the Cpu and Memory limts
type Limit struct {
	CPU    *float64 `json:"CPU,omitempty"`
	Memory *uint64  `json:"Memory,omitempty"`
}
