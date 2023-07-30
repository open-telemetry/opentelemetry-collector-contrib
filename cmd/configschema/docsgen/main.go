// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"path/filepath"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/configschema"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/configschema/docsgen/docsgen"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/components"
)

func main() {
	c, err := components.Components()
	if err != nil {
		panic(err)
	}
	dr := configschema.NewDirResolver(filepath.Join("..", ".."), configschema.DefaultModule)
	docsgen.CLI(c, dr)
}
