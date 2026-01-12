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
	FilePath     string
	DirPath      string
	OutputFolder string
	RootTypeName string
	FileType     string
	Mappings     Mappings
}

var (
	rootType     = flag.String("r", "", "Root type name (default is derived from file name)")
	outputFolder = flag.String("o", "", "Output schema folder")
	fileType     = flag.String("t", "yaml", "Output file type (yaml or json)")
)

func usage() {
	docs := []string{
		"Usage: schemagen [options] <path>",
		"This script is a tiny utility that walks a Go configuration file and emits JSON Schema that mirrors the exported structs.\n",
		"Options:\n",
		"\nArguments:",
		`  <input_file > Path to the file/dir to be processed (required). Depending on whether the path to a file or directory is specified, the script will generate a component or package schema.`,
		"\nExamples:",
		"  > schemagen ./components/test_receiver/config.go  # Generate schema for a single config file",
		"  > schemagen ./config/test_lib/                    # Generate schema for all types in a package",
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

	if len(flag.Args()) < 1 {
		return nil, errors.New("usage: schemagen [options] <path>; run with -h to see help text")
	}
	inputPath := flag.Arg(0)
	info, err := os.Stat(inputPath)
	if err != nil {
		return nil, err
	}

	var (
		filePath string
		dirPath  string
		output   = *outputFolder
		mode     RunMode
		mappings Mappings
	)

	switch {
	case info.IsDir():
		dirPath, _ = filepath.Abs(inputPath)
		mode = Package
	default:
		filePath = inputPath
		dirPath, _ = filepath.Abs(filepath.Dir(filePath))
		mode = Component
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

	s, ok := ReadSettingsFile()
	if ok {
		mappings = s.Mappings
		if output == "" && s.OutputFolder != "" {
			output = s.OutputFolder
		}
	}

	if output == "" {
		output = dirPath
	}

	return &Config{
		FilePath:     filePath,
		DirPath:      dirPath,
		OutputFolder: output,
		RootTypeName: *rootType,
		FileType:     *fileType,
		Mode:         mode,
		Mappings:     mappings,
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
