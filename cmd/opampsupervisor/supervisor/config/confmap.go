// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/provider/envprovider"
	"go.opentelemetry.io/collector/confmap/provider/fileprovider"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

var configURISchemeRegexp = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]+:`)

var providerSchemes = map[string]struct{}{
	"file": {},
	"env":  {},
}

// ErrConfigFileNotFound marks required config sources that point to missing files.
var ErrConfigFileNotFound = errors.New("config file not found")

func normalizeConfigURI(uri string) string {
	if configURISchemeRegexp.MatchString(uri) {
		return uri
	}

	absPath, err := filepath.Abs(uri)
	if err != nil {
		return uri
	}

	return absPath
}

func resolverSettings(uris []string) confmap.ResolverSettings {
	return confmap.ResolverSettings{
		URIs:               uris,
		ProviderFactories:  providerFactories(),
		ConverterFactories: []confmap.ConverterFactory{},
		DefaultScheme:      "env",
	}
}

func providerFactories() []confmap.ProviderFactory {
	return []confmap.ProviderFactory{
		fileprovider.NewFactory(),
		envprovider.NewFactory(),
	}
}

// ResolveURI resolves a single config URI into a Conf.
func ResolveURI(uri string) (*confmap.Conf, error) {
	return ResolveURIs([]string{uri})
}

// ResolveURIs resolves a list of config URIs into a single merged Conf.
func ResolveURIs(uris []string) (*confmap.Conf, error) {
	normalizedURIs := make([]string, len(uris))
	for i, uri := range uris {
		normalizedURIs[i] = normalizeConfigURI(uri)
	}

	resolver, err := confmap.NewResolver(resolverSettings(normalizedURIs))
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resolver.Shutdown(context.Background())
	}()

	return resolver.Resolve(context.Background())
}

func retrieveURIForProvider(uri string) (string, string) {
	if !configURISchemeRegexp.MatchString(uri) {
		return "file:" + uri, "file"
	}

	scheme, _, _ := strings.Cut(uri, ":")
	if _, ok := providerSchemes[scheme]; ok {
		return uri, scheme
	}

	// Preserve support for local file paths with a colon in the name.
	if _, err := os.Stat(uri); err == nil {
		return "file:" + uri, "file"
	}

	return uri, scheme
}

// RetrieveURIAsConf retrieves a URI as a confmap.Conf without resolving embedded ${...} values.
func RetrieveURIAsConf(uri string, logger *zap.Logger) (*confmap.Conf, error) {
	normalizedURI := normalizeConfigURI(uri)
	retrieveURI, scheme := retrieveURIForProvider(normalizedURI)
	if logger == nil {
		logger = zap.NewNop()
	}

	factories := providerFactories()
	providers := make([]confmap.Provider, 0, len(factories))
	var provider confmap.Provider
	for _, factory := range factories {
		p := factory.Create(confmap.ProviderSettings{Logger: logger})
		providers = append(providers, p)
		if p.Scheme() == scheme {
			provider = p
		}
	}
	defer func() {
		for _, p := range providers {
			_ = p.Shutdown(context.Background())
		}
	}()

	if provider == nil {
		return nil, fmt.Errorf("unsupported scheme on URI %q", uri)
	}

	retrieved, err := provider.Retrieve(context.Background(), retrieveURI, nil)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %w", ErrConfigFileNotFound, err)
		}
		return nil, err
	}
	defer func() {
		_ = retrieved.Close(context.Background())
	}()

	return retrieved.AsConf()
}

// NewConfFromYAML parses YAML bytes into a confmap.Conf.
func NewConfFromYAML(yamlBytes []byte) (*confmap.Conf, error) {
	if len(yamlBytes) == 0 {
		return confmap.New(), nil
	}
	retrieved, err := confmap.NewRetrievedFromYAML(yamlBytes)
	if err != nil {
		return nil, err
	}
	return retrieved.AsConf()
}

// MarshalConfToYAML marshals a confmap.Conf to YAML bytes.
func MarshalConfToYAML(conf *confmap.Conf) ([]byte, error) {
	return yaml.Marshal(conf.ToStringMap())
}

// MergeConfFromYAML merges YAML bytes into an existing confmap.Conf.
func MergeConfFromYAML(base *confmap.Conf, yamlBytes []byte) error {
	incoming, err := NewConfFromYAML(yamlBytes)
	if err != nil {
		return err
	}
	return MergeConf(base, incoming)
}

// MergeConf merges incoming into base while appending service.extensions with deduplication.
func MergeConf(base, incoming *confmap.Conf) error {
	baseExtensions := getExtensions(base)
	incomingExtensions := getExtensions(incoming)

	if err := base.Merge(incoming); err != nil {
		return err
	}

	if len(baseExtensions) > 0 || len(incomingExtensions) > 0 {
		setExtensions(base, deduplicateExtensions(baseExtensions, incomingExtensions))
	}

	return nil
}

func getExtensions(conf *confmap.Conf) []any {
	val := conf.Get("service::extensions")
	if val == nil {
		return nil
	}
	extensions, ok := val.([]any)
	if !ok {
		return nil
	}
	result := make([]any, len(extensions))
	copy(result, extensions)
	return result
}

func setExtensions(conf *confmap.Conf, extensions []any) {
	overlay := confmap.NewFromStringMap(map[string]any{
		"service": map[string]any{
			"extensions": extensions,
		},
	})
	_ = conf.Merge(overlay)
}

func deduplicateExtensions(base, incoming []any) []any {
	all := make([]any, 0, len(base)+len(incoming))
	all = append(all, base...)
	all = append(all, incoming...)

	seen := make(map[any]struct{}, len(all))
	result := make([]any, 0, len(all))
	for _, ext := range all {
		if _, ok := seen[ext]; !ok {
			seen[ext] = struct{}{}
			result = append(result, ext)
		}
	}
	return result
}
