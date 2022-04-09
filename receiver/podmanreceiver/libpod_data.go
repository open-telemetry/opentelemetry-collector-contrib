// Copyright  The OpenTelemetry Authors
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

//go:build !windows
// +build !windows

package podmanreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/podmanreceiver"

import "time"

type Container struct {
	AutoRemove bool
	Command    []string
	Created    string
	CreatedAt  string
	ExitCode   int
	Exited     bool
	Id         string
	Image      string
	ImageID    string
	IsInfra    bool
	Labels     map[string]string
	Mounts     []string
	Names      []string
	Namespaces map[string]string
	Networks   []string
	Pid        int
	Pod        string
	PodName    string
	Ports      []map[string]interface{}
	Size       map[string]string
	StartedAt  int
	State      string
	Status     string
}

type Event struct {
	ID     string
	Status string
}

type containerStats struct {
	AvgCPU        float64
	ContainerID   string
	Name          string
	PerCPU        []uint64
	CPU           float64
	CPUNano       uint64
	CPUSystemNano uint64
	DataPoints    int64
	SystemNano    uint64
	MemUsage      uint64
	MemLimit      uint64
	MemPerc       float64
	NetInput      uint64
	NetOutput     uint64
	BlockInput    uint64
	BlockOutput   uint64
	PIDs          uint64
	UpTime        time.Duration
	Duration      uint64
}

type containerStatsReport struct {
	Error string
	Stats []containerStats
}
