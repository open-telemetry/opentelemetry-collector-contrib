// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type RunMode string

const (
	Component RunMode = "component"
	Package   RunMode = "package"
)

type Config = struct {
	Mode         RunMode
	DirPath      string
	OutputFolder string
	RootTypeName string
	FileType     string
	Class        string
	Mappings     Mappings
}

var (
	rootType     = flag.String("r", "Config", "Root type name for component schema generation")
	outputFolder = flag.String("o", "", "Output schema folder (defaults to input folder)")
	fileType     = flag.String("t", "yaml", "Output file type (yaml or json)")
)

func usage() {
	docs := []string{
		"Usage: schemagen [options] <path>",
		"This script is a tiny utility that walks a Go configuration file and emits JSON Schema that mirrors the exported structs.\n",
		"Options:\n",
		"\nArguments:",
		`  <input_file > Path to the dir to be processed. If not provided, the current working directory is used.`,
		"\nExamples:",
		"  > schemagen ./components/test_receiver/  		 # Generate schema for a component",
		"  > schemagen -o component.schema ./config.go       # Generate schema with a custom output file name",
		"  > schemagen -t json ./config.go                   # Generate schema in JSON format",
		"  > schemagen -r DatabaseConfig ./config.go         # Generate schema for component with a custom root type name",
	}
	_, _ = fmt.Fprintf(os.Stderr, "%s", strings.Join(docs[:3], "\n"))
	flag.PrintDefaults()
	_, _ = fmt.Fprintf(os.Stderr, "%s", strings.Join(docs[3:], "\n"))
}

func ReadConfig() (*Config, error) {
	flag.Usage = usage
	flag.Parse()

	inputPath := flag.Arg(0)
	if inputPath == "" {
		inputPath = "."
	}
	info, err := os.Stat(inputPath)
	if err != nil {
		return nil, err
	}

	var (
		filePath string
		dirPath  string
		output   = *outputFolder
		mode     = Package
		mappings Mappings
		ctype    string
		class    string
	)

	switch {
	case info.IsDir():
		dirPath, _ = filepath.Abs(inputPath)
	default:
		dirPath, _ = filepath.Abs(filepath.Dir(inputPath))
	}

	if output == "" {
		output = dirPath
	}

	if *rootType == "" {
		file := filepath.Base(filePath)
		ext := filepath.Ext(file)
		fileName := strings.TrimSuffix(file, ext)
		*rootType = toPascalCase(fileName)
	}

	if *fileType != "json" && *fileType != "yaml" && *fileType != "yml" {
		return nil, errors.New("unknown schema file type - use yaml or json: " + *fileType)
	}

	if md, ok := ReadMetadata(dirPath); ok {
		ctype = md.Type
		class = md.Status.Class
		switch class {
		case "receiver", "processor", "exporter", "connector", "extension":
			mode = Component
		case "pkg":
			mode = Package
		default:
			return nil, fmt.Errorf("schema generation for class '%s' is not supported", md.Status.Class)
		}
	}

	if s, ok := ReadSettingsFile(); ok {
		mappings = s.Mappings
		comp := class + "/" + ctype
		if override, found := s.ComponentOverrides[comp]; found {
			*rootType = override.ConfigName
		}
	}

	return &Config{
		DirPath:      dirPath,
		OutputFolder: output,
		RootTypeName: *rootType,
		FileType:     *fileType,
		Mode:         mode,
		Mappings:     mappings,
		Class:        class,
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
