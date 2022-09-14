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
	"fmt"
	"log"
	"net/http"
	"os"
	"reflect"
	"strings"

	"go.opentelemetry.io/collector/component"
	"gopkg.in/yaml.v2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/configschema"
)

type cfgschemaHandler struct {
	logger   *log.Logger
	pipeline *pipeline
}

func (h cfgschemaHandler) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	// e.g. "/configschema/receiver/redis"
	parts := strings.Split(req.RequestURI, "/")
	componentType := parts[2]
	componentName := parts[3]

	fieldInfo, err := getFieldInfo(componentType, componentName, h.pipeline.factories, h.pipeline.dr)
	if err != nil {
		h.logger.Printf("cfgschemaHandler: ServeHTTP: error getting fieldInfo: %v", err)
		resp.WriteHeader(http.StatusInternalServerError)
		return
	}

	fjson, err := json.Marshal(fieldInfo)
	if err != nil {
		h.logger.Printf("cfgschemaHandler: ServeHTTP: error marshaling fieldInfo to json: %v", err)
		resp.WriteHeader(http.StatusInternalServerError)
		return
	}

	_, err = resp.Write(fjson)
	if err != nil {
		h.logger.Printf("cfgschemaHandler: ServeHTTP: error writing response: %v", err)
	}
}

func getFieldInfo(componentType string, componentName string, f component.Factories, dr configschema.DirResolver) (*configschema.Field, error) {
	override, err := readOverrideFile(componentType, componentName)
	if err == nil {
		return override, nil
	}
	return getFieldInfoByReflection(componentType, componentName, f, dr)
}

func getFieldInfoByReflection(componentType string, componentName string, f component.Factories, dr configschema.DirResolver) (*configschema.Field, error) {
	ci, err := configschema.GetCfgInfo(f, componentType, componentName)
	if err != nil {
		return nil, err
	}
	v := reflect.ValueOf(ci.CfgInstance)
	return configschema.ReadFields(v, dr)
}

func readOverrideFile(componentType, componentName string) (*configschema.Field, error) {
	fname := fmt.Sprintf("../overrides/%s-%s.yaml", componentType, componentName)
	bytes, err := os.ReadFile(fname)
	if err != nil {
		return nil, err
	}
	override := configschema.Field{}
	err = yaml.Unmarshal(bytes, &override)
	return &override, err
}
