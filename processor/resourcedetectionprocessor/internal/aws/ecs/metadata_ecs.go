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

package ecs

import (
	"encoding/json"
	"log"
	"net/http"
)

type TaskMetaData struct {
	Cluster          string
	LaunchType       string // TODO: Change to enum when defined in otel collector convent
	TaskARN          string
	Family           string
	AvailabilityZone string
	Containers       []Container
}

type Container struct {
	DockerID     string `json:"DockerId"`
	ContainerARN string
	Type         string
	KnownStatus  string
	LogDriver    string
	LogOptions   LogData
}

type LogData struct {
	LogGroup string `json:"awslogs-group"`
	Region   string `json:"awslogs-region"`
	Stream   string `json:"awslogs-stream"`
}

type ecsMetadataProviderImpl struct {
}

var _ ecsMetadataProvider = &ecsMetadataProviderImpl{}

// Retrieves the metadata for a task running on Amazon ECS
func (md *ecsMetadataProviderImpl) fetchTaskMetaData(tmde string) (*TaskMetaData, error) {
	ret, err := fetch(tmde+"/task", true)
	return ret.(*TaskMetaData), err
}

// Retrieves the metadata for the Amazon ECS Container the collector is running on
func (md *ecsMetadataProviderImpl) fetchContainerMetaData(tmde string) (*Container, error) {
	ret, err := fetch(tmde, false)
	return ret.(*Container), err
}

func fetch(tmde string, task bool) (tmdeResp interface{}, err error) {
	resp, err := http.Get(tmde)

	if err != nil {
		log.Panicf("Received error from ECS Task Metadata Endpoint: %v", err)
		return nil, err
	}

	if task {
		tmdeResp = &TaskMetaData{}
	} else {
		tmdeResp = &Container{}
	}

	err = json.NewDecoder(resp.Body).Decode(tmdeResp)
	defer resp.Body.Close()

	if err != nil {
		log.Panicf("Encountered unexpected error reading response from ECS Task Metadata Endpoint: %v", err)
		return nil, err
	}

	return tmdeResp, nil
}
