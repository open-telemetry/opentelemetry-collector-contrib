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

package prometheusreceiver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"

	"github.com/prometheus/common/model"
)

type MockTargetAllocator struct {
	mu          sync.Mutex // mu protects the fields below.
	endpoints   map[string][]mockTargetAllocatorResponse
	accessIndex map[string]*int32
	wg          *sync.WaitGroup
	srv         *httptest.Server
}

type mockTargetAllocatorResponse struct {
	code int
	data []byte
}

type mockTargetAllocatorResponseRaw struct {
	code int
	data interface{}
}

type HTTPSDResponse struct {
	Targets []string                             `json:"targets"`
	Labels  map[model.LabelName]model.LabelValue `json:"labels"`
}

func (mta *MockTargetAllocator) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	mta.mu.Lock()
	defer mta.mu.Unlock()

	iptr, ok := mta.accessIndex[req.URL.Path]
	if !ok {
		rw.WriteHeader(404)
		return
	}
	index := int(*iptr)
	atomic.AddInt32(iptr, 1)
	pages := mta.endpoints[req.URL.Path]
	if index >= len(pages) {
		if index == len(pages) {
			mta.wg.Done()
		}
		rw.WriteHeader(404)
		return
	}
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(pages[index].code)
	_, _ = rw.Write(pages[index].data)
}

func (mta *MockTargetAllocator) Start() {
	mta.srv.Start()
}

func (mta *MockTargetAllocator) Stop() {
	mta.srv.Close()
}

func transformTAResponseMap(rawResponses map[string][]mockTargetAllocatorResponseRaw) (map[string][]mockTargetAllocatorResponse, map[string]*int32, error) {
	responsesMap := make(map[string][]mockTargetAllocatorResponse)
	responsesIndexMap := make(map[string]*int32)
	for path, responsesRaw := range rawResponses {
		var responses []mockTargetAllocatorResponse
		for _, responseRaw := range responsesRaw {
			respBodyBytes, err := json.Marshal(responseRaw.data)
			if err != nil {
				return nil, nil, err
			}
			responses = append(responses, mockTargetAllocatorResponse{
				code: responseRaw.code,
				data: respBodyBytes,
			})
		}
		responsesMap[path] = responses

		v := int32(0)
		responsesIndexMap[path] = &v
	}
	return responsesMap, responsesIndexMap, nil
}

func setupMockTargetAllocator(rawResponses map[string][]mockTargetAllocatorResponseRaw) (*MockTargetAllocator, error) {
	responsesMap, responsesIndexMap, err := transformTAResponseMap(rawResponses)
	if err != nil {
		return nil, err
	}

	mockTA := &MockTargetAllocator{
		endpoints:   responsesMap,
		accessIndex: responsesIndexMap,
		wg:          &sync.WaitGroup{},
	}
	mockTA.srv = httptest.NewUnstartedServer(mockTA)
	mockTA.wg.Add(len(responsesMap))

	return mockTA, nil
}
