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

import (
	"encoding/json"
	"fmt"
)

// StatsProvider wraps a RestClient, returning an unmarshaled
// stats.Summary struct from the kubelet API.
type StatsProvider struct {
	rc RestClient
}

func NewStatsProvider(rc RestClient) *StatsProvider {
	return &StatsProvider{rc: rc}
}

// TaskStats calls the ecs task metadata endpoint and unmarshals the
// results into a stats.TaskStats struct.
func (p *StatsProvider) TaskStats() (map[string]ContainerStats, error) {
	response, err := p.rc.EndpointResponse()
	// fmt.Println(string(response))
	if err != nil {
		return nil, err
	}
	//var out Model
	m := make(map[string]ContainerStats)

	err = json.Unmarshal(response, &m)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Println(len(m))
	}

	for key, value := range m {
		fmt.Println(key, value.Name, value.Id)
	}
	return m, nil

	// err = json.Unmarshal(response, &out)
	// if err != nil {
	// 	fmt.Println("Unmarshal Error: \n", err)
	// 	return nil, err
	// }
	// return &out, nil
}
