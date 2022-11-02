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
	"path/filepath"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/configschema"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/configschema/cfgmetadatagen/cfgmetadatagen"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/components"
)

func main() {
	sourceDir, outputDir := getFlags()
	dr := configschema.NewDirResolver(sourceDir, configschema.DefaultModule)
	writer, err := cfgmetadatagen.NewYAMLWriter(dr, outputDir)
	if err != nil {
		fmt.Printf("error setting up yaml writer %v", err)
		os.Exit(1)
	}
	c, err := components.Components()
	if err != nil {
		fmt.Printf("error setting up yaml writer %v", err)
		os.Exit(1)
	}
	err = cfgmetadatagen.CLI(writer, c, dr)
	if err != nil {
		fmt.Print(err.Error())
	}
}

func getFlags() (string, string) {
	outputDir := flag.String("o", "", "output dir")
	sourceDir := flag.String("s", filepath.Join("..", ".."), "")
	flag.Parse()
	return *sourceDir, *outputDir
}
