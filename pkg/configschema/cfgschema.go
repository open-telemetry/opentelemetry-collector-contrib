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

package configschema // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/configschema"

import (
	"go.opentelemetry.io/collector/otelcol"
)

// GenerateYAMLFiles generates, for each component, a yaml file containing
// metadata about each component's configuration. The yaml files are written
// to `outputDir`.
func GenerateYAMLFiles(factories otelcol.Factories, sourceDir, outputDir, module string) error {
	dr := dirResolver{
		srcRoot:    sourceDir,
		moduleName: module,
	}
	cr := commentReader{dr: dr}
	writer := newYamlFileWriter(outputDir)
	return generateYAMLFiles(factories, cr.commentsForStruct, writer.writeFile)
}

// GenerateMDFile generates, for the given component, a Markdown file describing
// the component's configuration. The file is written to the component's
// directory.
func GenerateMDFile(factories otelcol.Factories, sourceDir, module, componentType, componentName string) error {
	dr := dirResolver{
		srcRoot:    sourceDir,
		moduleName: module,
	}
	cr := commentReader{dr: dr}
	writer := mdFileWriter{dr: dr}
	return generateMDFile(factories, cr.commentsForStruct, writer.writeFile, componentType, componentName)
}

// GenerateMDFiles generates, for each component, a Markdown file describing the
// component's configuration. The files are written to each component's
// directory.
func GenerateMDFiles(factories otelcol.Factories, sourceDir, module string) error {
	dr := dirResolver{
		srcRoot:    sourceDir,
		moduleName: module,
	}
	cr := commentReader{dr: dr}
	writer := mdFileWriter{dr: dr}
	return generateMDFiles(factories, cr.commentsForStruct, writer.writeFile)
}
