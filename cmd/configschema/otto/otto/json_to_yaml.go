// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package otto

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"gopkg.in/yaml.v2"
)

type jsonToYAMLHandler struct {
	logger *log.Logger
}

func (h jsonToYAMLHandler) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	bytes, err := io.ReadAll(req.Body)
	if err != nil {
		h.logger.Printf("jsonToYAMLHandler: ServeHTTP: error reading request: %v", err)
		resp.WriteHeader(http.StatusInternalServerError)
		return
	}
	m := map[string]interface{}{}
	err = json.Unmarshal(bytes, &m)
	if err != nil {
		h.logger.Printf("jsonToYAMLHandler: ServeHTTP: error unmarshaling request: %v", err)
		resp.WriteHeader(http.StatusInternalServerError)
		return
	}
	yml, err := yaml.Marshal(m)
	if err != nil {
		h.logger.Printf("jsonToYAMLHandler: ServeHTTP: error converting JSON to YAML: %v", err)
		resp.WriteHeader(http.StatusInternalServerError)
		return
	}
	_, err = resp.Write(yml)
	if err != nil {
		h.logger.Printf("jsonToYAMLHandler: ServeHTTP: error writing response: %v", err)
	}
}
