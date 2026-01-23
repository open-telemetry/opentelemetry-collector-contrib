// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"errors"
	"os"
	"path/filepath"
)

func WriteSchemaToFile(schema *Schema, config *Config) (string, error) {
	schemaPath := createOutputFilePath(schema, config)
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
		err = errors.New("unknown output file type; use json or yaml")
	}
	if err != nil {
		return "", err
	}

	err = writeFile(schemaPath, raw)
	if err != nil {
		return "", err
	}

	return schemaPath, nil
}

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0o600)
}

func createOutputFilePath(schema *Schema, config *Config) string {
	fileName := "config.schema." + config.FileType
	if config.Mode == Package {
		packageName := filepath.Base(schema.ID)
		fileName = packageName + ".schema." + config.FileType
	}
	return filepath.Join(config.OutputFolder, fileName)
}
