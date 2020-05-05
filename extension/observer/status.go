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

package observer

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"sync"
)

// Status listens to state change notifications so it can provide the current state.
type Status struct {
	sync.Mutex
	state map[string]Endpoint
}

// OnAdd is called when endpoints are added.
func (s *Status) OnAdd(added []Endpoint) {
	s.Lock()
	defer s.Unlock()
	for _, e := range added {
		s.state[e.ID] = e
	}
}

// OnRemove is called when endpoints are removed.
func (s *Status) OnRemove(removed []Endpoint) {
	s.Lock()
	defer s.Unlock()

	for _, e := range removed {
		delete(s.state, e.ID)
	}
}

// OnChange is called when endpoints are changed.
func (s *Status) OnChange(changed []Endpoint) {
	s.Lock()
	defer s.Unlock()

	for _, e := range changed {
		s.state[e.ID] = e
	}
}

type endpointWithType struct {
	Endpoint
	Type *string `json:"type"`
}

func (s *Status) json() ([]byte, error) {
	s.Lock()
	defer s.Unlock()

	endpoints := make([]endpointWithType, 0, len(s.state))
	for _, e := range s.state {
		var typeStr *string
		if typeOf := reflect.TypeOf(e.Details); typeOf != nil {
			typ := typeOf.Name()
			typeStr = &typ
		}
		endpoints = append(endpoints, endpointWithType{
			Endpoint: e,
			Type:     typeStr,
		})
	}

	sort.Slice(endpoints, func(i, j int) bool {
		return endpoints[i].ID < endpoints[j].ID
	})

	return json.Marshal(endpoints)
}

// StatusMux returns a mux that serves observer status.
func StatusMux(obs Observable) *http.ServeMux {
	mux := http.NewServeMux()
	s := &Status{state: map[string]Endpoint{}}
	obs.ListAndWatch(s)
	mux.HandleFunc("/status", func(writer http.ResponseWriter, request *http.Request) {
		data, err := s.json()
		if err != nil {
			http.Error(writer, fmt.Sprintf("failed to get status: %v", err), http.StatusInternalServerError)
			return
		}
		writer.Header().Set("content-type", "application/json")
		_, _ = writer.Write(data)
	})
	return mux
}
