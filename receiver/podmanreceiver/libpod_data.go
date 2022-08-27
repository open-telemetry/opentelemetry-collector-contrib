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

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

var (
	_ json.Unmarshaler = (*containerEnv)(nil)
)

type containerList []struct {
	ID string
}

type container struct {
	ID     string
	Config containerConfig
}

type containerConfig struct {
	Env    containerEnv
	Image  string
	Labels map[string]string
}

type containerEnv map[string]string

func (c *containerEnv) UnmarshalJSON(bytes []byte) error {
	var envs []string
	if err := json.Unmarshal(bytes, &envs); err != nil {
		return fmt.Errorf("failed to unmarshall envs: %s, %w", string(bytes), err)
	}

	parsedEnvs := make(map[string]string, len(envs))

	for _, v := range envs {
		parsedEnvParts := strings.SplitN(v, "=", 2)
		if len(parsedEnvParts) < 2 {
			continue
		}

		parsedEnvs[parsedEnvParts[0]] = parsedEnvParts[1]
	}

	*c = parsedEnvs

	return nil
}

type event struct {
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

type containerStatsReportError struct {
	Cause    string
	Message  string
	Response int64
}

type containerStatsReport struct {
	Error containerStatsReportError
	Stats []containerStats
}
