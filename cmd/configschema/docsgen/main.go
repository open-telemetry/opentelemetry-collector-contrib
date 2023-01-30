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
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/components"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/configschema"
)

func main() {
	err := run(os.Args)
	if err != nil {
		fmt.Printf("error: %v\n", err)
	}
}

func run(args []string) error {
	c, err := components.Components()
	if err != nil {
		return err
	}
	switch len(args) {
	case 2:
		return configschema.GenerateMDFiles(c, filepath.Join("..", ".."), "github.com/open-telemetry/opentelemetry-collector-contrib")
	case 3:
		componentType := args[1]
		componentName := args[2]
		return configschema.GenerateMDFile(c, filepath.Join("..", ".."), "github.com/open-telemetry/opentelemetry-collector-contrib", componentType, componentName)
	default:
		return errors.New("wrong number of arguments")
	}
}
