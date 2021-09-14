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

import (
	"fmt"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/configschema"
)

func CLI(io Clio, factories component.Factories) {
	fileName := promptFileName(io)
	service := map[string]interface{}{
		// this is the overview (top-level) part of the wizard, where the user just creates the pipelines
		"pipelines": pipelinesWizard(io, factories),
	}
	m := map[string]interface{}{
		"service": service,
	}
	dr := configschema.NewDirResolver(".", "github.com/open-telemetry/opentelemetry-collector-contrib")
	for componentGroup, names := range serviceToComponentNames(service) {
		handleComponent(io, factories, m, componentGroup, names, dr)
	}
	pr := io.newIndentingPrinter(0)
	yamlContents := buildYamlFile(m)
	pr.println("~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~")
	pr.println(yamlContents)
	writeFile(fileName, yamlContents)
	if !checkYamlFile(fileName) {
		panic(fmt.Errorf("invalid pipeline. Update file contents before trying to deploy"))
	}
}
