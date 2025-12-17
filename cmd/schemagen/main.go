// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main // import "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/schemagen"

import (
	"fmt"
	"log"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/schemagen/internal"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	config, err := internal.ReadConfig()
	if err != nil {
		return err
	}

	parser := internal.NewParser(config)
	schema, err := parser.Parse()
	if err != nil {
		return err
	}

	err = internal.WriteSchemaToFile(schema, config)
	if err != nil {
		return err
	}
	fmt.Println("Schema successfully written to", config.SchemaPath)
	return nil
}
