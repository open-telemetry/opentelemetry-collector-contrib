// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package dirprovider // import "github.com/open-telemetry/opentelemetry-collector-contrib/confmap/provider/dirprovider"

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"go.opentelemetry.io/collector/confmap"
	"gopkg.in/yaml.v3"
)

const (
	schemeName = "dir"
)

type provider struct{}

// NewFactory returns a new confmap.ProviderFactory that creates a confmap.Provider
// which reads configuration from all YAML files in a directory.
//
// This Provider supports "dir" scheme, and can be called with a "uri" that follows:
//
//	dir-uri : dir:/path/to/config/directory
//
// The provider will recursively find all .yaml and .yml files in the directory,
// sort them alphabetically by path, and merge them together.
//
// Examples:
// `dir:/etc/otel/config.d` - (unix)
// `dir:C:\otel\config.d` - (windows)
func NewFactory() confmap.ProviderFactory {
	return confmap.NewProviderFactory(newWithSettings)
}

func newWithSettings(_ confmap.ProviderSettings) confmap.Provider {
	return &provider{}
}

func (p *provider) Retrieve(_ context.Context, uri string, _ confmap.WatcherFunc) (*confmap.Retrieved, error) {
	if !strings.HasPrefix(uri, schemeName+":") {
		return nil, fmt.Errorf("%q uri is not supported by %q provider", uri, schemeName)
	}

	// Extract directory path from URI
	dirPath := strings.TrimPrefix(uri, schemeName+":")

	// Clean and validate the path
	dirPath = filepath.Clean(dirPath)

	// Check if directory exists
	info, err := os.Stat(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("directory %q does not exist", dirPath)
		}
		return nil, fmt.Errorf("failed to access directory %q: %w", dirPath, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%q is not a directory", dirPath)
	}

	// Find all YAML files recursively
	var yamlFiles []string
	err = filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".yaml" || ext == ".yml" {
			yamlFiles = append(yamlFiles, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk directory %q: %w", dirPath, err)
	}

	// Sort files alphabetically for deterministic ordering
	slices.Sort(yamlFiles)

	// Merge all YAML files
	merged := make(map[string]any)
	for _, file := range yamlFiles {
		content, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %q: %w", file, err)
		}

		var conf map[string]any
		if err := yaml.Unmarshal(content, &conf); err != nil {
			return nil, fmt.Errorf("failed to parse YAML file %q: %w", file, err)
		}

		// Merge this config into the result
		merged = mergeMaps(merged, conf)
	}

	return confmap.NewRetrieved(merged)
}

func (*provider) Scheme() string {
	return schemeName
}

func (*provider) Shutdown(context.Context) error {
	return nil
}

// mergeMaps recursively merges src into dst.
// Values in src take precedence over values in dst.
func mergeMaps(dst, src map[string]any) map[string]any {
	if dst == nil {
		dst = make(map[string]any)
	}

	for key, srcVal := range src {
		if dstVal, exists := dst[key]; exists {
			// If both are maps, merge recursively
			srcMap, srcIsMap := srcVal.(map[string]any)
			dstMap, dstIsMap := dstVal.(map[string]any)
			if srcIsMap && dstIsMap {
				dst[key] = mergeMaps(dstMap, srcMap)
				continue
			}
		}
		// Otherwise, src value overwrites dst value
		dst[key] = srcVal
	}

	return dst
}
