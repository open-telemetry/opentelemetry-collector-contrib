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

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/configschema"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/configschema/yamlgen/yamlgen"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/components"
)

func main() {
	isZip, zipFile, sourceRootDir := getFlags()
	dr := configschema.NewDirResolver(sourceRootDir, configschema.DefaultModule)
	writer, err := yamlgen.NewYAMLWriter(dr, isZip, zipFile)
	if err != nil {
		fmt.Printf("error setting up yaml writer %v", err)
		os.Exit(1)
	}
	c, err := components.Components()
	if err != nil {
		fmt.Printf("error setting up yaml writer %v", err)
		os.Exit(1)
	}
	err = yamlgen.CLI(writer, c, dr)
	if err != nil {
		fmt.Print(err.Error())
	}
}

func getFlags() (bool, string, string) {
	isZip := flag.Bool("z", false, "whether to create a zip file instead of writing yamls to each component directory")
	zipFile := flag.String("f", "yaml.zip", "the name of the output zip file (only valid with -z)")
	sourceRootDir := flag.String("d", "../..", "")
	flag.Parse()
	return *isZip, *zipFile, *sourceRootDir
}
