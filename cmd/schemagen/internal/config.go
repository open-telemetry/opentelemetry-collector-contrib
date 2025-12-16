// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
)

type Config = struct {
	FilePath       string
	DirPath        string
	SchemaPath     string
	SchemaIDPrefix string
	RootTypeName   string
	FileType       string
}

const (
	DefaultConfigGoFileName = "config.go"
	DefaultSchemaFileName   = "config.schema"
)

var (
	id       = flag.String("p", "", "Schema ID prefix")
	rootType = flag.String("r", "", "Root type name (default is derived from file name)")
	output   = flag.String("o", DefaultSchemaFileName, "Output schema file name (without extension)")
	fileType = flag.String("t", "yaml", "Output file type (yaml or json)")
)

func ReadConfig() (*Config, error) {
	flag.Parse()

	if len(flag.Args()) < 1 {
		return nil, errors.New("usage: schemagen <path>")
	}

	inputPath := flag.Arg(0)
	info, err := os.Stat(inputPath)
	if err != nil {
		return nil, err
	}

	var (
		filePath string
		dirPath  string
	)

	switch {
	case info.IsDir():
		dirPath = inputPath
		filePath = filepath.Join(dirPath, DefaultConfigGoFileName)
	default:
		filePath = inputPath
		dirPath = filepath.Dir(filePath)
	}

	if *rootType == "" {
		file := filepath.Base(filePath)
		ext := filepath.Ext(file)
		fileName := strings.TrimSuffix(file, ext)
		*rootType = toPascalCase(fileName)
	}

	ext := *fileType
	switch *fileType {
	case "yaml", "yml":
		ext = ".yaml"
	case "json":
		ext = ".json"
	default:
		return nil, errors.New("unknown output file type - use yaml or json: " + ext)
	}

	return &Config{
		FilePath:       filePath,
		DirPath:        dirPath,
		SchemaPath:     filepath.Join(dirPath, *output+ext),
		SchemaIDPrefix: *id,
		RootTypeName:   *rootType,
		FileType:       *fileType,
	}, nil
}

func toPascalCase(s string) string {
	parts := strings.Split(s, "_")
	for i, part := range parts {
		if part != "" {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, "")
}
