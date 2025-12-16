// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"errors"
	"os"
)

func WriteSchemaToFile(schema *Schema, config *Config) error {
	schemaPath := config.SchemaPath
	var (
		err error
		raw []byte
	)
	switch config.FileType {
	case "yaml", "yml":
		raw, err = schema.ToYAML()
	case "json":
		raw, err = schema.ToJSON()
	default:
		err = errors.New("unknown output file type; use json or yaml: " + config.FilePath)
	}
	if err != nil {
		return err
	}
	return writeFile(schemaPath, raw)
}

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0o600)
}
