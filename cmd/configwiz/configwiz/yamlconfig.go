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

package configwiz

import "gopkg.in/yaml.v2"

func checkYamlFile(filePath string) bool {
	// TODO: write functionality to check if the yaml file that was ouputted from the program is deployable
	return true
}

// buildYamlFile outputs a .yaml file based on the configuration we created
func buildYamlFile(m map[string]interface{}) (output string) {
	outMaps := yamlFile{
		map[string]interface{}{
			"receivers": m["receivers"],
		},
		map[string]interface{}{
			"processors": m["processors"],
		},
		map[string]interface{}{
			"exporters": m["exporters"],
		},
		map[string]interface{}{
			"extensions": m["extensions"],
		},
		map[string]interface{}{
			"service": m["service"],
		},
	}
	output += marshal(outMaps.receivers)
	output += marshal(outMaps.processors)
	output += marshal(outMaps.exporters)
	output += marshal(outMaps.extensions)
	output += marshal(outMaps.service)
	return
}

func marshal(comp interface{}) string {
	if bytes, err := yaml.Marshal(comp); err != nil {
		panic(err)
	} else {
		return string(bytes)
	}
}

type yamlFile struct {
	receivers  interface{}
	processors interface{}
	exporters  interface{}
	extensions interface{}
	service    interface{}
}
